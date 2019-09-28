package gocketio

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/queue-b/gocketio/socket"
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
	sync.RWMutex
	events          sync.Map
	acks            sync.Map
	namespace       string
	incomingPackets chan socket.Packet
	outgoingPackets chan socket.Packet
	ackCounter      int32
	currentState    SocketState
	id              string
	err             error
}

// State returns the current state of the socket
func (s *Socket) State() SocketState {
	s.RLock()
	defer s.RUnlock()
	return s.currentState
}

// ID returns the ID of the socket
func (s *Socket) ID() string {
	return s.id
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

	s.events.Store(event, reflect.ValueOf(handler))
	return nil
}

// Off removes the event handler for the event
func (s *Socket) Off(event string) {
	s.events.Delete(event)
}

// Send raises a "message" event on the server
func (s *Socket) Send(data ...interface{}) error {
	return s.Emit("message", data...)
}

func (s *Socket) checkState() error {
	state := s.State()

	if state == Errored {
		return s.err
	}

	if state != Connected {
		return ErrNotConnected
	}

	return nil
}

// Emit raises an event on the server
func (s *Socket) Emit(event string, data ...interface{}) error {
	err := s.checkState()

	if err != nil {
		return err
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
	err := s.checkState()

	if err != nil {
		return err
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
	ackCount := int(s.ackCounter)
	message.ID = &ackCount
	atomic.AddInt32(&s.ackCounter, 1)

	s.acks.Store(ackCount, ackFunc)

	s.outgoingPackets <- message

	return nil
}

func (s *Socket) raiseEvent(eventName string, data []interface{}) ([]interface{}, error) {
	s.Lock()
	defer s.Unlock()

	if handlerVal, ok := s.events.Load(eventName); ok {
		handler := handlerVal.(reflect.Value)
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
	var handlerVal interface{}
	var ok bool
	if handlerVal, ok = s.acks.Load(id); !ok {
		return ErrNoHandler
	}

	handler := handlerVal.(AckFunc)

	handler(id, data)
	s.acks.Delete(id)

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

	if s.currentState == Errored {
		return
	}

	if p == socket.Disconnect {
		s.currentState = Disconnected
	}

	if p == socket.Connect {
		s.currentState = Connected
	}
}

func (s *Socket) setError(err error) {
	s.Lock()
	defer s.Unlock()

	s.currentState = Errored
	s.err = err
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
					fmt.Println("Error raising event", err)
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
