package engine

import (
	"context"
	"errors"
	"log"
	"sync"
	"testing"
	"time"
)

type MockPacketConn struct {
	sync.RWMutex
	id             string
	supportsBinary bool
	read           func() (Packet, error)
	sequence       int32
	writeChan      chan Packet
	write          func(Packet) error
	close          func() error
	state          PacketConnState
}

func NewMockPacketConn(id string, supportsBinary bool, read func() (Packet, error), write func(Packet) error, close func() error) *MockPacketConn {
	return &MockPacketConn{
		sync.RWMutex{},
		id,
		supportsBinary,
		read,
		0,
		make(chan Packet, 10000),
		write,
		close,
		Connected,
	}
}

func (m *MockPacketConn) ID() string                { return m.id }
func (m *MockPacketConn) SupportsBinary() bool      { return m.supportsBinary }
func (m *MockPacketConn) Read() (Packet, error)     { return m.read() }
func (m *MockPacketConn) Write(packet Packet) error { return m.write(packet) }
func (m *MockPacketConn) Close() error {
	m.Lock()
	defer m.Unlock()
	m.state = Disconnected
	return m.close()
}
func (m *MockPacketConn) State() PacketConnState {
	m.RLock()
	defer m.RUnlock()
	return m.state
}

func DefaultMockPacketConn(sequence []Packet) *MockPacketConn {
	conn := NewMockPacketConn("test", true, nil, nil, func() error { return nil })

	conn.read = func() (Packet, error) {
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

	conn.write = func(packet Packet) error {
		conn.writeChan <- packet
		return nil
	}

	return conn
}

func TestKeepAlive(t *testing.T) {
	conn := DefaultMockPacketConn(packetSequenceOpen)

	keepConn := NewKeepAliveConn(conn, 100, make(chan Packet))
	keepConn.KeepAliveContext(context.Background())

	time.Sleep(1 * time.Second)

	// Make sure the server isn't closed before interval + timeout
	if keepConn.State() == Disconnected {
		log.Fatalf("[%v] Expected connection to be Connected", t.Name())
	}
}

func TestKeepAliveAccessors(t *testing.T) {
	conn := DefaultMockPacketConn(packetSequenceOpen)

	keepConn := NewKeepAliveConn(conn, 100, make(chan Packet))
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
	conn := DefaultMockPacketConn(packetSequenceOpen)

	keepConn := NewKeepAliveConn(conn, 100, make(chan Packet))
	keepConn.KeepAliveContext(context.Background())

	time.Sleep(3 * time.Second)

	// Make sure the server isn't closed before interval + timeout
	if keepConn.State() == Connected {
		log.Fatalf("[%v] Expected connection to be Disconnected", t.Name())
	}
}

func TestKeepAliveReadWrite(t *testing.T) {
	conn := DefaultMockPacketConn(packetSequenceNormal)

	outPackets := make(chan Packet)

	keepConn := NewKeepAliveConn(conn, 100, outPackets)
	keepConn.KeepAliveContext(context.Background())

	packet := <-keepConn.Read()

	if packet == nil {
		t.Fatalf("[%v] Expected packet, got nil", t.Name())
	}

	if packet.GetType() != Message {
		t.Fatalf("[%v] Expected Message packet, got %v", t.Name(), packet.GetType())
	}

	testData := "Hello, world"
	packet = &StringPacket{Type: Message, Data: &testData}

	keepConn.Write() <- packet

	writtenPacket := <-conn.writeChan

	if writtenPacket.GetType() != Message {
		t.Fatalf("[%v] Expected Message packet, got %v", t.Name(), writtenPacket.GetType())
	}

}

func TestKeepAliveWriteError(t *testing.T) {
	conn := DefaultMockPacketConn(packetSequenceNormal)
	conn.write = func(packet Packet) error { return errors.New("Mock error") }

	outPackets := make(chan Packet)

	keepConn := NewKeepAliveConn(conn, 100, outPackets)
	keepConn.KeepAliveContext(context.Background())

	testData := "Hello, world"
	packet := &StringPacket{Type: Message, Data: &testData}

	keepConn.Write() <- packet

}
