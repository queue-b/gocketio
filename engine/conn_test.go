package engine

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

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
	address := "http://localhost:60808/socket.io/"

	u, err := url.Parse(address)

	if err != nil {
		t.Fatalf("Unable to parse address %v -  %v\n", address, err)
	}

	var upgrader = websocket.Upgrader{}

	websocketHandler := func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}
		defer c.Close()
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
	}

	http.HandleFunc(u.Path, websocketHandler)
	go http.ListenAndServe("localhost:60808", nil)

	_, err = DialContext(context.Background(), address, 1*time.Second)

	if err != nil {
		t.Fatalf("Unable to connect %v\n", err)
	}
}
