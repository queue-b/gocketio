package gocket

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/queue-b/gocket/socket"

	"github.com/queue-b/gocket/engine"
)

type Manager struct {
	sync.Mutex
	address           *url.URL
	sockets           map[string]*Socket
	conn              *engine.Conn
	outgoing          chan socket.Packet
	ctx               context.Context
	cancel            context.CancelFunc
	reconnectInterval time.Duration
	reconnects        chan time.Duration
}

func handleDisconnects(ctx context.Context, manager *Manager, disconnects chan bool) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-disconnects:
			manager.cancel()
			manager.conn = nil
			manager.reconnects <- manager.reconnectInterval
		}
	}
}

func reconnectToServer(ctx context.Context, manager *Manager, reconnects chan time.Duration) {
	for {
		select {
		case <-ctx.Done():
			fmt.Println("ctx.Done, killing manager.reconnectToServer")
			return
		case interval := <-reconnects:
			if !manager.Connected() {
				fmt.Printf("Waiting %v for reconnect\n", interval)
				t := time.NewTimer(interval)

				select {
				case <-ctx.Done():
					return
				case <-t.C:
					_, _, err := manager.connectContext(ctx)

					if err != nil {
						fmt.Println(err)
						manager.reconnectInterval = manager.reconnectInterval * 2
						reconnects <- manager.reconnectInterval
					}
				}
			}
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
	nsSocket.outgoingPackets = m.outgoing
	nsSocket.namespace = namespace
	nsSocket.incomingPackets = make(chan socket.Packet)

	nsSocket.events = make(map[string]reflect.Value)

	go receiveFromManager(m.ctx, nsSocket, nsSocket.incomingPackets)

	if !socket.IsRootNamespace(namespace) {
		connectPacket := socket.Packet{}
		connectPacket.Namespace = namespace
		connectPacket.Type = socket.Connect

		m.outgoing <- connectPacket
	}

	m.sockets[namespace] = nsSocket

	return nsSocket, nil
}

func (m *Manager) connectContext(ctx context.Context) (*Manager, *Socket, error) {
	managerCtx, cancel := context.WithCancel(ctx)

	conn, err := engine.DialContext(managerCtx, m.address.String())

	if err != nil {
		cancel()
		return nil, nil, err
	}

	m.conn = conn
	m.ctx = ctx
	m.cancel = cancel

	go receiveFromEngine(managerCtx, m, conn.Receive)
	go sendToEngine(managerCtx, m, m.outgoing, conn.Send)
	go handleDisconnects(managerCtx, m, conn.Disconnects)

	s, err := m.Namespace("/")

	if err != nil {
		cancel()
		return nil, nil, err
	}

	return m, s, nil
}

// DialContext attempts to connect to the Socket.IO server at address
func DialContext(ctx context.Context, address string) (*Manager, *Socket, error) {
	manager := &Manager{}

	parsedAddress, err := url.Parse(address)

	if err != nil {
		return nil, nil, err
	}

	if parsedAddress.Path == "/" || parsedAddress.Path == "" {
		newPath := strings.TrimRight(parsedAddress.Path, "/")
		newPath += "/socket.io/"
		parsedAddress.Path = newPath
	}

	manager.outgoing = make(chan socket.Packet)
	manager.sockets = make(map[string]*Socket)
	manager.reconnects = make(chan time.Duration, 10)
	manager.address = parsedAddress
	manager.reconnectInterval = 500 * time.Millisecond

	go reconnectToServer(ctx, manager, manager.reconnects)

	return manager.connectContext(ctx)
}
