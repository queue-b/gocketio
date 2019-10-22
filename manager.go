package gocketio

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v3"

	"github.com/queue-b/gocketio/socket"

	"github.com/queue-b/gocketio/engine"
)

// ErrInvalidAddress is returned when the user-supplied address is invalid
var ErrInvalidAddress = errors.New("Invalid address")

// ManagerConfig contains configuration information for a Manager
type ManagerConfig struct {
	BackOff             backoff.BackOff
	ConnectionTimeout   time.Duration
	AdditionalQueryArgs map[string]string
}

// DefaultManagerConfig returns a ManagerConfig with sane defaults
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		ConnectionTimeout:   20 * time.Second,
		BackOff:             backoff.NewExponentialBackOff(),
		AdditionalQueryArgs: make(map[string]string),
	}
}

// Manager manages connections to the same server with different namespaces
type Manager struct {
	sync.Mutex
	address         *url.URL
	sockets         map[string]*Socket
	conn            engine.Conn
	outgoingPackets chan engine.Packet
	fromSockets     chan socket.Packet
	socketCtx       context.Context
	cancel          context.CancelFunc
	opts            *ManagerConfig
}

func (m *Manager) forwardMessage(ctx context.Context, message socket.Packet) {
	ns := message.Namespace

	if ns == "" {
		ns = "/"
	}

	m.Lock()
	if s, ok := m.sockets[ns]; ok {
		m.Unlock()

		select {
		case <-ctx.Done():
			return
		case s.incomingPackets <- message:
		}
	} else {
		m.Unlock()
	}
}

// Connected returns true if the underlying transport is connected, false otherwise
func (m *Manager) Connected() bool {
	return m.conn.State() == engine.Connected
}

// Namespace returns a socket for the specified namespace
func (m *Manager) Namespace(namespace string) (*Socket, error) {
	m.Lock()
	defer m.Unlock()

	if nsSocket, ok := m.sockets[namespace]; ok {
		return nsSocket, nil
	}

	nsSocket := newSocket(namespace, m.conn.ID(), m.fromSockets)

	m.sockets[namespace] = nsSocket

	nsSocket.onOpen(m.socketCtx, m.conn.ID())

	return nsSocket, nil
}

func (m *Manager) onReconnect(id string) {
	m.Lock()
	defer m.Unlock()

	for _, v := range m.sockets {
		v.onOpen(m.socketCtx, id)
	}
}

func (m *Manager) onDisconnect() {
	m.Lock()
	defer m.Unlock()

	for _, v := range m.sockets {
		v.onDisconnect(false)
	}
}

func (m *Manager) connectContext(ctx context.Context) error {
	managerCtx, cancel := context.WithCancel(ctx)

	deadlineCtx, deadlineCancel := context.WithTimeout(ctx, m.opts.ConnectionTimeout)
	defer deadlineCancel()

	// Notify the sockets that they are disconnected (temporarily)
	m.onDisconnect()

	conn, err := engine.DialContext(deadlineCtx, m.address.String())

	if err != nil {
		cancel()
		return err
	}

	m.Lock()
	m.conn = engine.NewKeepAliveConn(conn, 100, m.outgoingPackets)
	m.cancel = cancel
	m.Unlock()

	go m.conn.KeepAliveContext(managerCtx)
	go m.readFromEngineContext(managerCtx)
	go m.writeToEngineContext(managerCtx)

	openData := <-m.conn.Opened()

	// Notify the sockets that they are reconnected
	m.onReconnect(openData.SID)

	_, err = m.Namespace("/")

	if err != nil {
		cancel()
		return err
	}

	return nil
}

func (m *Manager) reconnectContext(ctx context.Context) {
	m.Lock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
		m.conn = nil
	}
	m.Unlock()

	err := backoff.Retry(m.startConnectionOperation(ctx), backoff.WithContext(m.opts.BackOff, ctx))

	if err != nil {
		fmt.Println(err)
		return
	}

	return
}

func (m *Manager) startConnectionOperation(ctx context.Context) backoff.Operation {
	return func() error {
		return m.connectContext(ctx)
	}
}

func fixupAddress(address string, additionaQueryArgs map[string]string) (*url.URL, error) {
	parsedAddress, err := url.Parse(address)

	if err != nil {
		return nil, err
	}

	if parsedAddress.Scheme != "http" && parsedAddress.Scheme != "https" {
		return nil, ErrInvalidAddress
	}

	if parsedAddress.Path == "/" || parsedAddress.Path == "" {
		newPath := strings.TrimRight(parsedAddress.Path, "/")
		newPath += "/socket.io/"
		parsedAddress.Path = newPath
	}

	vals := parsedAddress.Query()

	if additionaQueryArgs != nil {
		for k, v := range additionaQueryArgs {
			vals.Add(k, v)
		}

		parsedAddress.RawQuery = vals.Encode()
	}

	return parsedAddress, nil
}

func (m *Manager) readFromEngineContext(ctx context.Context) {
	d := socket.BinaryDecoder{}

	for {
		select {
		case <-ctx.Done():
			return
		case packet, ok := <-m.conn.Read():
			if !ok {
				go m.reconnectContext(m.socketCtx)
				return
			}

			message, err := d.Decode(packet)

			if err != nil && err != socket.ErrWaitingForMorePackets {
				fmt.Println(err)
				continue
			}

			go m.forwardMessage(ctx, message)
		}
	}
}

func (m *Manager) writeToEngineContext(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// We're the writer, so it's our job to close the channel
			close(m.outgoingPackets)
			return
		case packet, ok := <-m.fromSockets:
			if !ok {
				close(m.outgoingPackets)
				return
			}

			encodedData, err := packet.Encode(m.conn.SupportsBinary())

			if err != nil || len(encodedData) == 0 {
				continue
			}

			// The first encoded element will always be a string,
			// even if the packet has binary
			first := string(encodedData[0])

			p := engine.StringPacket{}
			p.Type = engine.Message
			p.Data = &first

			select {
			case <-ctx.Done():
				return
			case m.outgoingPackets <- &p:
			}

			// Packet has attachments
			if len(encodedData) > 1 {
				for _, v := range encodedData[1:] {
					b := engine.BinaryPacket{}
					b.Type = engine.Message
					b.Data = v

					select {
					case <-ctx.Done():
						return
					case m.outgoingPackets <- &b:
					}
				}
			}
		}
	}
}

func newManagerContext(ctx context.Context, address string, opts *ManagerConfig) (*Manager, error) {
	if opts == nil {
		return nil, errors.New("Missing config")
	}

	manager := &Manager{}

	parsedAddress, err := fixupAddress(address, opts.AdditionalQueryArgs)

	if err != nil {
		return nil, err
	}

	manager.fromSockets = make(chan socket.Packet)
	manager.sockets = make(map[string]*Socket)
	manager.outgoingPackets = make(chan engine.Packet, 1)
	manager.socketCtx = ctx

	manager.address = parsedAddress
	manager.opts = opts

	return manager, nil
}

// DialContext attempts to connect to the Socket.IO server at address
func DialContext(ctx context.Context, address string, cfg *ManagerConfig) (*Manager, error) {
	manager, err := newManagerContext(ctx, address, cfg)

	if err != nil {
		return nil, err
	}

	err = backoff.Retry(manager.startConnectionOperation(ctx), backoff.WithContext(cfg.BackOff, ctx))

	return manager, err
}
