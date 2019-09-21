package gocket

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/cenkalti/backoff"

	"github.com/queue-b/gocket/engine"

	"github.com/queue-b/gocket/socket"
)

func TestSendToEngine(t *testing.T) {
	s := &Socket{}
	s.events = make(map[string]reflect.Value)
	s.outgoingPackets = make(chan socket.Packet)

	m := &Manager{}
	m.sockets = make(map[string]*Socket)

	enginePackets := make(chan engine.Packet)

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	go sendToEngine(ctx, m, s.outgoingPackets, enginePackets)

	s.Emit("fancy", "pants")

	p := <-enginePackets

	// TODO: Additional tests to make sure that the packet was encoded correctly
	if p.GetType() != engine.Message {
		t.Errorf("Expected Message, got %v", p.GetType())
	}
}

func TestReceiveFromEngine(t *testing.T) {
	s := &Socket{}
	s.events = make(map[string]reflect.Value)
	s.incomingPackets = make(chan socket.Packet)

	m := &Manager{}
	m.sockets = make(map[string]*Socket)
	m.sockets["/"] = s

	enginePackets := make(chan engine.Packet)

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	go receiveFromEngine(ctx, m, enginePackets)

	p := socket.Packet{}
	p.Type = socket.Event
	p.Namespace = "/"
	p.Data = []interface{}{"fancy", "pants"}

	encoded, err := p.Encode()

	if err != nil {
		t.Errorf("Error encoding data %v\n", err)
	}

	encodedData := string(encoded[0])

	ep := engine.StringPacket{
		Type: engine.Message,
		Data: &encodedData,
	}

	enginePackets <- &ep

	rp := <-s.incomingPackets

	if rp.Type != socket.Event {
		t.Errorf("Expected Event, got %v", rp.Type)
	}
}

func TestHandleDisconnect(t *testing.T) {
	disconnects := make(chan struct{})

	invoked := make(chan struct{}, 1)

	reconnect := func() error {
		invoked <- struct{}{}
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &Manager{}
	manager.cancel = cancel
	manager.socketCtx = ctx
	manager.opts = DefaultManagerConfig()

	go handleDisconnect(manager, reconnect, disconnects)

	disconnects <- struct{}{}

	if len(invoked) != 1 {
		t.Fatal("Expected reconnect function to be invoked at least 1 time")
	}

}

func TestHandleDisconnectReconnectError(t *testing.T) {
	disconnects := make(chan struct{})

	invoked := make(chan struct{}, 1)

	reconnect := func() error {
		return backoff.Permanent(errors.New("Real bad stuff"))
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &Manager{}
	manager.cancel = cancel
	manager.socketCtx = ctx
	manager.opts = DefaultManagerConfig()

	go handleDisconnect(manager, reconnect, disconnects)

	disconnects <- struct{}{}

	if len(invoked) != 0 {
		t.Fatal("Expected reconnect function to be fail")
	}
}

func TestManagerNamespaceWithExistingSocket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		sockets:   make(map[string]*Socket),
		cancel:    cancel,
		socketCtx: ctx,
	}

	s := &Socket{}
	m.sockets["/"] = s

	ns, err := m.Namespace("/")

	if err != nil {
		t.Fatalf("Expected manager.Namespace to not return an error %v\n", err)
	}

	if s != ns {
		t.Fatal("Expected manager.Namespace to return existing socket")
	}
}

func TestManagerNamespaceWithNewSocket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	fromSockets := make(chan socket.Packet, 1)

	m := &Manager{
		sockets:     make(map[string]*Socket),
		cancel:      cancel,
		socketCtx:   ctx,
		fromSockets: fromSockets,
	}

	ns, err := m.Namespace("/fancy")

	if err != nil {
		t.Fatalf("Expected manager.Namespace to not return an error %v\n", err)
	}

	if ns == nil {
		t.Fatal("Expected manager.Namespace to return a valid socket")
	}

	if len(m.fromSockets) != 1 {
		t.Fatal("Expected manager.Namespace to send a connect message")
	}

}
