package gocketio

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/queue-b/gocketio/engine"

	"github.com/queue-b/gocketio/socket"
)

type mockConn struct{}

func (m *mockConn) ID() string           { return "hello" }
func (m *mockConn) SupportsBinary() bool { return true }

func TestSendToEngine(t *testing.T) {
	m := &Manager{}
	m.outgoingPackets = make(chan engine.Packet, 1)
	m.conn = engine.NewMockConn("test", true, make(chan engine.Packet), m.outgoingPackets, nil)
	m.sockets = make(map[string]*Socket)

	s := &Socket{}
	s.events = sync.Map{}
	m.fromSockets = make(chan socket.Packet, 1)

	s.outgoingPackets = m.fromSockets
	s.currentState = Connected

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	go m.writeToEngineContext(ctx)

	s.Emit("fancy", "pants")

	p := <-m.outgoingPackets

	// TODO: Additional tests to make sure that the packet was encoded correctly
	if p.GetType() != engine.Message {
		t.Errorf("Expected Message, got %v", p.GetType())
	}
}

func TestReceiveFromEngine(t *testing.T) {
	s := &Socket{}
	s.events = sync.Map{}
	s.incomingPackets = make(chan socket.Packet)
	s.currentState = Connected

	enginePackets := make(chan engine.Packet, 1)

	m := &Manager{}
	m.conn = engine.NewMockConn("test", true, enginePackets, make(chan engine.Packet), nil)
	m.sockets = make(map[string]*Socket)
	m.sockets["/"] = s

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	go m.readFromEngineContext(ctx)

	p := socket.Packet{}
	p.Type = socket.Event
	p.Namespace = "/"
	p.Data = []interface{}{"fancy", "pants"}

	encoded, err := p.Encode(true)

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

func TestManagerNamespaceWithExistingSocket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		sockets:   make(map[string]*Socket),
		cancel:    cancel,
		socketCtx: ctx,
		conn:      engine.NewMockConn("test", true, make(chan engine.Packet), make(chan engine.Packet), nil),
	}

	s := &Socket{}
	s.currentState = Connected
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
		conn:        engine.NewMockConn("test", true, make(chan engine.Packet), make(chan engine.Packet), nil),
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

func TestConnectContext(t *testing.T) {
	mux := http.NewServeMux()

	srv, address := engine.CreateTestSocketIOServer(mux)

	mux.HandleFunc("/socket.io/", engine.CreateOpenPingPongHandler())

	go srv.Start()
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())

	addr, err := url.Parse(address)

	if err != nil {
		t.Errorf("Invalid address %v\n", err)
	}

	outgoing := make(chan engine.Packet, 1)

	m := &Manager{
		opts:            DefaultManagerConfig(),
		outgoingPackets: outgoing,
		conn:            engine.NewMockConn("test", true, make(chan engine.Packet, 1), outgoing, nil),
		address:         addr,
		socketCtx:       ctx,
		sockets:         make(map[string]*Socket),
		fromSockets:     make(chan socket.Packet, 1),
	}

	err = m.connectContext(ctx)

	cancel()

	if err != nil {
		t.Fatalf("ConnectContext failed with %v\n", err)
	}

}
