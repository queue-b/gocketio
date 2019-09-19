package gocket

import (
	"context"
	"reflect"
	"testing"

	"github.com/queue-b/gocket/engine"

	"github.com/queue-b/gocket/socket"
)

func TestSendToEngine(t *testing.T) {
	s := &Socket{}
	s.events = make(map[string]reflect.Value)
	s.outgoingPackets = make(chan socket.Packet)

	m := &Manager{}
	m.sockets = make(map[string]*Socket)

	enginePackets := make(chan engine.Packet)

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	go sendToEngine(ctx, m, s.outgoingPackets, enginePackets)

	s.Emit("fancy", "pants")

	p := <-enginePackets

	// TODO: Additional tests to make sure that the packet was encoded correctly
	if p.GetType() != engine.Message {
		t.Errorf("Expected Message, got %v", p.GetType())
	}
}

func TestReceiveFromEngine(t *testing.T) {
	s := &Socket{}
	s.events = make(map[string]reflect.Value)
	s.incomingPackets = make(chan socket.Packet)

	m := &Manager{}
	m.sockets = make(map[string]*Socket)
	m.sockets["/"] = s

	enginePackets := make(chan engine.Packet)

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	go receiveFromEngine(ctx, m, enginePackets)

	p := socket.Packet{}
	p.Type = socket.Event
	p.Namespace = "/"
	p.Data = []interface{}{"fancy", "pants"}

	encoded, err := p.Encode()

	if err != nil {
		t.Errorf("Error encoding data %v\n", err)
	}

	encodedData := string(encoded[0])

	ep := engine.StringPacket{
		Type: engine.Message,
		Data: &encodedData,
	}

	enginePackets <- &ep

	rp := <-s.incomingPackets

	if rp.Type != socket.Event {
		t.Errorf("Expected Event, got %v", rp.Type)
	}
}

func TestManagerNamespace(t *testing.T) {

}
