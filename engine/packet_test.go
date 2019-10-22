package engine

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
			t.Errorf("Unable to encode packet %v", err)
		}

		if string(encoded) != expected {
			t.Errorf("Encoded packet invalid. Expected %v got %v", expected, encoded)
		}
	})

	t.Run("AsBinary", func(t *testing.T) {
		t.Parallel()
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
	})

	t.Run("WithoutData", func(t *testing.T) {
		t.Parallel()
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
	})
}
