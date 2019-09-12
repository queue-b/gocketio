package socket

import (
	"encoding/json"
	"testing"
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

	replaced := replacePlaceholdersWithByteSlices(decoded, nil)

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

	replaced := replacePlaceholdersWithByteSlices(decoded, nil)

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

	replaced := replacePlaceholdersWithByteSlices(decoded, nil)

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

	replaced := replacePlaceholdersWithByteSlices(decoded, attachments)

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

	replaced := replacePlaceholdersWithByteSlices(decoded, attachments)

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
