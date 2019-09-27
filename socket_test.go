package gocketio

import (
	"context"
	"sync"
	"testing"

	"github.com/queue-b/gocketio/socket"
)

func TestSocketNamespace(t *testing.T) {
	s := Socket{namespace: "/test"}

	if s.Namespace() != "/test" {
		t.Errorf("Namespace invalid. Expected /test, got %v", s.Namespace())
	}
}

func TestSocketOnWithFunctionHandler(t *testing.T) {
	s := Socket{}
	s.events = sync.Map{}
	err := s.On("fancy", func(s string) {})

	if err != nil {
		t.Errorf("Unable to add event handler %v", err)
	}

	if _, ok := s.events.Load("fancy"); !ok {
		t.Error("On() did not add handler to handlers map")
	}
}

func TestSocketOnWithNonFunctionHandler(t *testing.T) {
	s := Socket{}
	s.events = sync.Map{}
	s.currentState = Connected

	err := s.On("fancy", 5)

	if err == nil {
		t.Error("Adding non-function handler should not have succeeded")
	}

	if _, ok := s.events.Load("fancy"); ok {
		t.Error("On() should not add non-func handler to handlers map")
	}
}

func TestSocketOff(t *testing.T) {
	s := Socket{}
	s.events = sync.Map{}
	s.currentState = Connected

	err := s.On("fancy", func() {})

	if err != nil {
		t.Errorf("Unable to add event handler %v", err)
	}

	s.Off("fancy")

	if _, ok := s.events.Load("fancy"); ok {
		t.Error("Expected off to remove event handler")
	}
}

func TestReceiveFromManager(t *testing.T) {
	s := Socket{}
	s.events = sync.Map{}
	s.currentState = Connected

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	packets := make(chan socket.Packet)

	go receiveFromManager(ctx, &s, packets)

	results := make(chan string, 1)

	s.On("fancy", func(s string) {
		results <- s
	})

	p := socket.Packet{
		Type: socket.Event,
		Data: []interface{}{"fancy", "pants"},
	}

	packets <- p

	result := <-results

	if result != "pants" {
		t.Errorf("Expected pants, got %v", result)
	}
}

func TestSocketEmit(t *testing.T) {
	s := Socket{}
	s.outgoingPackets = make(chan socket.Packet, 1)
	s.currentState = Connected

	s.Emit("fancy", "pants")

	p := <-s.outgoingPackets

	if p.Type != socket.Event {
		t.Errorf("Expected Event, got %v", p.Type)
	}

	if p.Namespace != "" {
		t.Errorf("Expected no namespace, got %v", p.Namespace)
	}

	if p.ID != nil {
		t.Errorf("Expected no id, got %v", *p.ID)
	}

	switch data := p.Data.(type) {
	case []interface{}:
		if len(data) != 2 {
			t.Errorf("Expected .Data length 2, got %v", len(data))
		}

		switch first := data[0].(type) {
		case string:
			if first != "fancy" {
				t.Errorf("Expected data[0] to be 'fancy', got %v", data[0])
			}
		default:
			t.Errorf("Expected first data element to be string, got %T", first)
		}

		switch second := data[1].(type) {
		case string:
			if second != "pants" {
				t.Errorf("Expected data[1] to be 'pants', got %v", data[1])
			}
		default:
			t.Errorf("Expected second data element to be string, got %T", second)
		}
	}

}

func TestSocketSend(t *testing.T) {
	s := Socket{}
	s.outgoingPackets = make(chan socket.Packet, 1)
	s.currentState = Connected

	s.Send("pants")

	p := <-s.outgoingPackets

	if p.Type != socket.Event {
		t.Errorf("Expected Event, got %v", p.Type)
	}

	if p.Namespace != "" {
		t.Errorf("Expected no namespace, got %v", p.Namespace)
	}

	if p.ID != nil {
		t.Errorf("Expected no id, got %v", *p.ID)
	}

	switch data := p.Data.(type) {
	case []interface{}:
		if len(data) != 2 {
			t.Errorf("Expected .Data length 2, got %v", len(data))
		}

		switch first := data[0].(type) {
		case string:
			if first != "message" {
				t.Errorf("Expected data[0] to be 'message', got %v", data[0])
			}
		default:
			t.Errorf("Expected first data element to be string, got %T", first)
		}

		switch second := data[1].(type) {
		case string:
			if second != "pants" {
				t.Errorf("Expected data[1] to be 'pants', got %v", data[1])
			}
		default:
			t.Errorf("Expected second data element to be string, got %T", second)
		}
	}
}
