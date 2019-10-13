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
	address     *url.URL
	sockets     map[string]*Socket
	conn        engine.Transport
	fromSockets chan socket.Packet
	socketCtx   context.Context
	cancel      context.CancelFunc
	opts        *ManagerConfig
}

func handleDisconnect(manager *Manager, reconnectFunction func() error, disconnects <-chan struct{}) {
	for {
		select {
		case <-disconnects:
			manager.cancel()
			manager.conn = nil
			err := backoff.Retry(reconnectFunction, backoff.WithContext(manager.opts.BackOff, manager.socketCtx))

			if err != nil {
				fmt.Println(err)
				return
			}

			manager.onReconnect()
			return
		}
	}
}

func receiveFromEngine(ctx context.Context, manager *Manager, inputPackets chan engine.Packet) {
	d := socket.BinaryDecoder{}

	for {
		select {
		case <-ctx.Done():
			fmt.Println("ctx.Done, killing manager.receiveFromEngine")
			return
		case packet, ok := <-inputPackets:
			if !ok {
				fmt.Println("Invalid packet read, killing manager.receiveFromEngine")
				return
			}

			message, err := d.Decode(packet)

			if err != nil && err != socket.ErrWaitingForMorePackets {
				fmt.Println(err)
				continue
			}

			fmt.Printf("Received packet %v\n", message)

			go manager.forwardMessage(ctx, message)
		}
	}
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

		if message.Type == socket.Error {
			m.Lock()
			delete(m.sockets, ns)
			close(s.incomingPackets)
		}
	} else {
		m.Unlock()
	}
}

func sendToEngine(ctx context.Context, manager *Manager, outputPackets chan socket.Packet, enginePackets chan engine.Packet) {
	for {
		select {
		case <-ctx.Done():
			// We're the writer, so it's our job to close the channel
			fmt.Println("ctx.Done, killing manager.sendToEngine")
			close(enginePackets)
			return
		case packet, ok := <-outputPackets:
			if !ok {
				fmt.Println("Invalid read, killing manager.sendToEngine")
				return
			}

			encodedData, err := packet.Encode(manager.conn.SupportsBinary())

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
			case enginePackets <- &p:
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
					case enginePackets <- &b:
					}
				}
			}
		}
	}
}

// Connected returns true if the underlying transport is connected, false otherwise
func (m *Manager) Connected() bool {
	return m.conn != nil
}

// Namespace returns a socket for the specified namespace
func (m *Manager) Namespace(namespace string) (*Socket, error) {
	m.Lock()
	defer m.Unlock()

	if nsSocket, ok := m.sockets[namespace]; ok {
		return nsSocket, nil
	}

	nsSocket := &Socket{}
	nsSocket.outgoingPackets = m.fromSockets
	nsSocket.namespace = namespace
	nsSocket.incomingPackets = make(chan socket.Packet)

	nsSocket.events = sync.Map{}
	nsSocket.acks = sync.Map{}
	nsSocket.id = fmt.Sprintf("%v#%v", namespace, m.conn.ID())

	go receiveFromManager(m.socketCtx, nsSocket, nsSocket.incomingPackets)

	if !socket.IsRootNamespace(namespace) {
		connectPacket := socket.Packet{}
		connectPacket.Namespace = namespace
		connectPacket.Type = socket.Connect

		m.fromSockets <- connectPacket
	}

	m.sockets[namespace] = nsSocket

	return nsSocket, nil
}

func (m *Manager) onReconnect() {
	m.Lock()
	defer m.Unlock()

	for namespace := range m.sockets {
		if !socket.IsRootNamespace(namespace) {
			connectPacket := socket.Packet{}
			connectPacket.Namespace = namespace
			connectPacket.Type = socket.Connect

			m.fromSockets <- connectPacket
		}
	}
}

func connectContext(ctx context.Context, m *Manager, dialer engine.Dialer) error {
	managerCtx, cancel := context.WithCancel(ctx)

	conn, err := dialer(m.address.String())

	if err != nil {
		cancel()
		return err
	}

	m.conn = conn
	m.socketCtx = ctx
	m.cancel = cancel

	go receiveFromEngine(managerCtx, m, conn.Receive)
	go sendToEngine(managerCtx, m, m.fromSockets, conn.Send)
	go handleDisconnect(m, reconnect(ctx, m), conn.Disconnected())

	_, err = m.Namespace("/")

	if err != nil {
		cancel()
		return err
	}

	return nil
}

func reconnect(ctx context.Context, m *Manager) func() error {
	return func() error {
		return connectContext(ctx, m, engine.ContextDialer(ctx, m.address.String(), m.opts.ConnectionTimeout))
	}
}

// DialContext attempts to connect to the Socket.IO server at address
func DialContext(ctx context.Context, address string, cfg *ManagerConfig) (*Manager, error) {
	if cfg == nil {
		return nil, errors.New("Missing config")
	}

	manager := &Manager{}

	parsedAddress, err := url.Parse(address)

	if err != nil {
		return nil, err
	}

	if parsedAddress.Path == "/" || parsedAddress.Path == "" {
		newPath := strings.TrimRight(parsedAddress.Path, "/")
		newPath += "/socket.io/"
		parsedAddress.Path = newPath
	}

	vals := parsedAddress.Query()

	if cfg.AdditionalQueryArgs != nil {
		for k, v := range cfg.AdditionalQueryArgs {
			vals.Add(k, v)
		}

		parsedAddress.RawQuery = vals.Encode()
	}

	manager.fromSockets = make(chan socket.Packet)
	manager.sockets = make(map[string]*Socket)
	manager.address = parsedAddress
	manager.opts = cfg

	err = backoff.Retry(reconnect(ctx, manager), backoff.WithContext(cfg.BackOff, ctx))

	return manager, err
}
