package engine

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func createServer(mux *http.ServeMux) (*httptest.Server, string) {
	server := httptest.NewUnstartedServer(mux)

	return server, fmt.Sprintf("http://%v/socket.io/", server.Listener.Addr())
}

func createHandler(handler func(c *websocket.Conn)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var upgrader = websocket.Upgrader{}

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()

		handler(c)
	}
}

func quickEncode(p Packet) []byte {
	bytes, _ := p.Encode(true)
	return bytes
}

var normalOpenData = `{"sid":"abcd", "pingInterval": 1000, "pingTimeout": 250}`
var stringData = "hello 亜"

var packetSequenceNormal = []Packet{
	&StringPacket{Type: Message, Data: &stringData},
}

var packetSequenceOpen = []Packet{
	&StringPacket{Type: Open, Data: &normalOpenData},
	&StringPacket{Type: Pong},
}

func TestID(t *testing.T) {
	c := Conn{}

	id := "asdfjkl"
	c.setID(id)

	if c.ID() != id {
		t.Fatalf("Expected ID %v, got %v", id, c.ID())
	}
}

func TestFixupAddress(t *testing.T) {
	address := "http://localhost:3030"

	parsedAddress, err := fixupAddress(address)

	if err != nil {
		t.Fatalf("Unable to fixup %v %v\n", address, err)
	}

	if parsedAddress.String() != "ws://localhost:3030/engine.io/?EIO=3&transport=websocket" {
		t.Fatalf("Expected %v got %v", "ws://localhost:3030/engine.io/?EIO=3&transport=websocket", parsedAddress.String())
	}

	address = "https://localhost:3030/socket.io/"

	parsedAddress, err = fixupAddress(address)

	if err != nil {
		t.Fatalf("Unable to fixup %v %v\n", address, err)
	}

	if parsedAddress.String() != "wss://localhost:3030/socket.io/?EIO=3&transport=websocket" {
		t.Fatalf("Expected %v got %v", "wss://localhost:3030/socket.io/?EIO=3&transport=websocket", parsedAddress.String())
	}
}

func TestDialContext(t *testing.T) {
	mux := http.NewServeMux()

	srv, address := createServer(mux)

	mux.HandleFunc("/socket.io/", createHandler(func(c *websocket.Conn) {
		for {
			mt, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}
			log.Printf("recv: %s", message)
			err = c.WriteMessage(mt, message)
			if err != nil {
				log.Println("write:", err)
				break
			}
		}
	}))

	deadlineCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(250*time.Millisecond))

	go srv.Start()
	defer cancel()
	defer srv.Close()

	_, err := DialContext(deadlineCtx, address)

	if err != nil {
		t.Fatalf("Unable to connect %v\n", err)
	}
}

func TestConnectionWrite(t *testing.T) {
	mux := http.NewServeMux()

	srv, address := createServer(mux)

	mux.HandleFunc("/socket.io/", createHandler(func(c *websocket.Conn) {
		mt, message, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("Unable to read %v\n", err)
		}

		if mt != websocket.TextMessage {
			t.Fatal("Expected text message")
		}

		msgStr := string(message)

		if msgStr != "4the world" {
			t.Fatalf("Expected '4the world', got %v", msgStr)
		}

		forever := make(chan struct{})

		<-forever
	}))

	go srv.Start()
	defer srv.Close()

	conn, err := DialContext(context.Background(), address)

	if err != nil {
		t.Fatalf("Unable to connect %v\n", err)
	}

	data := "the world"

	packet := StringPacket{
		Type: Message,
		Data: &data,
	}

	err = conn.Write(nil)

	if err == nil {
		t.Fatal("Should not write nil packet")
	}

	err = conn.Write(&packet)

	if err != nil {
		t.Fatalf("Unable to write %v\n", err)
	}
}

func TestConnectionRead(t *testing.T) {
	mux := http.NewServeMux()

	srv, address := createServer(mux)

	mux.HandleFunc("/socket.io/", createHandler(func(c *websocket.Conn) {
		err := c.WriteMessage(websocket.TextMessage, quickEncode(packetSequenceNormal[0]))
		if err != nil {
			log.Println("write:", err)
		}

		forever := make(chan struct{})
		<-forever
	}))

	go srv.Start()
	defer srv.Close()

	conn, err := DialContext(context.Background(), address)

	if err != nil {
		t.Fatalf("Unable to connect %v\n", err)
	}

	p, err := conn.Read()

	if err != nil {
		t.Fatalf("Unable to read %v\n", err)
	}

	if p.GetType() != Message {
		t.Fatalf("Unexpected message type")
	}

	if string(p.GetData()) != "hello 亜" {
		t.Fatalf("Expected data %v, got %v\n", "hello 亜", string(p.GetData()))
	}
}

func TestConnectionReadCloseMessage(t *testing.T) {
	mux := http.NewServeMux()

	srv, address := createServer(mux)

	mux.HandleFunc("/socket.io/", createHandler(func(c *websocket.Conn) {
		for {
			err := c.WriteMessage(websocket.CloseMessage, nil)
			if err != nil {
				log.Println("write:", err)
				break
			}
		}
	}))

	go srv.Start()
	defer srv.Close()

	conn, err := DialContext(context.Background(), address)

	if err != nil {
		t.Fatalf("Unable to connect %v\n", err)
	}

	_, err = conn.Read()

	if err != ErrDisconnected {
		t.Fatalf("Expected error %v, got %v", ErrDisconnected, err)
	}
}

func TestConnectionReadOpenMessage(t *testing.T) {
	mux := http.NewServeMux()

	srv, address := createServer(mux)

	mux.HandleFunc("/socket.io/", createHandler(func(c *websocket.Conn) {
		err := c.WriteMessage(websocket.TextMessage, quickEncode(packetSequenceOpen[0]))
		if err != nil {
			log.Println("write:", err)
		}
	}))

	go srv.Start()
	defer srv.Close()

	conn, err := DialContext(context.Background(), address)

	if err != nil {
		t.Fatalf("Unable to connect %v\n", err)
	}

	_, err = conn.Read()

	if err != nil {
		t.Fatalf("Got error reading packet %v", err)
	}
}
