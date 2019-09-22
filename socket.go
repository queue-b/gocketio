package gocket

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/queue-b/gocket/socket"
)

type AckFunc func(id int, data interface{})

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

func (s *Socket) raiseEvent(eventName string, data []interface{}) error {
	s.Lock()
	defer s.Unlock()

	if handler, ok := s.events[eventName]; ok {
		// Check if there are items in the data array
		if len(data) > 1 {
			data = data[1:]

			args, err := convertUnmarshalledJSONToReflectValues(handler, data)

			if err != nil {
				return err
			}

			handler.Call(args)
		} else {
			handler.Call(nil)
		}
	}

	return nil
}

func (s *Socket) raiseAck(id int, data interface{}) error {
	var handler AckFunc
	var ok bool
	if handler, ok = s.acks[id]; !ok {
		return errors.New("Missing ack handler")
	}

	handler(id, data)

	delete(s.acks, id)

	return nil
}

func (s *Socket) sendAck(id int) {
	p := socket.Packet{
		Type:      socket.Ack,
		ID:        &id,
		Namespace: s.namespace,
	}

	s.outgoingPackets <- p
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

			if packet.Type == socket.Event || packet.Type == socket.BinaryEvent {
				data := packet.Data.([]interface{})
				eventName := data[0].(string)

				err := s.raiseEvent(eventName, data)

				if err != nil {
					continue
				}

				if packet.ID != nil {
					s.sendAck(*packet.ID)
				}
			}

			if packet.Type == socket.Ack || packet.Type == socket.BinaryAck {
				s.raiseAck(*packet.ID, packet.Data)
			}
		}
	}
}
