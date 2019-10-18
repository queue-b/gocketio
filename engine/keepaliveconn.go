package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// KeepAliveConn wraps a Conn and handles the Ping/Pong process
type KeepAliveConn struct {
	sync.RWMutex
	conn   *Conn
	read   chan Packet
	write  <-chan Packet
	ping   chan Packet
	pong   chan struct{}
	open   chan Packet
	cancel context.CancelFunc
}

// NewKeepAliveConn returns a new instance of KeepAliveConn
func NewKeepAliveConn(conn *Conn, readBufferSize int, outgoing <-chan Packet) *KeepAliveConn {
	return &KeepAliveConn{
		conn:  conn,
		read:  make(chan Packet, readBufferSize),
		write: outgoing,
		ping:  make(chan Packet),
		pong:  make(chan struct{}),
		open:  make(chan Packet),
	}
}

// ID returns the ID of the wrapped Conn
func (k *KeepAliveConn) ID() string {
	k.RLock()
	defer k.RUnlock()

	if k.conn == nil {
		return ""
	}

	return k.conn.ID()
}

// SupportsBinary returns whether the wrapped Conn supports binary
func (k *KeepAliveConn) SupportsBinary() bool {
	k.RLock()
	defer k.RUnlock()

	if k.conn == nil {
		return false
	}

	return k.conn.SupportsBinary()
}

func (k *KeepAliveConn) Write() <-chan Packet {
	return k.write
}

func (k *KeepAliveConn) Read() chan<- Packet {
	return k.read
}

// Close stops the Ping/Pong process and closes the wrapped Conn
func (k *KeepAliveConn) Close() error {
	fmt.Println("Close")
	k.Lock()
	defer k.Unlock()

	if k.conn == nil {
		return nil
	}

	if k.cancel == nil {
		return nil
	}

	k.cancel()
	err := k.conn.Close()
	k.conn = nil

	return err
}

// KeepAliveContext starts the Ping/Pong process
func (k *KeepAliveConn) KeepAliveContext(ctx context.Context) {
	k.Lock()
	defer k.Unlock()

	if k.cancel != nil {
		return
	}

	derivedCtx, cancel := context.WithCancel(ctx)

	go k.readContext(derivedCtx)
	go k.writeContext(derivedCtx)
	go k.keepAliveContext(derivedCtx)

	k.cancel = cancel
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
			return
		case <-ticker.C:
			select {
			case <-ctx.Done():
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
			close(k.read)
			return
		default:
			k.RLock()
			conn := k.conn
			k.RUnlock()

			if conn == nil {
				k.Close()
				return
			}

			packet, err := conn.Read()

			if err != nil {
				k.Close()
				return
			}

			switch packet.GetType() {
			case Open:
				k.open <- packet
			case Pong:
				k.pong <- struct{}{}
			}

			k.read <- packet
		}
	}
}

func (k *KeepAliveConn) writeTo(packet Packet) error {
	k.RLock()
	conn := k.conn
	k.RUnlock()

	if conn == nil {
		return errors.New("No connection")
	}

	return conn.Write(packet)
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
