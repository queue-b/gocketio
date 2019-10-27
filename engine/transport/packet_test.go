package transport

import "testing"

func TestEncodeBinary(t *testing.T) {
	packet := BinaryPacket{
		Type: Message,
		Data: []byte{0, 1, 2, 3, 4},
	}

	t.Run("AsString", func(t *testing.T) {
		t.Parallel()
		expected := "b4AAECAwQ="

		encoded, err := packet.Encode(false)

		if err != nil {
			t.Fatalf("Unable to encode packet %v", err)
		}

		if string(encoded) != expected {
			t.Fatalf("Encoded packet invalid. Expected %v got %v", expected, encoded)
		}
	})

	t.Run("AsBinary", func(t *testing.T) {
		t.Parallel()
		expected := []byte{byte(Message), 0, 1, 2, 3, 4}

		encoded, err := packet.Encode(true)

		if err != nil {
			t.Fatalf("Unable to encode packet %v", err)
		}

		if encoded == nil {
			t.Error("Encoded packet invalid. Expected non-nil got nil")
		}

		if len(encoded) != len(expected) {
			t.Fatalf("Encoded packet invalid. Expected length %v got length %v", len(expected), len(encoded))
		}

		for i, v := range encoded {
			if v != expected[i] {
				t.Fatalf("Encoded packet invalid. Expected %v at %v, got %v", expected[i], i, v)
			}
		}
	})
}

func TestEncodeString(t *testing.T) {
	t.Run("WithData", func(t *testing.T) {
		t.Parallel()

		data := "hello 亜"

		packet := StringPacket{
			Type: Message,
			Data: &data,
		}

		expected := "4hello 亜"

		encoded, err := packet.Encode(false)

		if err != nil {
			t.Fatalf("Unable to encode packet %v", err)
		}

		if encoded == nil {
			t.Error("Encoded packet invalid. Expected non-nil got nil")
		}

		if len(encoded) != len(expected) {
			t.Fatalf("Encoded packet invalid. Expected length %v got length %v", len(expected), len(encoded))
		}

		if string(encoded) != expected {
			t.Fatalf("Encoded packet invalid. Expected %v, got %v", expected, string(encoded))
		}
	})

	t.Run("WithoutData", func(t *testing.T) {
		t.Parallel()
		packet := StringPacket{
			Type: Message,
		}

		expected := "4"

		encoded, err := packet.Encode(false)

		if err != nil {
			t.Fatalf("Unable to encode packet %v", err)
		}

		if encoded == nil {
			t.Error("Encoded packet invalid. Expected non-nil got nil")
		}

		if len(encoded) != len(expected) {
			t.Fatalf("Encoded packet invalid. Expected length %v got length %v", len(expected), len(encoded))
		}

		if string(encoded) != expected {
			t.Fatalf("Encoded packet invalid. Expected %v, got %v", expected, string(encoded))
		}
	})
}

