package engine

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
