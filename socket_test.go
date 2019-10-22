package gocketio

import (
	"context"
	"log"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/queue-b/gocketio/engine"
	"github.com/queue-b/gocketio/socket"
)

func TestSocketNamespace(t *testing.T) {
	t.Parallel()
	s := Socket{namespace: "/test"}

	if s.Namespace() != "/test" {
		t.Errorf("Namespace invalid. Expected /test, got %v", s.Namespace())
	}
}

func TestSocketID(t *testing.T) {
	t.Parallel()
	s := Socket{id: "ididid"}

	if s.ID() != "ididid" {
		t.Fatalf("ID invalid. Expected 'ididid', got %v\n", s.ID())
	}
}

func TestSocketOn(t *testing.T) {
	t.Run("WithFunctionHandler", func(t *testing.T) {
		t.Parallel()
		s := newSocket("/", "id", make(chan socket.Packet, 1))
		err := s.On("fancy", func(s string) {})

		if err != nil {
			t.Errorf("Unable to add event handler %v", err)
		}

		if _, ok := s.events.Load("fancy"); !ok {
			t.Error("On() did not add handler to handlers map")
		}
	})

	t.Run("WithNonFunctionHandler", func(t *testing.T) {
		t.Parallel()
		s := newSocket("/", "id", make(chan socket.Packet, 1))

		err := s.On("fancy", 5)

		if err == nil {
			t.Error("Adding non-function handler should not have succeeded")
		}

		if _, ok := s.events.Load("fancy"); ok {
			t.Error("On() should not add non-func handler to handlers map")
		}
	})
}

func TestSocketOff(t *testing.T) {
	t.Parallel()
	s := newSocket("/", "id", make(chan socket.Packet, 1))

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
	t.Run("WithData", func(t *testing.T) {
		t.Parallel()
		s := newSocket("/", "id", make(chan socket.Packet, 1))
		packets := make(chan socket.Packet, 1)

		s.incomingPackets = packets

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go s.readFromManager(ctx)

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
	})

	t.Run("WithNoData", func(t *testing.T) {
		t.Parallel()
		s := newSocket("/", "id", make(chan socket.Packet, 1))
		packets := make(chan socket.Packet, 1)

		s.incomingPackets = packets

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go s.readFromManager(ctx)

		results := make(chan struct{}, 1)

		s.On("fancy", func() {
			results <- struct{}{}
		})

		p := socket.Packet{
			Type: socket.Event,
			Data: []interface{}{"fancy"},
		}

		packets <- p

		timer := time.NewTimer(500 * time.Millisecond)

		select {
		case <-timer.C:
			t.Fatal("Handler was not invoked")
		case <-results:
		}
	})

	t.Run("WithoutEventHandler", func(t *testing.T) {
		t.Parallel()
		s := newSocket("/", "id", make(chan socket.Packet, 1))
		packets := make(chan socket.Packet, 1)

		s.incomingPackets = packets

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go s.readFromManager(ctx)

		results := make(chan string, 1)

		s.On("fancy", func(s string) {
			results <- s
		})

		p := socket.Packet{
			Type: socket.Event,
			Data: []interface{}{"plain", "pants"},
		}

		packets <- p

		timer := time.NewTimer(500 * time.Millisecond)

		select {
		case <-results:
			t.Fatal("Event should not have been raised")
		case <-timer.C:
		}
	})

	t.Run("WithoutAckHandler", func(t *testing.T) {
		t.Parallel()
		s := newSocket("/", "id", make(chan socket.Packet, 1))
		packets := make(chan socket.Packet, 1)

		s.incomingPackets = packets

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go s.readFromManager(ctx)

		results := make(chan int64, 1)

		s.acks.Store(10, reflect.ValueOf(func(id int64, data interface{}) {
			results <- id
		}))

		ackID := int64(7)

		p := socket.Packet{
			Type: socket.Ack,
			ID:   &ackID,
			Data: []interface{}{"acky", "tack"},
		}

		packets <- p

		timer := time.NewTimer(500 * time.Millisecond)

		select {
		case <-results:
			t.Fatal("Event should not have been raised")
		case <-timer.C:
		}
	})

	t.Run("WithAck", func(t *testing.T) {
		t.Parallel()
		s := newSocket("/", "id", make(chan socket.Packet, 1))
		packets := make(chan socket.Packet, 1)

		s.incomingPackets = packets

		ackIds := make(chan int64, 1)

		s.EmitWithAck("fancier", func(id int64, data interface{}) { ackIds <- id })

		expectedID := int64(1)

		ackPacket := socket.Packet{
			Type:      socket.Ack,
			ID:        &expectedID,
			Namespace: "/",
			Data:      nil,
		}

		s.incomingPackets <- ackPacket

		ctx, cancel := context.WithCancel(context.Background())

		defer cancel()

		go s.readFromManager(ctx)

		firstAckID := <-ackIds

		if firstAckID != expectedID {
			t.Fatalf("Expected Ack ID %v, got %v\n", expectedID, firstAckID)
		}
	})

	t.Run("WithAckRequest", func(t *testing.T) {
		t.Parallel()
		s := newSocket("/", "id", make(chan socket.Packet, 1))
		packets := make(chan socket.Packet, 1)

		s.incomingPackets = packets

		s.On("fancyAckable", func() {})

		id := int64(15)

		packetForAck := socket.Packet{
			Type:      socket.Event,
			ID:        &id,
			Namespace: "/",
			Data:      []interface{}{"fancyAckable", "pantses"},
		}

		ctx, cancel := context.WithCancel(context.Background())

		defer cancel()

		go s.readFromManager(ctx)

		s.incomingPackets <- packetForAck

		ackPacket := <-s.outgoingPackets

		if ackPacket.Type != socket.Ack {
			t.Fatalf("Expected ACK packet, got %v\n", ackPacket.Type)
		}

		if *ackPacket.ID != id {
			t.Fatalf("Expected ACK packet ID %v, got %v\n", id, *ackPacket.ID)
		}
	})
}

