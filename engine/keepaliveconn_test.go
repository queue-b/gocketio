package engine

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/queue-b/gocketio/engine/transport"
)

type mockPacketConn struct {
	sync.RWMutex
	id             string
	supportsBinary bool
	read           func() (transport.Packet, error)
	sequence       int32
	writeChan      chan transport.Packet
	write          func(transport.Packet) error
	close          func() error
	connected      bool
}

func newMockPacketConn(id string, supportsBinary bool, read func() (transport.Packet, error), write func(transport.Packet) error, close func() error) *mockPacketConn {
	return &mockPacketConn{
		sync.RWMutex{},
		id,
		supportsBinary,
		read,
		0,
		make(chan transport.Packet, 10000),
		write,
		close,
		true,
	}
}

func (m *mockPacketConn) ID() string                          { return m.id }
func (m *mockPacketConn) SupportsBinary() bool                { return m.supportsBinary }
func (m *mockPacketConn) Read() (transport.Packet, error)     { return m.read() }
func (m *mockPacketConn) Write(packet transport.Packet) error { return m.write(packet) }
func (m *mockPacketConn) Close() error {
	m.Lock()
	defer m.Unlock()
	m.connected = false
	return m.close()
}
func (m *mockPacketConn) Connected() bool {
	m.RLock()
	defer m.RUnlock()
	return m.connected
}

func defaultMockPacketConn(sequence []transport.Packet) *mockPacketConn {
	conn := newMockPacketConn("test", true, nil, nil, func() error { return nil })

	conn.read = func() (transport.Packet, error) {
		conn.Lock()
		defer conn.Unlock()

		if int(conn.sequence) < len(sequence) {
			packet := sequence[conn.sequence]
			conn.sequence++

			return packet, nil
		}

		time.Sleep(250 * time.Millisecond)

		return nil, nil
	}

	conn.write = func(packet transport.Packet) error {
		conn.writeChan <- packet
		return nil
	}

	return conn
}

func TestKeepAlive(t *testing.T) {
	t.Parallel()
	conn := defaultMockPacketConn(packetSequenceOpen)

	keepConn := NewKeepAliveConn(conn, 100, make(chan transport.Packet))
	keepConn.KeepAliveContext(context.Background())

	time.Sleep(1 * time.Second)

	// Make sure the server isn't closed before interval + timeout
	if !keepConn.Connected() {
		t.Fatalf("[%v] Expected connection to be Connected", t.Name())
	}
}

func TestKeepAliveAccessors(t *testing.T) {
	t.Parallel()
	conn := defaultMockPacketConn(packetSequenceOpen)

	keepConn := NewKeepAliveConn(conn, 100, make(chan transport.Packet))
	keepConn.KeepAliveContext(context.Background())

	id := keepConn.ID()

	if id == "" {
		t.Fatal("Empty ID")
	}

	supportsBinary := keepConn.SupportsBinary()

	if supportsBinary == false {
		t.Fatal("Supports binary")
	}

	keepConn.Close()

	err := keepConn.Close()

	if err != nil {
		t.Fatal("Double close caused error")
	}
}

func TestKeepAliveTimeout(t *testing.T) {
	t.Parallel()
	conn := defaultMockPacketConn(packetSequenceOpen)

	keepConn := NewKeepAliveConn(conn, 100, make(chan transport.Packet))
	keepConn.KeepAliveContext(context.Background())

	time.Sleep(3 * time.Second)

	// Make sure the server isn't closed before interval + timeout
	if keepConn.Connected() {
		t.Fatalf("[%v] Expected connection to be Disconnected", t.Name())
	}
}

func TestKeepAliveReadWrite(t *testing.T) {
	t.Parallel()
	conn := defaultMockPacketConn(packetSequenceNormal)

	outPackets := make(chan transport.Packet)

	keepConn := NewKeepAliveConn(conn, 100, outPackets)
	keepConn.KeepAliveContext(context.Background())

	packet := <-keepConn.Read()

	if packet == nil {
		t.Fatalf("[%v] Expected packet, got nil", t.Name())
	}

	if packet.GetType() != transport.Message {
		t.Fatalf("[%v] Expected Message packet, got %v", t.Name(), packet.GetType())
	}

	testData := "Hello, world"
	packet = &transport.StringPacket{Type: transport.Message, Data: &testData}

	keepConn.Write() <- packet

	writtenPacket := <-conn.writeChan

	if writtenPacket.GetType() != transport.Message {
		t.Fatalf("[%v] Expected Message packet, got %v", t.Name(), writtenPacket.GetType())
	}

}

func TestKeepAliveWriteError(t *testing.T) {
	t.Parallel()
	conn := defaultMockPacketConn(packetSequenceNormal)
	conn.write = func(packet transport.Packet) error { return errors.New("Mock error") }

	outPackets := make(chan transport.Packet)

	keepConn := NewKeepAliveConn(conn, 100, outPackets)
	keepConn.KeepAliveContext(context.Background())

	testData := "Hello, world"
	packet := &transport.StringPacket{Type: transport.Message, Data: &testData}

	keepConn.Write() <- packet

}
