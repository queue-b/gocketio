package engine

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/websocket"
)

var normalOpenData = `{"sid":"abcd", "pingInterval": 1000, "pingTimeout": 250}`
var longOpenData = `{"sid":"abcd", "pingInterval": 10000, "pingTimeout": 5000}`

var stringData = "hello 亜"

var packetSequenceNormal = []Packet{
	&StringPacket{Type: Message, Data: &stringData},
}

var packetSequenceOpen = []Packet{
	&StringPacket{Type: Open, Data: &normalOpenData},
	&StringPacket{Type: Pong},
}

var packetSequenceTimeout = []Packet{
	&StringPacket{Type: Open, Data: &longOpenData},
}

func CreateTestSocketIOServer(mux *http.ServeMux) (*httptest.Server, string) {
	server := httptest.NewUnstartedServer(mux)

	return server, fmt.Sprintf("http://%v/socket.io/", server.Listener.Addr())
}

func CreateTestSocketIOHandler(handler func(c *websocket.Conn)) http.HandlerFunc {
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

func CreateOpenPingPongHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var upgrader = websocket.Upgrader{}

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		defer c.Close()

		err = c.WriteMessage(websocket.TextMessage, QuickEncode(packetSequenceTimeout[0]))
		if err != nil {
			log.Println("write:", err)
		}

		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Fatalf("Unable to read %v\n", err)
		}

		if mt != websocket.TextMessage {
			log.Fatal("Expected text message")
		}

		msgStr := string(message)

		if msgStr != fmt.Sprintf("%v", Ping) {
			log.Fatalf("Expected ping")
		}

		err = c.WriteMessage(websocket.TextMessage, QuickEncode(packetSequenceOpen[0]))
		if err != nil {
			log.Println("write:", err)
		}

		forever := make(chan struct{})
		<-forever
	}
}

func QuickEncode(p Packet) []byte {
	bytes, _ := p.Encode(true)
	return bytes
}
