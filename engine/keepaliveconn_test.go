package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestKeepAlive(t *testing.T) {
	mux := http.NewServeMux()

	srv, address := createServer(mux)

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

		fmt.Println("Writing open")
		err = c.WriteMessage(websocket.TextMessage, p)
		if err != nil {
			log.Println("write:", err)
		}

		fmt.Println("Reading ping")

		mt, message, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("Unable to read %v\n", err)
		}

		if mt != websocket.TextMessage {
			t.Fatal("Expected text message")
		}

		msgStr := string(message)

		if msgStr != fmt.Sprintf("%v", Ping) {
			t.Fatalf("Expected ping")
		}

		fmt.Println("Writing pong")

		pong := &StringPacket{Type: Pong}

		p, _ = pong.Encode(true)

		err = c.WriteMessage(websocket.TextMessage, p)
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

	keepConn := NewKeepAliveConn(conn, 100, make(chan Packet))
	keepConn.KeepAliveContext(context.Background())

	time.Sleep(1 * time.Second)

	// Make sure the server isn't closed
	if keepConn.ID() == "" {
		log.Fatal("Unexpected empty ID")
	}
}

func TestKeepAliveAccessors(t *testing.T) {
	mux := http.NewServeMux()

	srv, address := createServer(mux)

	mux.HandleFunc("/socket.io/", createHandler(func(c *websocket.Conn) {
		openContent := openData{
			SID:          "15",
			PingInterval: 10000,
			PingTimeout:  2500,
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

	go srv.Start()
	defer srv.Close()

	conn, err := DialContext(context.Background(), address)

	if err != nil {
		t.Fatalf("Unable to connect %v\n", err)
	}

	keepConn := NewKeepAliveConn(conn, 100, make(chan Packet))
	keepConn.KeepAliveContext(context.Background())

	time.Sleep(1000 * time.Millisecond)

	id := keepConn.ID()

	if id == "" {
		t.Fatal("Empty ID")
	}

	supportsBinary := keepConn.SupportsBinary()

	if supportsBinary == false {
		t.Fatal("Supports binary")
	}

	keepConn.Close()

	id = keepConn.ID()

	if id != "" {
		t.Fatal("Invalid ID")
	}

	supportsBinary = keepConn.SupportsBinary()

	if supportsBinary == true {
		t.Fatal("Supports binary")
	}

	err = keepConn.Close()

	if err != nil {
		t.Fatal("Double close caused error")
	}
}

func TestKeepAliveTimeout(t *testing.T) {
	mux := http.NewServeMux()

	srv, address := createServer(mux)

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

		mt, message, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("Unable to read %v\n", err)
		}

		if mt != websocket.TextMessage {
			t.Fatal("Expected text message")
		}

		msgStr := string(message)

		if msgStr != fmt.Sprintf("%v", Ping) {
			t.Fatalf("Expected ping")
		}
	}))

	go srv.Start()
	defer srv.Close()

	conn, err := DialContext(context.Background(), address)

	if err != nil {
		t.Fatalf("Unable to connect %v\n", err)
	}

	keepConn := NewKeepAliveConn(conn, 100, make(chan Packet))
	keepConn.KeepAliveContext(context.Background())

	time.Sleep(2 * time.Second)
}
