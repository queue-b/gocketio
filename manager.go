package gocket

import (
	"context"
	"fmt"
	"sync"

	"github.com/queue-b/gocket/socket"

	"github.com/queue-b/gocket/engine"
)

type Manager struct {
	sync.Mutex
	sockets  map[string]*Socket
	conn     *engine.Conn
	outgoing chan socket.Packet
}

func receiveFromEngine(ctx context.Context, manager *Manager, inputPackets chan engine.Packet) {
	d := socket.BinaryDecoder{}

	for {
		select {
		case <-ctx.Done():
			return
		case packet := <-inputPackets:
			message, err := d.Decode(packet)

			if err != nil && err != socket.ErrWaitingForMorePackets {
				fmt.Println(err)
				continue
			}

			fmt.Println("Received message")

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
			return
		case packet := <-outputPackets:
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

	connectPacket := socket.Packet{}
	connectPacket.Namespace = namespace
	connectPacket.Type = socket.Connect

	m.outgoing <- connectPacket
	m.sockets[namespace] = nsSocket

	return nsSocket, nil
}

func Dial(address string) (*Manager, error) {
	manager := &Manager{}

	conn, err := engine.Dial(address)

	if err != nil {
		return nil, err
	}

	manager.conn = conn
	manager.outgoing = make(chan socket.Packet)

	return manager, nil
}