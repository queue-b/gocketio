// Package gocketio provides a client for the https://socket.io/ protocol
package gocketio

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/queue-b/gocketio/socket"
)

// Javascript Number.MAX_SAFE_INTEGER
var maxSafeInteger = int64(math.Pow(2, 53) - 1)

// AckFunc is a func that can be called in response to receiving an ACK packet
type AckFunc func(id int64, data interface{})

var errNoHandler = errors.New("No handler registered")

// ErrBlacklistedEvent is returned when an attempt is made to Emit a reserved event from a Socket
var ErrBlacklistedEvent = errors.New("Blacklisted event")

// Socket sends messages to and receive messages from a Socket.IO Namespace
// https://socket.io/docs/rooms-and-namespaces/
type Socket struct {
	sync.RWMutex
	events          sync.Map
	acks            sync.Map
	namespace       string
	incomingPackets chan socket.Packet
	outgoingPackets chan socket.Packet
	// ackCounter is a bit tricky. The largest number we should ever see from or send to JS is
	// Number.MAX_SAFE_INTEGER
	// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Number/MAX_SAFE_INTEGER
	// Which is currently ((2^53) - 1)
	ackCounter    int64
	isConnected   bool
	id            string
	err           error
	destroyOnce   sync.Once
	openOnce      sync.Once
	cancelReceive context.CancelFunc
	errHandler    func(data interface{})
}

// Connected returns true if the Socket is connected, false otherwise
//
// The socket is considered to be connected if
// 1. The Manager that created the Socket is connected and
// 2. The socket has not received a DISCONNCT message from the server
func (s *Socket) Connected() bool {
	s.RLock()
	defer s.RUnlock()
	return s.isConnected
}

// ID returns the ID of the socket
func (s *Socket) ID() string {
	s.RLock()
	defer s.RUnlock()
	return s.id
}

// Namespace returns the Namespace that this socket is
// connected to
func (s *Socket) Namespace() string {
	return s.namespace
}

// SetErrorHandler sets the error handler that is invoked when Error
// packets are received
func (s *Socket) SetErrorHandler(handler func(data interface{})) {
	s.Lock()
	defer s.Unlock()

	s.errHandler = handler
}

// There are certain events that users are not allowed to emit
// https://github.com/socketio/socket.io-client/blob/71d7b799652d3c80b00b24d99b33b626841e5631/lib/socket.js#L28
func isBlacklisted(event string) bool {
	switch event {
	case "connect":
		fallthrough
	case "connect_error":
		fallthrough
	case "connect_timeout":
		fallthrough
	case "connecting":
		fallthrough
	case "disconnect":
		fallthrough
	case "error":
		fallthrough
	case "reconnect":
		fallthrough
	case "reconnect_attempt":
		fallthrough
	case "reconnect_failed":
		fallthrough
	case "reconnect_error":
		fallthrough
	case "reconnecting":
		fallthrough
	case "ping":
		fallthrough
	case "pong":
		return true
	default:
		return false
	}
}

func newSocket(namespace, id string, outgoing chan socket.Packet) *Socket {
	nsSocket := &Socket{}
	nsSocket.outgoingPackets = outgoing
	nsSocket.namespace = namespace
	nsSocket.incomingPackets = make(chan socket.Packet)

	nsSocket.events = sync.Map{}
	nsSocket.acks = sync.Map{}
	nsSocket.id = fmt.Sprintf("%v#%v", namespace, id)

	return nsSocket
}

func (s *Socket) onOpen(ctx context.Context, id string) {
	s.Lock()
	s.id = id
	s.Unlock()

	s.openOnce.Do(func() {
		dCtx, cancel := context.WithCancel(ctx)
		s.cancelReceive = cancel
		go s.readFromManager(dCtx)
	})

	if !socket.IsRootNamespace(s.namespace) {
		connectPacket := socket.Packet{}
		connectPacket.Namespace = s.namespace
		connectPacket.Type = socket.Connect

		s.outgoingPackets <- connectPacket
	}
}

