package gocketio

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/queue-b/gocketio/engine"

	"github.com/queue-b/gocketio/socket"
)

type mockConn struct {
	id             string
	supportsBinary bool
	read           chan engine.Packet
	write          chan engine.Packet
	closeError     error
	opened         chan engine.OpenData
}

type testBinary struct {
	Bytes []byte
	Label string
}

func newMockConn(id string, supportsBinary bool, read, write chan engine.Packet, closeError error) *mockConn {
	return &mockConn{
		id,
		supportsBinary,
		read,
		write,
		closeError,
		make(chan engine.OpenData, 1),
	}
}

func (m *mockConn) ID() string                           { return m.id }
func (m *mockConn) SupportsBinary() bool                 { return m.supportsBinary }
func (m *mockConn) Read() <-chan engine.Packet           { return m.read }
func (m *mockConn) Write() chan<- engine.Packet          { return m.write }
func (m *mockConn) Close() error                         { return m.closeError }
func (m *mockConn) KeepAliveContext(ctx context.Context) { return }
func (m *mockConn) State() engine.PacketConnState        { return engine.Connected }
func (m *mockConn) Opened() <-chan engine.OpenData       { return m.opened }

func TestSendToEngine(t *testing.T) {
	m := &Manager{}
	m.outgoingPackets = make(chan engine.Packet, 1)
	m.conn = newMockConn("test", true, make(chan engine.Packet), m.outgoingPackets, nil)
	m.sockets = make(map[string]*Socket)
	m.fromSockets = make(chan socket.Packet, 1)

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	go m.writeToEngineContext(ctx)

	m.fromSockets <- socket.Packet{Type: socket.Event, Namespace: "/", Data: "pants"}

	p := <-m.outgoingPackets

	// TODO: Additional tests to make sure that the packet was encoded correctly
	if p.GetType() != engine.Message {
		t.Errorf("Expected Message, got %v", p.GetType())
	}
}

func TestSendToEngineBinary(t *testing.T) {
	m := &Manager{}
	m.outgoingPackets = make(chan engine.Packet, 2)
	m.conn = newMockConn("test", true, make(chan engine.Packet), m.outgoingPackets, nil)
	m.sockets = make(map[string]*Socket)
	m.fromSockets = make(chan socket.Packet, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)

	defer cancel()

	go m.writeToEngineContext(ctx)

	m.fromSockets <- socket.Packet{Type: socket.BinaryEvent, Namespace: "/", Data: testBinary{Bytes: []byte{0, 1, 2}, Label: "bubbles"}}

	var results []engine.Packet

	for i := 0; i < 2; i++ {
		select {
		case <-ctx.Done():
		case result := <-m.outgoingPackets:
			results = append(results, result)
		}
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 packets, got %v\n", len(m.outgoingPackets))
	}

	p := results[0]

	// TODO: Additional tests to make sure that the packet was encoded correctly
	if p.GetType() != engine.Message {
		t.Fatalf("Expected Message, got %v", p.GetType())
	}

	p = results[1]

	// TODO: Additional tests to make sure that the packet was encoded correctly
	if p.GetType() != engine.Message {
		t.Fatalf("Expected Message, got %v", p.GetType())
	}
}

func TestReceiveFromEngine(t *testing.T) {
	s := &Socket{}
	s.events = sync.Map{}
	s.incomingPackets = make(chan socket.Packet)

	enginePackets := make(chan engine.Packet, 1)

	m := &Manager{}
	m.conn = newMockConn("test", true, enginePackets, make(chan engine.Packet), nil)
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

func TestReceiveFromEngineNamespaceWithNoMatchingSocket(t *testing.T) {
	s := &Socket{}
	s.events = sync.Map{}
	s.incomingPackets = make(chan socket.Packet)

	enginePackets := make(chan engine.Packet, 1)

	m := &Manager{}
	m.conn = newMockConn("test", true, enginePackets, make(chan engine.Packet), nil)
	m.sockets = make(map[string]*Socket)
	m.sockets["/"] = s

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	go m.readFromEngineContext(ctx)

	p := socket.Packet{}
	p.Type = socket.Event
	p.Namespace = "/unknown"
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

	timer := time.NewTimer(20 * time.Millisecond)

	select {
	case <-timer.C:
	case <-s.incomingPackets:
		t.Fatal("Delivered packet to wrong socket")
	}
}

func TestManagerConnected(t *testing.T) {
	m := &Manager{
		sockets: make(map[string]*Socket),
		conn:    newMockConn("test", true, make(chan engine.Packet), make(chan engine.Packet), nil),
	}

	if m.Connected() != true {
		t.Fatal("Expected true, got false")
	}
}

func TestManagerNamespaceWithExistingSocket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		sockets:   make(map[string]*Socket),
		cancel:    cancel,
		socketCtx: ctx,
		conn:      newMockConn("test", true, make(chan engine.Packet), make(chan engine.Packet), nil),
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
		conn:        newMockConn("test", true, make(chan engine.Packet), make(chan engine.Packet), nil),
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

func TestDisconnect(t *testing.T) {
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
		address:         addr,
		socketCtx:       ctx,
		sockets:         make(map[string]*Socket),
		fromSockets:     make(chan socket.Packet, 1),
	}

	err = m.connectContext(ctx)

	if err != nil {
		t.Fatalf("ConnectContext failed with %v\n", err)
	}

	time.Sleep(500 * time.Millisecond)

	m.conn.Close()

	cancel()
}

func TestConnectContext(t *testing.T) {
	mux := http.NewServeMux()

	srv, address := engine.CreateTestSocketIOServer(mux)

	mux.HandleFunc("/socket.io/", engine.CreateOpenPingPongHandler())

	go srv.Start()
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr, err := url.Parse(address)

	if err != nil {
		t.Errorf("Invalid address %v\n", err)
	}

	outgoing := make(chan engine.Packet, 1)

	m := &Manager{
		opts:            DefaultManagerConfig(),
		outgoingPackets: outgoing,
		address:         addr,
		socketCtx:       ctx,
		sockets:         make(map[string]*Socket),
		fromSockets:     make(chan socket.Packet, 1),
	}

	err = m.connectContext(ctx)

	if err != nil {
		t.Fatalf("ConnectContext failed with %v\n", err)
	}

}

func TestDialContext(t *testing.T) {
	mux := http.NewServeMux()

	srv, address := engine.CreateTestSocketIOServer(mux)

	mux.HandleFunc("/socket.io/", engine.CreateOpenPingPongHandler())

	go srv.Start()
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := DialContext(ctx, address, DefaultManagerConfig())

	if err != nil {
		t.Fatalf("DialContext failed with %v\n", err)
	}
}

func TestFixupAddress(t *testing.T) {
	addr := "http://test.google.com/"

	_, err := fixupAddress(addr, nil)

	if err != nil {
		t.Fatalf("Unable to parse valid addr %v", addr)
	}

	addr = "htp:/test.google.com/"

	_, err = fixupAddress(addr, nil)

	if err == nil {
		t.Fatalf("Parsed invalid address %v", addr)
	}
}
