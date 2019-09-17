package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Conn is a connection to an Engine.IO connection
type Conn struct {
	socket  *websocket.Conn
	id      string
	Errors  chan error
	Send    chan EnginePacket
	Receive chan EnginePacket
}

func (conn *Conn) startEnginePing(pingInterval int) {
	pingDuration := time.Duration(pingInterval) * time.Millisecond
	t := time.NewTicker(pingDuration)

	p := &StringPacket{Type: Ping}

	conn.Send <- p

	for {
		select {
		case <-t.C:
			p = &StringPacket{Type: Ping}
			conn.Send <- p
		}
	}
}

func (conn *Conn) receiveMessage() error {
	t, message, err := conn.socket.ReadMessage()
	var packet EnginePacket

	if err != nil {
		return err
	}

	switch t {
	case websocket.CloseMessage:
		fmt.Println("Received ws Close message")
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
			go conn.startEnginePing(data.PingInterval)
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
			err := conn.receiveMessage()

			if err != nil {
				fmt.Printf("Error receiving %v", err)
				conn.Errors <- err
			}
		}
	}
}

func (conn *Conn) sendMessage() error {
	message := <-conn.Send
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
			err := conn.sendMessage()

			if err != nil {
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

func Dial(address string) (*Conn, error) {
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

	socket, _, err := websocket.DefaultDialer.Dial(parsedAddress.String(), nil)

	socket.SetCloseHandler(func(code int, message string) error {
		fmt.Println("Closing", message)
		return nil
	})

	if err != nil {
		return nil, err
	}

	errs := make(chan error, 10000)
	sends := make(chan EnginePacket, 10000)
	receives := make(chan EnginePacket, 10000)

	conn := &Conn{
		socket:  socket,
		Errors:  errs,
		Send:    sends,
		Receive: receives,
	}

	return conn, nil
}
