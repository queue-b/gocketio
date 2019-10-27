package socket

import (
	"encoding/json"
	"log"
	"testing"

	"github.com/queue-b/gocketio/engine"
)

func TestReplacePlaceholdersWithByteSlicesSimpleNumber(t *testing.T) {
	test := 5

	bytes, err := json.Marshal(test)

	if err != nil {
		t.Errorf("Unable to encode json %v\n", err)
	}

	var decoded interface{}

	err = json.Unmarshal(bytes, &decoded)

	if err != nil {
		t.Errorf("Unable to decode json %v\n", err)
	}

	replaced, _ := replacePlaceholdersWithByteSlices(decoded, nil)

	if replaced == nil {
		t.Error("Invalid replacement. Expected non-nil, got nil")
	}

	switch replacement := replaced.(type) {
	case float64:
		if replacement != 5 {
			t.Errorf("Invalid replacement. Expected 5, got %v", replacement)
		}
	default:
		t.Errorf("Invalid replacement. Expected float got %T", replaced)
	}

}

func TestReplacePlaceholdersWithByteSlicesSimpleString(t *testing.T) {
	test := "hello"

	bytes, err := json.Marshal(test)

	if err != nil {
		t.Errorf("Unable to encode json %v\n", err)
	}

	var decoded interface{}

	err = json.Unmarshal(bytes, &decoded)

	if err != nil {
		t.Errorf("Unable to decode json %v\n", err)
	}

	replaced, _ := replacePlaceholdersWithByteSlices(decoded, nil)

	if replaced == nil {
		t.Error("Invalid replacement. Expected non-nil, got nil")
	}

	switch replacement := replaced.(type) {
	case string:
		if replacement != "hello" {
			t.Errorf("Invalid replacement. Expected 'hello', got %v", replacement)
		}
	default:
		t.Errorf("Invalid replacement. Expected string got %T", replaced)
	}
}

func TestReplacePlaceholdersWithByteSlicesSimpleSlice(t *testing.T) {
	test := []interface{}{5, "hello"}

	bytes, err := json.Marshal(test)

	if err != nil {
		t.Errorf("Unable to encode json %v\n", err)
	}

	var decoded interface{}

	err = json.Unmarshal(bytes, &decoded)

	if err != nil {
		t.Errorf("Unable to decode json %v\n", err)
	}

	replaced, _ := replacePlaceholdersWithByteSlices(decoded, nil)

	if replaced == nil {
		t.Error("Invalid replacement. Expected non-nil, got nil")
	}

	switch replacement := replaced.(type) {
	case []interface{}:
		if len(replacement) != 2 {
			t.Errorf("Invalid replacement. Expected slice with 2 elements, got %v elements", len(replacement))
		}

		switch first := replacement[0].(type) {
		case float64:
			if first != 5 {
				t.Errorf("Invalid replacement. Expected 5, got %v", first)
			}
		default:
			t.Errorf("Invalid replacement. Expected float got %T", first)
		}

		switch second := replacement[1].(type) {
		case string:
			if second != "hello" {
				t.Errorf("Invalid replacement. Expected 'hello', got %v", second)
			}
		default:
			t.Errorf("Invalid replacement. Expected string got %T", second)
		}
	default:
		t.Errorf("Invalid replacement. Expected string got %T", replaced)
	}
}