func TestDecodeStringPacket(t *testing.T) {
	t.Run("WithBinaryPayload", func(t *testing.T) {
		t.Parallel()
		testData := "b4AAECAwQ=" // type: message (4), contains binary data 1,2,3,4

		decodedPacket, err := DecodeStringPacket(testData)

		if err != nil {
			t.Fatalf("Unable to decode string packet %v", err)
		}

		switch p := decodedPacket.(type) {
		case *BinaryPacket:
			if p.Type != Message {
				t.Fatalf("Decoded packet.Type invalid. Expected %v, got %v", Message, p.Type)
			}

			if p.Data == nil {
				t.Error("Decoded packet.Data invalid. Expected non-nil, got nil")
			}

			if len(p.Data) != 5 {
				t.Fatalf("Decoded packet.Data invalid. Expected length to be 5, got %v", len(p.Data))
			}

			expectedData := []byte{0, 1, 2, 3, 4}

			for i := 0; i < 5; i++ {
				if expectedData[i] != p.Data[i] {
					t.Fatalf("Decoded packet.Data invalid at index %v. Expected %v got %v", i, expectedData[i], p.Data[i])
				}
			}
		default:
			t.Error("Received wrong output packet type")
		}
	})

	t.Run("WithStringPayload", func(t *testing.T) {
		t.Parallel()
		testData := "4hello 亜"

		decodedPacket, err := DecodeStringPacket(testData)

		if err != nil {
			t.Fatalf("Unable to decode packet %v", err)
		}

		switch p := decodedPacket.(type) {
		case *StringPacket:
			if p.Type != Message {
				t.Fatalf("Decoded packet.Type invalid. Expected %v, got %v", Message, p.Type)
			}

			if p.Data == nil {
				t.Fatalf("Decoded packet.Data invalid. Expected non-nil, got nil")
			}

			if len([]rune(*p.Data)) != 7 {
				t.Fatalf("Decoded packet.Data invalid. Expected length 7, got %v", len(*p.Data))
			}

			if *p.Data != "hello 亜" {
				t.Fatalf("Decoded packet.Data invalid. Expected 'hello 亜', got %v", *p.Data)
			}
		default:
			t.Fatalf("Expected gocket.StringPacket got %T", p)
		}
	})

	t.Run("WithMinLength", func(t *testing.T) {
		t.Parallel()
		p, err := DecodeStringPacket("0")

		if err != nil {
			t.Fatalf("Unexpected error while decoding string packet %v\n", err)
		}

		if p.GetType() != Open {
			t.Fatal("Expected Open packet type")
		}
	})

	t.Run("WithLessThanMinLength", func(t *testing.T) {
		t.Parallel()
		_, err := DecodeStringPacket("")

		if err != ErrPacketTooShort {
			t.Fatal("Should return error on short packet")
		}
	})

	t.Run("InvalidType", func(t *testing.T) {
		t.Parallel()
		_, err := DecodeStringPacket("z")

		if err != ErrInvalidType {
			t.Fatal("Should return error on invalid packet type")
		}
	})

	t.Run("MalformedBinaryString", func(t *testing.T) {
		t.Parallel()
		_, err := DecodeStringPacket("bzxxx")

		if err != ErrInvalidType {
			t.Fatal("Should return error on invalid packet type")
		}

		_, err = DecodeStringPacket("b4AAECAw")

		if err == nil {
			t.Fatalf("Should return error on invalid base64 encoding")
		}
	})
}
func TestDecodeBinaryPacket(t *testing.T) {
	t.Run("WithNormalPayload", func(t *testing.T) {
		t.Parallel()
		testData := []byte{04, 00, 01, 02, 03, 04}

		decodedPacket, err := DecodeBinaryPacket(testData)

		if err != nil {
			t.Fatalf("Unable to decode binary packet %v", err)
		}

		switch p := decodedPacket.(type) {
		case *BinaryPacket:
			if p.Type != Message {
				t.Fatalf("Decoded packet.Type invalid. Expected %v, got %v", Message, p.Type)
			}

			if p.Data == nil {
				t.Error("Decoded packet.Data invalid. Expected non-nil, got nil")
			}

			if len(p.Data) != 5 {
				t.Fatalf("Decoded packet.Data invalid. Expected length to be 5, got %v", len(p.Data))
			}

			expectedData := []byte{0, 1, 2, 3, 4}

			for i := 0; i < 5; i++ {
				if expectedData[i] != p.Data[i] {
					t.Fatalf("Decoded packet.Data invalid at index %v. Expected %v got %v", i, expectedData[i], p.Data[i])
				}
			}
		default:
			t.Error("Received wrong output packet type")
		}
	})

	t.Run("WithLessThanMinLength", func(t *testing.T) {
		t.Parallel()
		_, err := DecodeBinaryPacket([]byte{0})

		if err != ErrPacketTooShort {
			t.Fatal("Should return error on short packet")
		}
	})

}
