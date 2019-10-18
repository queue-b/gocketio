package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ErrDisconnected is returned when the transport is closed
var ErrDisconnected = errors.New("Transport disconnected")

type openData struct {
	SID          string `json:"sid"`
	PingInterval int64
	PingTimeout  int64
}

type Transport interface {
	ID() string
	SupportsBinary() bool
}

type ConnConfig struct {
	ConnectTimeout        time.Duration
	OutgoingChannelLength int32
}

// Conn is a connection to an Engine.IO connection
type Conn struct {
	sync.RWMutex
	readMutex  sync.Mutex
	writeMutex sync.Mutex
	socket     *websocket.Conn
	id         string
}

// ID returns the remote ID assigned to this connection
func (conn *Conn) ID() string {
	conn.RLock()
	defer conn.RUnlock()
	return conn.id
}

// SupportsBinary returns true if the underlying connection supports sending raw binary data (bytes),
// false otherwise
func (conn *Conn) SupportsBinary() bool { return true }

func (conn *Conn) setID(id string) {
	conn.Lock()
	defer conn.Unlock()
	conn.id = id
}

func fixupAddress(address string) (*url.URL, error) {
	parsedAddress, err := url.Parse(address)

	if err != nil {
		return nil, err
	}

	if parsedAddress.Scheme == "http" {
		parsedAddress.Scheme = "ws"
	}

	if parsedAddress.Scheme == "https" {
		parsedAddress.Scheme = "wss"
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

	return parsedAddress, nil
}

func (conn *Conn) Write(packet Packet) error {
	if packet == nil {
		return errors.New("Cannot send nil message")
	}

	data, err := packet.Encode(conn.SupportsBinary())

	if err != nil {
		return err
	}

	conn.writeMutex.Lock()
	defer conn.writeMutex.Unlock()
	switch packet.(type) {
	case *StringPacket:
		err = conn.socket.WriteMessage(websocket.TextMessage, data)
	case *BinaryPacket:
		err = conn.socket.WriteMessage(websocket.BinaryMessage, data)
	}

	return err
}

func (conn *Conn) Read() (Packet, error) {
	conn.readMutex.Lock()
	t, message, err := conn.socket.ReadMessage()
	conn.readMutex.Unlock()
	var packet Packet

	if err != nil {
		switch err.(type) {
		case *websocket.CloseError:
			return nil, ErrDisconnected
		case *net.OpError:
			return nil, ErrDisconnected

		}

		return nil, err
	}

	switch t {
	case websocket.BinaryMessage:
		packet, err = DecodeBinaryPacket(message)

		if err != nil {
			return nil, err
		}
	// Ping, Pong, Close, and Error messages all optionally have string data attached
	case websocket.TextMessage:
		packet, err = DecodeStringPacket(string(message))

		if err != nil {
			return nil, err
		}
	}

	switch packet.GetType() {
	case Open:
		err = conn.onOpen(packet)

		if err != nil {
			return nil, err
		}

		return packet, nil
	case Message:
		return packet, nil
	case Pong:
		return packet, nil
	case Upgrade:
		return packet, nil
	case Close:
		return nil, ErrDisconnected
	case NoOp:
		return packet, nil
	}

	return nil, nil
}

func (conn *Conn) onOpen(packet Packet) error {
	if packet.GetData() == nil {
		return errors.New("Invalid open packet")
	}

	data := openData{}
	err := json.Unmarshal(packet.GetData(), &data)

	if err != nil {
		return err
	}

	conn.setID(data.SID)
	return nil
}

// Close closes the underlying transport
func (conn *Conn) Close() error {
	return conn.socket.Close()
}

// DialContext creates a Conn to the Engine.IO server located at address
func DialContext(ctx context.Context, address string) (*Conn, error) {
	parsedAddress, err := fixupAddress(address)

	if err != nil {
		return nil, err
	}

	socket, _, err := websocket.DefaultDialer.DialContext(ctx, parsedAddress.String(), nil)

	if err != nil {
		return nil, err
	}

	conn := &Conn{
		socket: socket,
	}

	return conn, nil
}
