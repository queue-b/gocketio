package engine

import (
	"context"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/queue-b/gocketio/engine/transport"
)

func TestID(t *testing.T) {
	t.Parallel()
	c := RawConn{}

	id := "asdfjkl"
	c.setID(id)

	if c.ID() != id {
		t.Fatalf("Expected ID %v, got %v", id, c.ID())
	}
}

func TestFixupAddress(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	mux := http.NewServeMux()

	srv, address := CreateTestSocketIOServer(mux)

	mux.HandleFunc("/socket.io/", CreateTestSocketIOHandler(func(c *websocket.Conn) {
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
	t.Parallel()
	mux := http.NewServeMux()

	srv, address := CreateTestSocketIOServer(mux)

	mux.HandleFunc("/socket.io/", CreateTestSocketIOHandler(func(c *websocket.Conn) {
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

	packet := transport.StringPacket{
		Type: transport.Message,
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
	t.Parallel()
	mux := http.NewServeMux()

	srv, address := CreateTestSocketIOServer(mux)

	mux.HandleFunc("/socket.io/", CreateTestSocketIOHandler(func(c *websocket.Conn) {
		err := c.WriteMessage(websocket.TextMessage, QuickEncode(packetSequenceNormal[0]))
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

	if p.GetType() != transport.Message {
		t.Fatalf("Unexpected message type")
	}

	if string(p.GetData()) != "hello 亜" {
		t.Fatalf("Expected data %v, got %v\n", "hello 亜", string(p.GetData()))
	}
}

func TestConnectionReadCloseMessage(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()

	srv, address := CreateTestSocketIOServer(mux)

	mux.HandleFunc("/socket.io/", CreateTestSocketIOHandler(func(c *websocket.Conn) {
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
	t.Parallel()
	mux := http.NewServeMux()

	srv, address := CreateTestSocketIOServer(mux)

	mux.HandleFunc("/socket.io/", CreateTestSocketIOHandler(func(c *websocket.Conn) {
		err := c.WriteMessage(websocket.TextMessage, QuickEncode(packetSequenceOpen[0]))
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