func TestReplacePlaceholdersWithByteSlicesSimplePlaceholder(t *testing.T) {
	test := []interface{}{BinaryPlaceholder{Placeholder: true, Number: 0}}

	bytes, err := json.Marshal(test)

	if err != nil {
		t.Errorf("Unable to encode json %v\n", err)
	}

	var decoded interface{}

	err = json.Unmarshal(bytes, &decoded)

	if err != nil {
		t.Errorf("Unable to decode json %v\n", err)
	}

	var attachments [][]byte

	attachments = append(attachments, []byte{0, 1, 2})

	replaced, _ := replacePlaceholdersWithByteSlices(decoded, attachments)

	if replaced == nil {
		t.Error("Invalid replacement. Expected non-nil, got nil")
	}

	switch replacement := replaced.(type) {
	case []interface{}:
		if len(replacement) != 1 {
			t.Errorf("Invalid replacement. Expected []interface{} with 1 element, got %v elements", len(replacement))
		}

		switch first := replacement[0].(type) {
		case []byte:
			if len(first) != 3 {
				t.Errorf("Invalid replacement at index 0. Expected []byte with 3 elements, got %v elements", len(first))
			}

			for i, v := range first {
				if byte(i) != v {
					t.Errorf("Invalid replacement at [0][%v]. Expected %v got %v", i, i, v)
				}
			}
		default:
			t.Errorf("Invalid replacement at index 0. Expected []byte got %T", replaced)
		}
	default:
		t.Errorf("Invalid replacement. Expected []interface{} got %T", replaced)
	}
}

func TestReplacePlaceholdersWithByteSlicesSimplePlaceholderAndOther(t *testing.T) {
	test := []interface{}{BinaryPlaceholder{Placeholder: true, Number: 0}}

	bytes, err := json.Marshal(test)

	if err != nil {
		t.Errorf("Unable to encode json %v\n", err)
	}

	var decoded interface{}

	err = json.Unmarshal(bytes, &decoded)

	if err != nil {
		t.Errorf("Unable to decode json %v\n", err)
	}

	var attachments [][]byte

	attachments = append(attachments, []byte{0, 1, 2})

	replaced, _ := replacePlaceholdersWithByteSlices(decoded, attachments)

	if replaced == nil {
		t.Error("Invalid replacement. Expected non-nil, got nil")
	}

	switch replacement := replaced.(type) {
	case []interface{}:
		if len(replacement) != 1 {
			t.Errorf("Invalid replacement. Expected []interface{} with 1 element, got %v elements", len(replacement))
		}

		switch first := replacement[0].(type) {
		case []byte:
			if len(first) != 3 {
				t.Errorf("Invalid replacement at index 0. Expected []byte with 3 elements, got %v elements", len(first))
			}

			for i, v := range first {
				if byte(i) != v {
					t.Errorf("Invalid replacement at [0][%v]. Expected %v got %v", i, i, v)
				}
			}
		default:
			t.Errorf("Invalid replacement at index 0. Expected []byte got %T", replaced)
		}
	default:
		t.Errorf("Invalid replacement. Expected []interface{} got %T", replaced)
	}
}

func TestDecodeBinaryPacketWithoutNamespaceWithoutAttachments(t *testing.T) {
	data := `523["a"]`

	m, err := decodeMessage(data)

	if err != nil {
		t.Errorf("Unable to decode message %v\n", err)
	}

	if m.Type != BinaryEvent {
		t.Errorf("Invalid decoded message. Expected type BinaryEvent, got %v", m.Type)
	}

	if m.AttachmentCount != 0 {
		t.Errorf("Invalid decoded message. Expected no attachments, got %v", m.AttachmentCount)
	}

	if *m.ID != 23 {
		t.Errorf("Invalid decoded message. Expected ID 23, got %v", m.ID)
	}

	if m.Namespace != "" {
		t.Errorf("Invalid decoded message. Expected no namespace, got %v", m.Namespace)
	}

	switch p := m.Data.(type) {
	case []interface{}:
		if len(p) != 1 {
			t.Errorf("Invalid decoded message. Expected data length 1, got %v", len(p))
		}
	}
}

func TestDecodeBinaryPacketWithNamespaceWithoutAttachments(t *testing.T) {
	data := `5/cool,23["a"]`

	m, err := decodeMessage(data)

	if err != nil {
		t.Errorf("Unable to decode message %v\n", err)
	}

	if m.AttachmentCount != 0 {
		t.Errorf("Invalid decoded message. Expected no attachments, got %v", m.AttachmentCount)
	}

	if *m.ID != 23 {
		t.Errorf("Invalid decoded message. Expected ID 23, got %v", m.ID)
	}

	if m.Namespace != "/cool" {
		t.Errorf("Invalid decoded message. Expected Namespace /cool, got %v", m.Namespace)
	}

	switch p := m.Data.(type) {
	case []interface{}:
		if len(p) != 1 {
			t.Errorf("Invalid decoded message. Expected data length 1, got %v", len(p))
		}
	}
}

