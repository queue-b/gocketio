package gocket

import (
	"reflect"
	"sync"

	"github.com/queue-b/gocket/engine"

	"github.com/queue-b/gocket/socket"
)

type Socket struct {
	sync.Mutex
	events     map[string]reflect.Value
	acks       map[int]reflect.Value
	ackCounter int
	namespace  string
	client     engine.Conn
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
func (s *Socket) Off(event string, handler interface{}) {
	s.Lock()
	defer s.Unlock()
	delete(s.events, event)
}

// Emit
func (s *Socket) Emit(event string, data ...interface{}) error {
	message := socket.Packet{}

	// TODO: Check ...data for []byte elements and emit binary events
	message.Type = socket.Event
	message.Namespace = s.namespace

	messageData := make([]interface{}, len(data)+1)
	messageData[0] = event

	if len(messageData) > 1 {
		copy(messageData[:1], data)
	}

	encodedData, err := message.Encode()

	if err != nil {
		return err
	}

	first := string(encodedData[0])

	p := engine.StringPacket{}
	p.Type = engine.Message
	p.Data = &first

	s.client.Send <- &p

	// Packet has attachments
	if len(encodedData) > 1 {
		for _, v := range encodedData[1:] {
			b := engine.BinaryPacket{}
			b.Type = engine.Message
			b.Data = v

			s.client.Send <- &b
		}
	}

	return nil
}

func (s *Socket) receiveMessages() {
	d := socket.BinaryDecoder{}

	for {
		packet := <-s.client.Receive

		m, err := d.Decode(packet)

		if err != nil {
			continue
		}

		// TODO: Check if namespaces match
		if m.Type == socket.Event || m.Type == socket.BinaryEvent {
			s.Lock()
			defer s.Unlock()

			data := m.Data.([]interface{})
			eventName := data[0].(string)

			if handler, ok := s.events[eventName]; ok {
				args, err := convertUnmarshalledJSONToReflectValues(handler, m.Data)

				if err != nil {
					continue
				}

				handler.Call(args)
			}
		}
	}
}

// EmitWithAck
func (s *Socket) EmitWithAck(event string, ackHandler interface{}, data ...interface{}) {

}