// On sets the event handler for the event.
// It returns an error if the event name is blacklisted, or if the
// handler is not a func
//
// There may be exactly one handler for an event at any time. Once an
// event handler has been set, subsequent calls to On will replace
// the existing handler.
//
// The On method is safe for use by multiple concurrent goroutines;
// however, the order in which the event handlers are added is not
// strictly defined.
func (s *Socket) On(event string, handler interface{}) error {
	if ok := isBlacklisted(event); ok {
		return ErrBlacklistedEvent
	}

	err := isFunction(handler)

	if err != nil {
		return err
	}

	s.events.Store(event, reflect.ValueOf(handler))
	return nil
}

// Off removes the event handler for the event. Calling Off() for
// an event that does not have a handler defined is a no-op.
//
// The Off method is safe for use by multiple concurrent goroutines
func (s *Socket) Off(event string) {
	s.events.Delete(event)
}

// Send raises a "message" event on the server
//
// The Send method is safe for use by multiple concurrent goroutines
func (s *Socket) Send(data ...interface{}) error {
	return s.Emit("message", data...)
}

// Emit raises an event on the server
//
// The Emit method is safe for use by multiple concurrent goroutines;
// however, the order that events are raised on the server is not
// guaranteed.
func (s *Socket) Emit(event string, data ...interface{}) error {
	if isBlacklisted(event) {
		return ErrBlacklistedEvent
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
//
// The Emit withAckMethod is safe for use by multiple concurrent goroutines;
// however, the order that events are raised on the server is not
// guaranteed.
func (s *Socket) EmitWithAck(event string, ackFunc AckFunc, data ...interface{}) error {
	if isBlacklisted(event) {
		return ErrBlacklistedEvent
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

	ackCount := atomic.AddInt64(&s.ackCounter, 1)

	message.ID = &ackCount

	s.acks.Store(ackCount, ackFunc)

	s.outgoingPackets <- message

	return nil
}

func (s *Socket) raiseEvent(eventName string, data []interface{}) (interface{}, error) {
	var handlerVal interface{}
	var ok bool

	if handlerVal, ok = s.events.Load(eventName); !ok {
		return nil, errNoHandler
	}

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

	// Special case for no result; if the empty handlerResults array is returned
	// it looks weird to the type system.
	// Also just setting it to nil and returning it doesn't work
	if len(handlerResults) == 0 {
		return nil, nil
	}

	return handlerResults, nil
}

func (s *Socket) raiseAck(id int64, data interface{}) error {
	var handlerVal interface{}
	var ok bool
	if handlerVal, ok = s.acks.Load(id); !ok {
		return errNoHandler
	}

	handler := handlerVal.(AckFunc)

	handler(id, data)
	s.acks.Delete(id)

	return nil
}

func (s *Socket) sendAck(id int64, data interface{}) {
	p := socket.Packet{
		Type:      socket.Ack,
		ID:        &id,
		Namespace: s.namespace,
		Data:      data,
	}

	s.outgoingPackets <- p
}

func (s *Socket) onConnect() {
	s.Lock()
	defer s.Unlock()

	s.isConnected = true
}

func (s *Socket) onDisconnect(server bool) {
	s.Lock()
	defer s.Unlock()
	if server {
		s.destroyOnce.Do(func() {
			if s.cancelReceive != nil {
				s.cancelReceive()
			}
		})
	}

	s.isConnected = false
}

func (s *Socket) onError(data interface{}) {
	s.RLock()
	defer s.RUnlock()

	if s.errHandler != nil {
		s.errHandler(data)
	}
}

func (s *Socket) readFromManager(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			fmt.Println("ctx.Done, Killing socket receiveFromManager")
			return
		case packet, ok := <-s.incomingPackets:
			if !ok {
				fmt.Println("Invalid read, killing socket receiveFromManager")
				return
			}

			if packet.Type == socket.Error {
				s.onError(packet.Data)
				continue
			}

			if packet.Type == socket.Connect {
				s.onConnect()
				continue
			}

			if packet.Type == socket.Disconnect {
				s.onDisconnect(true)
				continue
			}

			if packet.Type == socket.Event || packet.Type == socket.BinaryEvent {
				// Protect against a malformed packet
				if packet.Data != nil {
					switch data := packet.Data.(type) {
					case []interface{}:
						if len(data) == 0 {
							continue
						}

						switch eventName := data[0].(type) {
						case string:
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
					}
				}
			}

			if packet.Type == socket.Ack || packet.Type == socket.BinaryAck {
				if packet.ID != nil {
					s.raiseAck(*packet.ID, packet.Data)
				}
			}
		}
	}
}
