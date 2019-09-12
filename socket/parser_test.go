package socket

import "testing"

func TestEncodeConnectWithID(t *testing.T) {
	id := 1
	m := Message{
		ID:   &id,
		Type: Connect,
		Data: []string{"test"},
	}

	expected := `01["test"]`

	encoded, err := m.Encode()

	if err != nil {
		t.Errorf("Unable to encode %v", err)
	}

	if string(encoded[0]) != expected {
		t.Errorf("Encoded message invalid. Expected %v got %v", expected, string(encoded[0]))
	}
}

func TestEncodeDisconnect(t *testing.T) {
	m := Message{
		Type:      Disconnect,
		Namespace: "/woot",
	}

	expected := `1/woot,`

	encoded, err := m.Encode()

	if err != nil {
		t.Errorf("Unable to encode %v", err)
	}

	if string(encoded[0]) != expected {
		t.Errorf("Encoded message invalid. Expected %v got %v", expected, string(encoded[0]))
	}
}

func TestEncodeEventWithNamespaceAndID(t *testing.T) {
	id := 1
	m := Message{
		Type:      Event,
		Namespace: "/test",
		ID:        &id,
		Data:      []interface{}{"a", 1, struct{}{}},
	}

	expected := `2/test,1["a",1,{}]`

	encoded, err := m.Encode()

	if err != nil {
		t.Errorf("Unable to encode %v", err)
	}

	if string(encoded[0]) != expected {
		t.Errorf("Encoded message invalid. Expected %v got %v", expected, string(encoded[0]))
	}
}

func TestEncodeSimpleBinary(t *testing.T) {
	id := 23

	/* 51-/cool,23["a",{"_placeholder":true,"num":0}]
	<Buffer 61 62 63> */

	expected := `51-/cool,23["a",{"_placeholder":true,"num":0}]`

	m := Message{
		Type:      BinaryEvent,
		Namespace: "/cool",
		ID:        &id,
		Data:      []interface{}{"a", []byte("abc")},
	}

	encoded, err := m.Encode()

	if err != nil {
		t.Errorf("Unable to encode packet %v", err)
	}

	if len(encoded) != 2 {
		t.Errorf("Encoded message invalid. Expected 2 items, got %v", len(encoded))
	}

	if string(encoded[0]) != expected {
		t.Errorf("Encoded message invalid. Expected %v got %v", expected, string(encoded[0]))
	}

}

func TestEncodeBinaryFixedLengthByteArray(t *testing.T) {
	id := 23

	/* 51-/cool,23["a",{"_placeholder":true,"num":0}]
	<Buffer 61 62 63> */

	expected := `51-/cool,23["a",{"_placeholder":true,"num":0}]`

	m := Message{
		Type:      BinaryEvent,
		Namespace: "/cool",
		ID:        &id,
		Data:      []interface{}{"a", [1]byte{10}},
	}

	encoded, err := m.Encode()

	if err != nil {
		t.Errorf("Unable to encode packet %v", err)
	}

	if len(encoded) != 2 {
		t.Errorf("Encoded message invalid. Expected 2 items, got %v", len(encoded))
	}

	if string(encoded[0]) != expected {
		t.Errorf("Encoded message invalid. Expected %v got %v", expected, string(encoded[0]))
	}
}

func TestEncodeBinaryByteArrayInStruct(t *testing.T) {
	id := 23

	/* 51-/cool,23["a",{"_placeholder":true,"num":0}]
	<Buffer 61 62 63> */

	expected := `51-/cool,23["a",{"Test":{"_placeholder":true,"num":0}}]`

	m := Message{
		Type:      BinaryEvent,
		Namespace: "/cool",
		ID:        &id,
		Data:      []interface{}{"a", struct{ Test []byte }{Test: []byte("asdf")}},
	}

	encoded, err := m.Encode()

	if err != nil {
		t.Errorf("Unable to encode packet %v", err)
	}

	if len(encoded) != 2 {
		t.Errorf("Encoded message invalid. Expected 2 items, got %v", len(encoded))
	}

	if string(encoded[0]) != expected {
		t.Errorf("Encoded message invalid. Expected %v got %v", expected, string(encoded[0]))
	}
}

func TestDecodeEventPacketWithNoIDNoNamespaceNoData(t *testing.T) {

}

func TestDecodeBinaryPacketWithoutNamespaceWithoutAttachments(t *testing.T) {
	data := `523["a"]`

	m, err := DecodeMessage(data)

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

	m, err := DecodeMessage(data)

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

func TestDecodeBinaryPacketWithAttachments(t *testing.T) {
	data := `51-/cool,23["a",{"_placeholder":true,"num":0}]`

	m, err := DecodeMessage(data)

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
