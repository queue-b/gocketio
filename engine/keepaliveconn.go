package engine

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

type MockConn struct {
	id             string
	supportsBinary bool
	read           chan Packet
	write          chan Packet
	closeError     error
}

func NewMockConn(id string, supportsBinary bool, read, write chan Packet, closeError error) *MockConn {
	return &MockConn{
		id,
		supportsBinary,
		read,
		write,
		closeError,
	}
}

func (m *MockConn) ID() string                           { return m.id }
func (m *MockConn) SupportsBinary() bool                 { return m.supportsBinary }
func (m *MockConn) Read() <-chan Packet                  { return m.read }
func (m *MockConn) Write() chan<- Packet                 { return m.write }
func (m *MockConn) Close() error                         { return m.closeError }
func (m *MockConn) KeepAliveContext(ctx context.Context) { return }

type Conn interface {
	ID() string
	SupportsBinary() bool
	Read() <-chan Packet
	Write() chan<- Packet
	Close() error
	KeepAliveContext(context.Context)
}

// KeepAliveConn wraps a Conn and handles the Ping/Pong process
type KeepAliveConn struct {
	sync.RWMutex
	conn     PacketConn
	read     chan Packet
	write    chan Packet
	ping     chan Packet
	pong     chan struct{}
	open     chan Packet
	cancel   context.CancelFunc
	isClosed bool
	once     *sync.Once
	openOnce *sync.Once
	closeErr error
}

// NewKeepAliveConn returns a new instance of KeepAliveConn
func NewKeepAliveConn(conn PacketConn, readBufferSize int, outgoing chan Packet) *KeepAliveConn {
	return &KeepAliveConn{
		conn:     conn,
		read:     make(chan Packet, readBufferSize),
		write:    outgoing,
		ping:     make(chan Packet),
		pong:     make(chan struct{}),
		open:     make(chan Packet),
		openOnce: &sync.Once{},
		once:     &sync.Once{},
	}
}

func (k *KeepAliveConn) State() PacketConnState {
	return k.conn.State()
}

// ID returns the ID of the wrapped Conn
func (k *KeepAliveConn) ID() string {
	return k.conn.ID()
}

// SupportsBinary returns whether the wrapped Conn supports binary
func (k *KeepAliveConn) SupportsBinary() bool {
	return k.conn.SupportsBinary()
}

func (k *KeepAliveConn) Write() chan<- Packet {
	return k.write
}

func (k *KeepAliveConn) Read() <-chan Packet {
	return k.read
}

// Close stops the Ping/Pong process and closes the wrapped Conn
func (k *KeepAliveConn) Close() error {
	k.once.Do(func() {
		k.Lock()
		defer k.Unlock()

		if k.cancel != nil {
			k.cancel()
		}

		close(k.read)
		k.closeErr = k.conn.Close()
		k.cancel = nil
		k.isClosed = true
	})

	return k.closeErr
}

// KeepAliveContext starts the Ping/Pong process
func (k *KeepAliveConn) KeepAliveContext(ctx context.Context) {
	k.Lock()
	defer k.Unlock()

	if k.cancel != nil || k.isClosed {
		return
	}

	derivedCtx, cancel := context.WithCancel(ctx)
	k.cancel = cancel

	go k.readContext(derivedCtx)
	go k.writeContext(derivedCtx)
	go k.keepAliveContext(derivedCtx)

}

func (k *KeepAliveConn) keepAliveContext(ctx context.Context) {
	var openPacket Packet
	select {
	case <-ctx.Done():
		return
	case openPacket = <-k.open:
	}

	if openPacket.GetData() == nil {
		k.Close()
		return
	}

	data := openData{}
	err := json.Unmarshal(openPacket.GetData(), &data)

	if err != nil {
		k.Close()
		return
	}

	// TODO: Ensure sane values?
	keepAliveInterval := time.Duration(data.PingInterval) * time.Millisecond
	keepAliveTimeout := time.Duration(data.PingTimeout) * time.Millisecond

	// Send an initial ping
	select {
	case <-ctx.Done():
		return
	case k.ping <- &StringPacket{Type: Ping}:
		go k.checkKeepAliveContext(ctx, keepAliveTimeout)
	}

	ticker := time.NewTicker(keepAliveInterval)

	for {
		select {
		case <-ctx.Done():
			close(k.ping)
			return
		case <-ticker.C:
			select {
			case <-ctx.Done():
				close(k.ping)
				return
			case k.ping <- &StringPacket{Type: Ping}:
				go k.checkKeepAliveContext(ctx, keepAliveTimeout)
			}
		}
	}
}

func (k *KeepAliveConn) checkKeepAliveContext(ctx context.Context, timeout time.Duration) {
	t := time.NewTimer(timeout)

	select {
	case <-ctx.Done():
		stopTimer(t)
	case <-k.pong:
		stopTimer(t)
	case <-t.C:
		k.Close()
	}
}

func (k *KeepAliveConn) readContext(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			packet, err := k.conn.Read()

			if err != nil {
				k.Close()
				return
			}

			if packet == nil {
				return
			}

			switch packet.GetType() {
			case Open:
				k.open <- packet
				k.openOnce.Do(func() { k.read <- packet })
			case Pong:
				k.pong <- struct{}{}
			default:
				k.read <- packet
			}

		}
	}
}

func (k *KeepAliveConn) writeTo(packet Packet) error {
	return k.conn.Write(packet)
}

func (k *KeepAliveConn) writeContext(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case packet, ok := <-k.ping:
			if !ok {
				k.Close()
				return
			}

			err := k.writeTo(packet)

			if err != nil {
				k.Close()
				return
			}

		case packet, ok := <-k.write:
			if !ok {
				k.Close()
				return
			}

			err := k.writeTo(packet)

			if err != nil {
				k.Close()
				return
			}
		}
	}
}

func stopTimer(timer *time.Timer) {
	timer.Stop()

	select {
	case <-timer.C:
	default:
	}
}
