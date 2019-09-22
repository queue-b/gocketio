package gocket

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/queue-b/gocket/socket"
)

type SocketState int

const (
	Disconnected SocketState = 1 << iota
	Connecting
	Connected
	Reconnecting
	Errored
)

type AckFunc func(id int, data interface{})

var ErrNoHandler = errors.New("No handler registered")
var ErrNotConnected = errors.New("Not connected")

// Socket is a Socket.IO socket that can send messages to and
// received messages from a namespace
type Socket struct {
	sync.Mutex
	events          map[string]reflect.Value
	acks            map[int]AckFunc
	namespace       string
	incomingPackets chan socket.Packet
	outgoingPackets chan socket.Packet
	ackCounter      int
	currentState    SocketState
	id              string
}

// State returns the current state of the socket
func (s *Socket) State() SocketState {
	return s.currentState
}

// Namespace returns the namespace that this socket uses to send and receive
// events from
func (s *Socket) Namespace() string {
	return s.namespace
}

// On adds the event handler for the event
func (s *Socket) On(event string, handler interface{}) error {
	err := isFunction(handler)

	if err != nil {
		return err
	}

	s.Lock()
	defer s.Unlock()

	s.events[event] = reflect.ValueOf(handler)
	return nil
}

// Off removes the event handler for the event
func (s *Socket) Off(event string) {
	s.Lock()
	defer s.Unlock()
	delete(s.events, event)
}

// Send raises a "message" event on the server
func (s *Socket) Send(data ...interface{}) error {
	return s.Emit("message", data...)
}

// Emit raises an event on the server
func (s *Socket) Emit(event string, data ...interface{}) error {
	s.Lock()
	defer s.Unlock()

	if s.currentState != Connected {
		return ErrNotConnected
	}

	message := socket.Packet{}

	// TODO: Check ...data for []byte elements and emit binary events
	message.Type = socket.Event
	message.Namespace = s.namespace

	messageData := make([]interface{}, len(data)+1)
	messageData[0] = event

	if len(messageData) > 1 {
		copy(messageData[1:], data)
	}

	message.Data = messageData

	s.outgoingPackets <- message

	return nil
}

// EmitWithAck raises an event on the server, and registers a callback that is invoked
// when the server acknowledges receipt
func (s *Socket) EmitWithAck(event string, ackFunc AckFunc, data ...interface{}) error {
	s.Lock()
	defer s.Unlock()

	if s.currentState != Connected {
		return ErrNotConnected
	}

	message := socket.Packet{}
	message.Type = socket.Event
	message.Namespace = s.namespace

	messageData := make([]interface{}, len(data)+1)
	messageData[0] = event

	if len(messageData) > 1 {
		copy(messageData[1:], data)
	}

	message.Data = messageData
	ackCount := s.ackCounter
	message.ID = &ackCount
	s.ackCounter++

	s.acks[ackCount] = ackFunc

	s.outgoingPackets <- message

	return nil
}

func (s *Socket) raiseEvent(eventName string, data []interface{}) ([]interface{}, error) {
	s.Lock()
	defer s.Unlock()

	if handler, ok := s.events[eventName]; ok {
		var handlerResults []interface{}

		args, err := convertUnmarshalledJSONToReflectValues(handler, data)

		if err != nil {
			return nil, err
		}

		vals := handler.Call(args)

		if vals != nil {
			for _, v := range vals {
				handlerResults = append(handlerResults, v.Interface())
			}
		}

		return handlerResults, nil
	}

	return nil, ErrNoHandler
}

func (s *Socket) raiseAck(id int, data interface{}) error {
	var handler AckFunc
	var ok bool
	if handler, ok = s.acks[id]; !ok {
		return ErrNoHandler
	}

	handler(id, data)

	delete(s.acks, id)

	return nil
}

func (s *Socket) sendAck(id int, data []interface{}) {
	p := socket.Packet{
		Type:      socket.Ack,
		ID:        &id,
		Namespace: s.namespace,
		Data:      data,
	}

	s.outgoingPackets <- p
}

func (s *Socket) setStateFromPacketType(p socket.PacketType) {
	s.Lock()
	defer s.Unlock()

	if p == socket.Disconnect {
		s.currentState = Disconnected
	}

	if p == socket.Connect {
		s.currentState = Connected
	}

	if p == socket.Error {
		s.currentState = Errored
	}
}

func receiveFromManager(ctx context.Context, s *Socket, incomingPackets chan socket.Packet) {
	for {
		select {
		case <-ctx.Done():
			fmt.Println("ctx.Done, Killing socket receiveFromManager")
			return
		case packet, ok := <-incomingPackets:
			if !ok {
				fmt.Println("Invalid read, killing socket receiveFromManager")
				return
			}

			s.setStateFromPacketType(packet.Type)

			if packet.Type == socket.Event || packet.Type == socket.BinaryEvent {
				data := packet.Data.([]interface{})
				eventName := data[0].(string)

				if len(data) > 1 {
					data = data[1:]
				} else {
					data = make([]interface{}, 0)
				}

				results, err := s.raiseEvent(eventName, data)

				if err != nil {
					continue
				}

				if packet.ID != nil {
					s.sendAck(*packet.ID, results)
				}
			}

			if packet.Type == socket.Ack || packet.Type == socket.BinaryAck {
				s.raiseAck(*packet.ID, packet.Data)
			}
		}
	}
}
