package socket

import "testing"

func TestEncodeConnectWithID(t *testing.T) {
	id := int64(1)
	p := Packet{
		ID:   &id,
		Type: Connect,
		Data: []string{"test"},
	}

	expected := `01["test"]`

	encoded, err := p.Encode(true)

	if err != nil {
		t.Errorf("Unable to encode %v", err)
	}

	if string(encoded[0]) != expected {
		t.Errorf("Encoded message invalid. Expected %v got %v", expected, string(encoded[0]))
	}
}

func TestEncodeDisconnect(t *testing.T) {
	p := Packet{
		Type:      Disconnect,
		Namespace: "/woot",
	}

	expected := `1/woot,`

	encoded, err := p.Encode(true)

	if err != nil {
		t.Errorf("Unable to encode %v", err)
	}

	if string(encoded[0]) != expected {
		t.Errorf("Encoded message invalid. Expected %v got %v", expected, string(encoded[0]))
	}
}

func TestEncodeEventWithNamespaceAndID(t *testing.T) {
	id := int64(1)
	p := Packet{
		Type:      Event,
		Namespace: "/test",
		ID:        &id,
		Data:      []interface{}{"a", 1, struct{}{}},
	}

	expected := `2/test,1["a",1,{}]`

	encoded, err := p.Encode(true)

	if err != nil {
		t.Errorf("Unable to encode %v", err)
	}

	if string(encoded[0]) != expected {
		t.Errorf("Encoded message invalid. Expected %v got %v", expected, string(encoded[0]))
	}
}

func TestEncodeSimpleBinary(t *testing.T) {
	id := int64(23)

	/* 51-/cool,23["a",{"_placeholder":true,"num":0}]
	<Buffer 61 62 63> */

	expected := `51-/cool,23["a",{"_placeholder":true,"num":0}]`

	p := Packet{
		Type:      BinaryEvent,
		Namespace: "/cool",
		ID:        &id,
		Data:      []interface{}{"a", []byte("abc")},
	}

	encoded, err := p.Encode(true)

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
	id := int64(23)

	/* 51-/cool,23["a",{"_placeholder":true,"num":0}]
	<Buffer 61 62 63> */

	expected := `51-/cool,23["a",{"_placeholder":true,"num":0}]`

	p := Packet{
		Type:      BinaryEvent,
		Namespace: "/cool",
		ID:        &id,
		Data:      []interface{}{"a", [1]byte{10}},
	}

	encoded, err := p.Encode(true)

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
	id := int64(23)

	/* 51-/cool,23["a",{"_placeholder":true,"num":0}]
	<Buffer 61 62 63> */

	expected := `51-/cool,23["a",{"Test":{"_placeholder":true,"num":0}}]`

	p := Packet{
		Type:      BinaryEvent,
		Namespace: "/cool",
		ID:        &id,
		Data:      []interface{}{"a", struct{ Test []byte }{Test: []byte("asdf")}},
	}

	encoded, err := p.Encode(true)

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
