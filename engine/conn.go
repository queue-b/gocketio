package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Dialer func(address string) (*Conn, error)

func ContextDialer(ctx context.Context, address string) Dialer {
	return func(address string) (*Conn, error) {
		return DialContext(ctx, address)
	}
}

// Conn is a connection to an Engine.IO connection
type Conn struct {
	socket     *websocket.Conn
	id         string
	disconnect chan struct{}
	Errors     chan error
	Send       chan Packet
	Receive    chan Packet
	cancel     context.CancelFunc
}

// Disconnected returns a channel that is closed when the Conn disconnects
func (conn *Conn) Disconnected() <-chan struct{} {
	return conn.disconnect
}

// SupportsBinary returns true if the underlying connection supports sending raw binary data (bytes),
// false otherwise
func (conn *Conn) SupportsBinary() bool { return true }

func (conn *Conn) startEnginePing(ctx context.Context, pingInterval time.Duration) {
	t := time.NewTicker(pingInterval)

	p := &StringPacket{Type: Ping}

	conn.Send <- p

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p = &StringPacket{Type: Ping}
			conn.Send <- p
		}
	}
}

func (conn *Conn) receiveFromTransport(ctx context.Context) error {
	t, message, err := conn.socket.ReadMessage()
	var packet Packet

	if err != nil {
		return err
	}

	switch t {
	case websocket.CloseMessage:
		return &websocket.CloseError{}
	case websocket.PongMessage:
		fmt.Println("Received ws Pong message")
	case websocket.PingMessage:
		fmt.Println("Received ws Ping message")
	case websocket.BinaryMessage:
		fmt.Println("Received ws BinaryMessage")
		packet, err = DecodeBinaryPacket(message)

		if err != nil {
			return err
		}
	// Ping, Pong, Close, and Error messages all optionally have string data attached
	case websocket.TextMessage:
		packet, err = DecodeStringPacket(string(message))

		if err != nil {
			return err
		}
	}

	switch packet.GetType() {
	case Open:
		data := openData{}

		if packet.GetData() != nil {
			err = json.Unmarshal(packet.GetData(), &data)

			if err != nil {
				return err
			}

			conn.id = data.SID
			go conn.startEnginePing(ctx, time.Duration(data.PingInterval)*time.Millisecond)
		}
	case Message:
		conn.Receive <- packet
	case Ping:
		fmt.Println("Received Ping")
	case Pong:
		fmt.Println("Received Pong")
	case Close:
		fmt.Println("Received Close")
	case NoOp:
		fmt.Println("Received NoOp")
	case Upgrade:
		fmt.Println("Received Upgrade")
	}

	return nil
}

func (conn *Conn) receiveMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := conn.receiveFromTransport(ctx)

			if err != nil {
				conn.Errors <- err

				if _, ok := err.(*websocket.CloseError); ok {
					conn.cancel()
					close(conn.Receive)
					close(conn.disconnect)
					return
				}
			}
		}
	}
}

func (conn *Conn) sendToTransport(message Packet) error {
	if message == nil {
		return errors.New("Cannot send nil message")
	}

	data, err := message.Encode(true)

	if err != nil {
		return err
	}

	switch message.(type) {
	case *StringPacket:
		err = conn.socket.WriteMessage(websocket.TextMessage, data)
	case *BinaryPacket:
		err = conn.socket.WriteMessage(websocket.BinaryMessage, data)
	}

	return err
}

func (conn *Conn) sendMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			message, ok := <-conn.Send

			if !ok {
				return
			}

			err := conn.sendToTransport(message)

			if err != nil {
				// TODO: Check for special websocket error
				fmt.Printf("Error sending %v", err)
				conn.Errors <- err
			}
		}
	}
}

type openData struct {
	SID          string `json:"sid"`
	PingInterval int
	PingTimeout  int
}

// DialContext creates a Conn to the Engine.IO server located at address
func DialContext(ctx context.Context, address string) (*Conn, error) {
	parsedAddress, err := url.Parse(address)

	if err != nil {
		return nil, err
	}

	if parsedAddress.Path == "/" || parsedAddress.Path == "" {
		newPath := strings.TrimRight(parsedAddress.Path, "/")
		newPath += "/engine.io/"
		parsedAddress.Path = newPath
	}

	eio := fmt.Sprintf("%v", ParserProtocol)

	query := parsedAddress.Query()
	query.Set("EIO", eio)
	query.Set("transport", "websocket")
	parsedAddress.RawQuery = query.Encode()

	fmt.Println("Dialing", parsedAddress.String())

	dialer := websocket.DefaultDialer
	dialer.NetDial = func(network, address string) (net.Conn, error) {
		return net.DialTimeout(network, address, 200*time.Millisecond)
	}

	socket, _, err := websocket.DefaultDialer.DialContext(ctx, parsedAddress.String(), nil)

	if err != nil {
		return nil, err
	}

	errs := make(chan error, 10000)
	sends := make(chan Packet, 10000)
	receives := make(chan Packet, 10000)
	disconnects := make(chan struct{}, 10000)
	runCtx, cancel := context.WithCancel(ctx)

	conn := &Conn{
		socket:     socket,
		Errors:     errs,
		Send:       sends,
		Receive:    receives,
		disconnect: disconnects,
		cancel:     cancel,
	}

	go conn.receiveMessages(runCtx)
	go conn.sendMessages(runCtx)

	return conn, nil
}