func TestSocketEmit(t *testing.T) {
	t.Run("WithoutAck", func(t *testing.T) {
		t.Parallel()
		s := newSocket("/", "id", make(chan socket.Packet, 1))
		packets := make(chan socket.Packet, 1)

		s.incomingPackets = packets

		s.Emit("fancy", "pants")

		p := <-s.outgoingPackets

		if p.Type != socket.Event {
			t.Errorf("Expected Event, got %v", p.Type)
		}

		if p.Namespace != "/" {
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
	})

	t.Run("WithAck", func(t *testing.T) {
		t.Parallel()
		s := newSocket("/", "id", make(chan socket.Packet, 1))
		packets := make(chan socket.Packet)

		s.incomingPackets = packets

		err := s.EmitWithAck("fancy", func(id int64, data interface{}) {}, "pants")

		if err != nil {
			t.Fatalf("Unexpected error - EmitWithAck: %v\n", err)
		}

		p := <-s.outgoingPackets

		if p.Type != socket.Event {
			t.Errorf("Expected Event, got %v", p.Type)
		}

		if p.Namespace != "/" {
			t.Errorf("Expected no namespace, got %v", p.Namespace)
		}

		if *p.ID != 1 {
			t.Errorf("Expected 0, got %v", *p.ID)
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
	})
}

func TestSocketSend(t *testing.T) {
	t.Parallel()
	s := newSocket("/", "id", make(chan socket.Packet, 1))
	packets := make(chan socket.Packet, 1)

	s.incomingPackets = packets

	s.Send("pants")

	p := <-s.outgoingPackets

	if p.Type != socket.Event {
		t.Errorf("Expected Event, got %v", p.Type)
	}

	if p.Namespace != "/" {
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

func TestEventBlacklist(t *testing.T) {
	t.Parallel()
	blacklistedEvents := []string{
		"connect",
		"connect_error",
		"connect_timeout",
		"connecting",
		"disconnect",
		"error",
		"reconnect",
		"reconnect_attempt",
		"reconnect_failed",
		"reconnect_error",
		"reconnecting",
		"ping",
		"pong",
	}

	for _, e := range blacklistedEvents {
		if !isBlacklisted(e) {
			t.Fatalf("%v should be blacklisted\n", e)
		}
	}

	s := newSocket("/", "id", make(chan socket.Packet, 1))
	packets := make(chan socket.Packet, 1)

	s.incomingPackets = packets

	err := s.Emit("connect", "hello")

	if err != ErrBlacklistedEvent {
		t.Fatal("Emit should not emit a blacklisted event")
	}

	err = s.EmitWithAck("connect", func(id int64, data interface{}) {}, "hello")

	if err != ErrBlacklistedEvent {
		t.Fatal("EmitWithAck should not emit a blacklisted event")
	}

}

// TODO: Additional tests for acking functions
func TestOnOpenOnDisconnect(t *testing.T) {
	t.Parallel()
	oldID := "oldID"
	newID := "newID"

	s := newSocket("/", oldID, make(chan socket.Packet, 1))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.onOpen(ctx, newID)

	if s.ID() != newID {
		t.Fatalf("Expected ID %v, got %v\n", newID, oldID)
	}

	if s.Connected() {
		t.Fatal("Expected Disconnected, got Connected")
	}

	s.onConnect()

	if !s.Connected() {
		t.Fatal("Expected Connected, got Disconnected")
	}

	s.onDisconnect(true)
	s.onDisconnect(true)

	if s.Connected() {
		t.Fatal("Expected Disconnected, got Connected")
	}
}

func TestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end tests")
	}

	mux := http.NewServeMux()

	srv, address := engine.CreateTestSocketIOServer(mux)

	doneSending := make(chan struct{})

	mux.HandleFunc("/socket.io/", engine.CreateTestSocketIOHandler(func(c *websocket.Conn) {
		var wg sync.WaitGroup

		var writeMutex sync.Mutex

		var normalOpenData = `{"sid":"abcd", "pingInterval": 10000, "pingTimeout": 5000}`

		packetsFromSocketIO := make(chan engine.Packet, 10000)

		ackIds := []int64{
			0, 1, 2,
		}

		eventPackets := []socket.Packet{
			socket.Packet{
				Namespace: "/",
				Type:      socket.Event,
				ID:        &ackIds[0],
				Data:      []interface{}{"someEvent", "data"},
			},
			socket.Packet{
				Namespace: "/",
				Type:      socket.Event,
				Data:      []interface{}{"otherEvent", "nonData"},
			},
			socket.Packet{
				Namespace: "/test",
				Type:      socket.Event,
				ID:        &ackIds[1],
				Data:      []interface{}{"otherEvent", "withAck"},
			},
			socket.Packet{
				Namespace: "/ssdtd",
				Type:      socket.Event,
				Data:      []interface{}{"someEvent", "helpfulData"},
			},
		}

		packetsForSocketIO := []engine.Packet{
			&engine.StringPacket{Type: engine.Open, Data: &normalOpenData},
		}

		for _, v := range eventPackets {
			data, err := v.Encode(true)

			if err != nil {
				t.Fatalf("Error encoding test data %v\n", err)
			}

			strData := string(data[0])

			packetsForSocketIO = append(packetsForSocketIO, &engine.StringPacket{Type: engine.Message, Data: &strData})
		}

		wg.Add(2)
		go func() {
			for {
				mt, message, err := c.ReadMessage()
				if err != nil {
					log.Println("read:", err)
					break
				}

				if mt == websocket.TextMessage {
					packet, err := engine.DecodeStringPacket(string(message))

					if err != nil {
						log.Println(err)
						continue
					}

					packetsFromSocketIO <- packet

					if packet.GetType() == engine.Ping {
						writeMutex.Lock()

						pong := engine.StringPacket{Type: engine.Pong}
						encoded, err := pong.Encode(true)

						if err != nil {
							log.Println(err)
							continue
						}

						c.WriteMessage(websocket.TextMessage, encoded)
						writeMutex.Unlock()
					}
				} else if mt == websocket.BinaryMessage {
					packet, err := engine.DecodeBinaryPacket(message)

					if err != nil {
						log.Println(err)
						continue
					}

					packetsFromSocketIO <- packet
				} else {
					wg.Done()
				}

				log.Printf("recv: %s", message)
			}

			wg.Done()
		}()

		go func() {
			time.Sleep(100 * time.Millisecond)
			for _, v := range packetsForSocketIO {
				switch v.(type) {
				case *engine.BinaryPacket:
					data, err := v.Encode(true)

					if err != nil {
						continue
					}

					c.WriteMessage(websocket.BinaryMessage, data)
				case *engine.StringPacket:
					data, err := v.Encode(true)

					if err != nil {
						continue
					}

					c.WriteMessage(websocket.TextMessage, data)
				}

				time.Sleep(100 * time.Millisecond)
			}

			doneSending <- struct{}{}

			wg.Done()
		}()

		wg.Wait()
	}))

	go srv.Start()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	m, err := DialContext(ctx, address, DefaultManagerConfig())

	if err != nil {
		t.Fatalf("Unable to dial %v\n", err)
	}

	rootSocket, err := m.Namespace("/")

	if err != nil {
		t.Fatalf("Unable to create root socket %v\n", err)
	}

	testSocket, err := m.Namespace("/test")

	if err != nil {
		t.Fatalf("Unable to create test socket %v\n", err)
	}

	rootSocketEventsReceived := make(chan string, 10000)
	testSocketEventsReceived := make(chan string, 10000)

	err = rootSocket.On("someEvent", func(test string) {
		rootSocketEventsReceived <- test
	})

	if err != nil {
		t.Fatalf("Unable to add event handler %v\n", err)
	}

	err = testSocket.On("otherEvent", func(test string) {
		testSocketEventsReceived <- test
	})

	if err != nil {
		t.Fatalf("Unable to add event handler %v\n", err)
	}

	<-doneSending

	time.Sleep(10 * time.Millisecond)

	if len(rootSocketEventsReceived) != 1 {
		t.Fatalf("Expected 1 event received by root namespace socket, got %v\n", len(rootSocketEventsReceived))
	}

	if len(testSocketEventsReceived) != 1 {
		t.Fatalf("Expected 1 event received by test namespace socket, got %v\n", len(testSocketEventsReceived))
	}
}
