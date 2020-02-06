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
	// Backoff is the backoff strategy used for connection attempts
	BackOff backoff.BackOff
	// ConnectionTimeout is the amount of time to wait during a connection
	// attempt before stopping and trying again
	ConnectionTimeout time.Duration
	// AdditionalQueryArgs is a map of additional string values that are appended to the
	// query string of the server address
	AdditionalQueryArgs map[string]string
}

// DefaultManagerConfig returns a ManagerConfig with sane defaults
//
// ConnectionTimeout is 20 seconds
// Backoff is backoff.ExponentialBackoff
// AdditionalQueryArgs is an empty map
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		ConnectionTimeout:   20 * time.Second,
		BackOff:             backoff.NewExponentialBackOff(),
		AdditionalQueryArgs: make(map[string]string),
	}
}

// Manager manages Socket.IO connections to a single server across
// multiple namespaces
type Manager struct {
	sync.RWMutex
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
	return m.conn.Connected()
}

// Namespace returns a Socket for the Namespace.
// If there is an existing Socket for the Namespace, it is returned.
// If there is no existing Socket for the Namespace, it is created and
// returned.
//
// The Namespace method is safe for use by multiple concurrent goroutines
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
	opened := m.conn.Opened()

	m.Unlock()

	go m.conn.KeepAliveContext(managerCtx)
	go m.readFromEngineContext(managerCtx)
	go m.writeToEngineContext(managerCtx)

	openData := <-opened

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

	m.RLock()
	read := m.conn.Read()
	socketCtx := m.socketCtx
	m.RUnlock()

	for {
		select {
		case <-ctx.Done():
			return
		case packet, ok := read:
			if !ok {
				go m.reconnectContext(socketCtx)
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
	m.RLock()
	fromSockets := m.fromSockets
	outgoing := m.outgoingPackets
	supportsBinary := m.conn.SupportsBinary()
	m.RUnlock()

	for {
		select {
		case <-ctx.Done():
			// We're the writer, so it's our job to close the channel
			close(outgoing)
			return
		case packet, ok := <-fromSockets:
			if !ok {
				close(outgoing)
				return
			}

			encodedData, err := packet.Encode(supportsBinary)

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
					case outgoing <- &b:
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

// DialContext creates a new instance of Manager connected to the server at address
//
// ctx is used to create derived contexts for a variety of long-running operations.
// If ctx is cancelled, either manually or automatically
// (i.e. created with context.WithTimeout or or context.WithDeadline) the Manager and its Sockets
// will cease to function and must be recreated using a new context and call to DialContext
func DialContext(ctx context.Context, address string, cfg *ManagerConfig) (*Manager, error) {
	manager, err := newManagerContext(ctx, address, cfg)

	if err != nil {
		return nil, err
	}

	err = backoff.Retry(manager.startConnectionOperation(ctx), backoff.WithContext(cfg.BackOff, ctx))

	return manager, err
}