func TestBinaryPacketWithInvalidEverything(t *testing.T) {
	data := `5{""`

	_, err := decodeMessage(data)

	if err == nil {
		t.Fatal("Expected error decoding invalid packet")
	}

	data = `3-	`

	if err == nil {
		t.Fatal("Expected error decoding invalid packet")
	}
}

func TestDecodeBinaryPacketWithAttachments(t *testing.T) {
	data := `51-/cool,23["a",{"_placeholder":true,"num":0}]`

	m, err := decodeMessage(data)

	if err != nil {
		t.Errorf("Unable to decode message %v\n", err)
	}

	if m.AttachmentCount != 1 {
		t.Errorf("Invalid decoded message. Expected AttachmentCount 1, got %v", m.AttachmentCount)
	}

	if *m.ID != 23 {
		t.Errorf("Invalid decoded message. Expected ID 23, got %v", m.ID)
	}

	if m.Namespace != "/cool" {
		t.Errorf("Invalid decoded message. Expected Namespace /cool, got %v", m.Namespace)
	}

	switch p := m.Data.(type) {
	case []interface{}:
		if len(p) != 2 {
			t.Errorf("Invalid decoded message. Expected data length 2, got %v", len(p))
		}
	}
}

func TestDecodeStringConnectWithNamespaceNoData(t *testing.T) {
	data := `0/flight,`

	p, err := decodeMessage(data)

	if err != nil {
		t.Fatal("Unable to decode valid message")
	}

	if p.Type != Connect {
		t.Fatalf("Invalid type, expected %v got %v", Connect, p.Type)
	}
}

func TestIsRootNamespace(t *testing.T) {
	isRoot := IsRootNamespace("/")

	if isRoot != true {
		t.Fatal("/ is root namespace")
	}

	isRoot = IsRootNamespace("")

	if isRoot != true {
		t.Fatal("Empty string is root")
	}
}

func TestReassembleBinaryPacket(t *testing.T) {
	type testBinary struct {
		SomeAmountOfBinaryData []byte
		ACounter               int
		MoreBinaryData         []byte
		AnotherCounter         uint8
	}

	item := testBinary{
		[]byte{1, 3, 5, 8, 2, 3, 254, 22, 1, 2},
		5583,
		[]byte{23, 00, 99, 77, 33, 21},
		38,
	}

	packet := Packet{
		Type:      BinaryEvent,
		Namespace: "/test",
		Data:      item,
	}

	encoded, err := packet.Encode(true)

	if err != nil {
		log.Fatal(err)
	}

	first := string(encoded[0])

	var enginePackets []engine.Packet

	enginePackets = append(enginePackets, &engine.StringPacket{
		Type: engine.Message,
		Data: &first,
	})

	enginePackets = append(enginePackets, &engine.BinaryPacket{
		Type: engine.Message,
		Data: encoded[1],
	})

	enginePackets = append(enginePackets, &engine.BinaryPacket{
		Type: engine.Message,
		Data: encoded[2],
	})

	decoder := BinaryDecoder{}
	decoder.Reset()

	_, err = decoder.Decode(enginePackets[0])

	if err != ErrWaitingForMorePackets {
		t.Fatalf("Expected ErrWaitingForMorePackets, got %v\n", err)
	}

	_, err = decoder.Decode(enginePackets[1])

	if err != ErrWaitingForMorePackets {
		t.Fatalf("Expected ErrWaitingForMorePackets, got %v\n", err)
	}

	_, err = decoder.Decode(enginePackets[2])

	if err != nil {
		t.Fatalf("Expected nil, got %v\n", err)
	}
}
