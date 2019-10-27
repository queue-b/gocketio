package transport

import "github.com/gorilla/websocket"

type WebSocket struct {
	socket *websocket.Conn
}
