package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var ports = map[int64]bool{}
var mutex = sync.Mutex{}

func createServer(mux *http.ServeMux) (*http.Server, string) {
	port := rand.Int63n(65535-1024) + 1024

	mutex.Lock()
	defer mutex.Unlock()
	for {
		if _, ok := ports[port]; !ok {
			ports[port] = true
			break
		}
	}

	srv := http.Server{Addr: fmt.Sprintf("localhost:%v", port)}
	srv.Handler = mux

	go srv.ListenAndServe()

	return &srv, fmt.Sprintf("http://localhost:%v/socket.io/", port)
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
	srv.Handler = mux

	var wg sync.WaitGroup
	wg.Add(1)

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

	go srv.ListenAndServe()
	defer cancel()
	defer srv.Shutdown(deadlineCtx)

	_, err := DialContext(deadlineCtx, address)

	if err != nil {
		t.Fatalf("Unable to connect %v\n", err)
	}
}

func TestConnectionWrite(t *testing.T) {
	mux := http.NewServeMux()

	srv, address := createServer(mux)
	srv.Handler = mux

	var wg sync.WaitGroup
	wg.Add(1)

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

		wg.Done()
	}))

	deadlineCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Second))

	go srv.ListenAndServe()
	defer cancel()
	defer srv.Shutdown(deadlineCtx)

	conn, err := DialContext(deadlineCtx, address)

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

	wg.Wait()
}

func TestConnectionRead(t *testing.T) {
	mux := http.NewServeMux()

	srv, address := createServer(mux)
	srv.Handler = mux

	mux.HandleFunc("/socket.io/", createHandler(func(c *websocket.Conn) {
		for {
			data := "hello 亜"

			packet := StringPacket{
				Type: Message,
				Data: &data,
			}

			message, err := packet.Encode(true)

			err = c.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Println("write:", err)
				break
			}
		}
	}))

	deadlineCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Second))

	go srv.ListenAndServe()
	defer cancel()
	defer srv.Shutdown(deadlineCtx)

	conn, err := DialContext(deadlineCtx, address)

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

func TestConnectionPingTimeout(t *testing.T) {
	mux := http.NewServeMux()

	srv, address := createServer(mux)
	srv.Handler = mux

	mux.HandleFunc("/socket.io/", createHandler(func(c *websocket.Conn) {
		openContent := openData{
			SID:          "15",
			PingTimeout:  500,
			PingInterval: 750,
		}

		m, err := json.Marshal(&openContent)
		ms := string(m)
		packet := StringPacket{
			Type: Open,
			Data: &ms,
		}

		p, _ := packet.Encode(true)

		err = c.WriteMessage(websocket.TextMessage, p)
		if err != nil {
			log.Println("write:", err)
		}

		forever := make(chan struct{})

		<-forever

	}))

	deadlineCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))

	go srv.ListenAndServe()
	defer cancel()
	defer srv.Shutdown(deadlineCtx)

	conn, err := DialContext(deadlineCtx, address)

	if err != nil {
		t.Fatalf("Unable to connect %v\n", err)
	}

	p, err := conn.Read()

	if err != nil {
		t.Fatalf("Unable to read %v\n", err)
	}

	if p.GetType() != Open {
		t.Fatalf("Unexpected message type")
	}

	time.Sleep(2 * time.Second)

	_, err = conn.Read()

	if err != ErrDisconnected {
		t.Fatalf("Read from disconnected Conn should return ErrDisconnected, got %T", err)
	}
}

func TestConnectionReadCloseMessage(t *testing.T) {
	mux := http.NewServeMux()

	srv, address := createServer(mux)
	srv.Handler = mux

	mux.HandleFunc("/socket.io/", createHandler(func(c *websocket.Conn) {
		for {
			err := c.WriteMessage(websocket.CloseMessage, nil)
			if err != nil {
				log.Println("write:", err)
				break
			}
		}
	}))

	deadlineCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Second))

	go srv.ListenAndServe()
	defer cancel()
	defer srv.Shutdown(deadlineCtx)

	conn, err := DialContext(deadlineCtx, address)

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
	srv.Handler = mux

	mux.HandleFunc("/socket.io/", createHandler(func(c *websocket.Conn) {
		openContent := openData{
			SID:          "15",
			PingInterval: 1000,
			PingTimeout:  250,
		}

		m, err := json.Marshal(&openContent)
		ms := string(m)
		packet := StringPacket{
			Type: Open,
			Data: &ms,
		}

		p, _ := packet.Encode(true)

		err = c.WriteMessage(websocket.TextMessage, p)
		if err != nil {
			log.Println("write:", err)
		}
	}))

	deadlineCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Second))

	go srv.ListenAndServe()
	defer cancel()
	defer srv.Shutdown(deadlineCtx)

	conn, err := DialContext(deadlineCtx, address)

	if err != nil {
		t.Fatalf("Unable to connect %v\n", err)
	}

	_, err = conn.Read()

	if err != nil {
		t.Fatalf("Got error reading packet %v", err)
	}
}
