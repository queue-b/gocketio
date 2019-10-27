package transport

import "testing"

func TestDecodeStringPayload(t *testing.T) {
	testData := "10:b4AAECAwQ=8:4hello 亜"

	packets, err := DecodeStringPayload(testData)

	if err != nil {
		t.Fatalf("Unable to decode payload %v ", err)
	}

	if len(packets) != 2 {
		t.Fatalf("Expected 2 packets, got %v", len(packets))
	}

	firstPacket := packets[0]
	secondPacket := packets[1]

	switch p := firstPacket.(type) {
	case *BinaryPacket:
	default:
		t.Fatalf("Expected gocket.BinaryPacket, got %T", p)
	}

	switch p := secondPacket.(type) {
	case *StringPacket:
	default:
		t.Fatalf("Expected gocket.StringPacket, got %T", p)
	}
}

func TestDecodeBinaryPayload(t *testing.T) {
	testData := []byte{01, 06, 0xff, 04, 00, 01, 02, 03, 04, 00, 01, 00, 0xff, 34, 68, 65, 0x6c, 0x6c, 0x6f, 20, 0xe4, 0xba, 0x9c}

	packets, err := DecodeBinaryPayload(testData)

	if err != nil {
		t.Fatalf("Unable to decode payload %v ", err)
	}

	if len(packets) != 2 {
		t.Fatalf("Expected 2 packets, got %v", len(packets))
	}

	firstPacket := packets[0]
	secondPacket := packets[1]

	switch p := firstPacket.(type) {
	case *BinaryPacket:
	default:
		t.Fatalf("Expected gocket.BinaryPacket, got %T", p)
	}

	switch p := secondPacket.(type) {
	case *StringPacket:
	default:
		t.Fatalf("Expected gocket.StringPacket, got %T", p)
	}
}
