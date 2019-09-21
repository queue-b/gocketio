package gocket

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"

	"github.com/queue-b/gocket/socket"

	"github.com/queue-b/gocket/engine"
)

// ManagerConfig contains configuration information for a Manager
type ManagerConfig struct {
	BackOff           backoff.BackOff
	ConnectionTimeout time.Duration
}

// DefaultManagerConfig returns a ManagerConfig with sane defaults
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		ConnectionTimeout: 20 * time.Second,
		BackOff:           backoff.NewExponentialBackOff(),
	}
}

type Manager struct {
	sync.Mutex
	address     *url.URL
	sockets     map[string]*Socket
	conn        *engine.Conn
	fromSockets chan socket.Packet
	socketCtx   context.Context
	cancel      context.CancelFunc
	opts        *ManagerConfig
}

func handleDisconnects(manager *Manager, disconnects chan bool) {
	for {
		select {
		case <-disconnects:
			manager.cancel()
			manager.conn = nil
			err := backoff.Retry(connectContext(manager.socketCtx, manager), backoff.WithContext(manager.opts.BackOff, manager.socketCtx))

			if err != nil {
				fmt.Println(err)
			}

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

			ns := message.Namespace

			if ns == "" {
				ns = "/"
			}

			if socket, ok := manager.sockets[ns]; ok {
				select {
				case <-ctx.Done():
					return
				case socket.incomingPackets <- message:
				}
			}
		}
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

			encodedData, err := packet.Encode()

			if err != nil || len(encodedData) == 0 {
				continue
			}

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

	nsSocket.events = make(map[string]reflect.Value)

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

func (m *Manager) connectContext(ctx context.Context) error {
	managerCtx, cancel := context.WithCancel(ctx)

	conn, err := engine.DialContext(managerCtx, m.address.String())

	if err != nil {
		cancel()
		return err
	}

	m.conn = conn
	m.socketCtx = ctx
	m.cancel = cancel

	go receiveFromEngine(managerCtx, m, conn.Receive)
	go sendToEngine(managerCtx, m, m.fromSockets, conn.Send)
	go handleDisconnects(m, conn.Disconnects)

	_, err = m.Namespace("/")

	if err != nil {
		cancel()
		return err
	}

	return nil
}

func connectContext(ctx context.Context, m *Manager) func() error {
	return func() error {
		return m.connectContext(ctx)
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

	manager.fromSockets = make(chan socket.Packet)
	manager.sockets = make(map[string]*Socket)
	manager.address = parsedAddress
	manager.opts = cfg

	err = backoff.Retry(connectContext(ctx, manager), backoff.WithContext(cfg.BackOff, ctx))

	return manager, nil
}
