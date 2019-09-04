package transport

import "testing"

func TestDecodeStringPacketWithBinaryPayload(t *testing.T) {
	testData := "b4AAECAwQ=" // type: message (4), contains binary data 1,2,3,4

	decodedPacket, err := DecodeStringPacket(testData)

	if err != nil {
		t.Errorf("Unable to decode string packet %v", err)
	}

	switch p := decodedPacket.(type) {
	case *BinaryPacket:
		if p.Type != Message {
			t.Errorf("Decoded packet.Type invalid. Expected %v, got %v", Message, p.Type)
		}

		if p.Data == nil {
			t.Error("Decoded packet.Data invalid. Expected non-nil, got nil")
		}

		if len(p.Data) != 5 {
			t.Errorf("Decoded packet.Data invalid. Expected length to be 5, got %v", len(p.Data))
		}

		expectedData := []byte{0, 1, 2, 3, 4}

		for i := 0; i < 5; i++ {
			if expectedData[i] != p.Data[i] {
				t.Errorf("Decoded packet.Data invalid at index %v. Expected %v got %v", i, expectedData[i], p.Data[i])
			}
		}
	default:
		t.Error("Received wrong output packet type")
	}
}

func TestDecodeStringPacketWithStringPayload(t *testing.T) {
	testData := "4hello 亜"

	decodedPacket, err := DecodeStringPacket(testData)

	if err != nil {
		t.Errorf("Unable to decode packet %v", err)
	}

	switch p := decodedPacket.(type) {
	case *StringPacket:
		if p.Type != Message {
			t.Errorf("Decoded packet.Type invalid. Expected %v, got %v", Message, p.Type)
		}

		if p.Data == nil {
			t.Errorf("Decoded packet.Data invalid. Expected non-nil, got nil")
		}

		if len([]rune(*p.Data)) != 7 {
			t.Errorf("Decoded packet.Data invalid. Expected length 7, got %v", len(*p.Data))
		}

		if *p.Data != "hello 亜" {
			t.Errorf("Decoded packet.Data invalid. Expected 'hello 亜', got %v", *p.Data)
		}
	default:
		t.Errorf("Expected gocket.StringPacket got %T", p)
	}
}

func TestDecodeBinaryPacket(t *testing.T) {
	testData := []byte{04, 00, 01, 02, 03, 04}

	decodedPacket, err := DecodeBinaryPacket(testData)

	if err != nil {
		t.Errorf("Unable to decode binary packet %v", err)
	}

	switch p := decodedPacket.(type) {
	case *BinaryPacket:
		if p.Type != Message {
			t.Errorf("Decoded packet.Type invalid. Expected %v, got %v", Message, p.Type)
		}

		if p.Data == nil {
			t.Error("Decoded packet.Data invalid. Expected non-nil, got nil")
		}

		if len(p.Data) != 5 {
			t.Errorf("Decoded packet.Data invalid. Expected length to be 5, got %v", len(p.Data))
		}

		expectedData := []byte{0, 1, 2, 3, 4}

		for i := 0; i < 5; i++ {
			if expectedData[i] != p.Data[i] {
				t.Errorf("Decoded packet.Data invalid at index %v. Expected %v got %v", i, expectedData[i], p.Data[i])
			}
		}
	default:
		t.Error("Received wrong output packet type")
	}
}

func TestDecodeStringPayload(t *testing.T) {
	testData := "10:b4AAECAwQ=8:4hello 亜"

	packets, err := DecodeStringPayload(testData)

	if err != nil {
		t.Errorf("Unable to decode payload %v ", err)
	}

	if len(packets) != 2 {
		t.Errorf("Expected 2 packets, got %v", len(packets))
	}

	firstPacket := packets[0]
	secondPacket := packets[1]

	switch p := firstPacket.(type) {
	case *BinaryPacket:
	default:
		t.Errorf("Expected gocket.BinaryPacket, got %T", p)
	}

	switch p := secondPacket.(type) {
	case *StringPacket:
	default:
		t.Errorf("Expected gocket.StringPacket, got %T", p)
	}
}

func TestDecodeBinaryPayload(t *testing.T) {
	testData := []byte{01, 06, 0xff, 04, 00, 01, 02, 03, 04, 00, 01, 00, 0xff, 34, 68, 65, 0x6c, 0x6c, 0x6f, 20, 0xe4, 0xba, 0x9c}

	packets, err := DecodeBinaryPayload(testData)

	if err != nil {
		t.Errorf("Unable to decode payload %v ", err)
	}

	if len(packets) != 2 {
		t.Errorf("Expected 2 packets, got %v", len(packets))
	}

	firstPacket := packets[0]
	secondPacket := packets[1]

	switch p := firstPacket.(type) {
	case *BinaryPacket:
	default:
		t.Errorf("Expected gocket.BinaryPacket, got %T", p)
	}

	switch p := secondPacket.(type) {
	case *StringPacket:
	default:
		t.Errorf("Expected gocket.StringPacket, got %T", p)
	}
}

func TestEncodeBinaryPacketAsString(t *testing.T) {
	packet := BinaryPacket{
		Type: Message,
		Data: []byte{0, 1, 2, 3, 4},
	}

	expected := "b4AAECAwQ="

	encoded, err := packet.Encode(false)

	if err != nil {
		t.Errorf("Unable to encode packet %v", err)
	}

	if string(encoded) != expected {
		t.Errorf("Encoded packet invalid. Expected %v got %v", expected, encoded)
	}
}

func TestEncodeBinaryPacketAsBinary(t *testing.T) {
	packet := BinaryPacket{
		Type: Message,
		Data: []byte{0, 1, 2, 3, 4},
	}

	expected := []byte{byte(Message), 0, 1, 2, 3, 4}

	encoded, err := packet.Encode(true)

	if err != nil {
		t.Errorf("Unable to encode packet %v", err)
	}

	if encoded == nil {
		t.Error("Encoded packet invalid. Expected non-nil got nil")
	}

	if len(encoded) != len(expected) {
		t.Errorf("Encoded packet invalid. Expected length %v got length %v", len(expected), len(encoded))
	}

	for i, v := range encoded {
		if v != expected[i] {
			t.Errorf("Encoded packet invalid. Expected %v at %v, got %v", expected[i], i, v)
		}
	}
}

func TestEncodeStringPacketWithData(t *testing.T) {
	data := "hello 亜"

	packet := StringPacket{
		Type: Message,
		Data: &data,
	}

	expected := "4hello 亜"

	encoded, err := packet.Encode(false)

	if err != nil {
		t.Errorf("Unable to encode packet %v", err)
	}

	if encoded == nil {
		t.Error("Encoded packet invalid. Expected non-nil got nil")
	}

	if len(encoded) != len(expected) {
		t.Errorf("Encoded packet invalid. Expected length %v got length %v", len(expected), len(encoded))
	}

	if string(encoded) != expected {
		t.Errorf("Encoded packet invalid. Expected %v, got %v", expected, string(encoded))
	}
}

func TestEncodeStringPacketWithoutData(t *testing.T) {
	packet := StringPacket{
		Type: Message,
	}

	expected := "4"

	encoded, err := packet.Encode(false)

	if err != nil {
		t.Errorf("Unable to encode packet %v", err)
	}

	if encoded == nil {
		t.Error("Encoded packet invalid. Expected non-nil got nil")
	}

	if len(encoded) != len(expected) {
		t.Errorf("Encoded packet invalid. Expected length %v got length %v", len(expected), len(encoded))
	}

	if string(encoded) != expected {
		t.Errorf("Encoded packet invalid. Expected %v, got %v", expected, string(encoded))
	}
}
