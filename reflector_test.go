package gocketio

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type testIntAlias int

const (
	aliasTypeUnknown testIntAlias = iota
)

// UnmarshalJSON enables JSON unmarshaling of a SDPType
func (t *testIntAlias) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	switch strings.ToLower(s) {
	default:
		return errors.New("Bad encoding")
	case "unknown":
		*t = aliasTypeUnknown
	}

	return nil
}

type testUnmarshalWithUnmarshalJSON struct {
	TestAlias testIntAlias
}

type testUnmarshalType struct {
	First  string
	Second int
}

func TestIsFunction(t *testing.T) {
	err := isFunction(nil)

	if err == nil {
		t.Error("Expected error, received none")
	}

	err = isFunction(5)

	if err == nil {
		t.Errorf("Expected error, received none")
	}

	fn := func(a string) {}

	err = isFunction(fn)

	if err != nil {
		t.Errorf("Expected no error, received %v", err)
	}
}

func TestConvertUnmarshalledJSONToReflectValues(t *testing.T) {
	fn := func(a string) {}
	fnVal := reflect.ValueOf(fn)

	vals, err := convertUnmarshalledJSONToReflectValues(fnVal, "a")

	if err != nil {
		t.Errorf("Unable to convert %v", err)
	}

	if len(vals) != 1 {
		t.Errorf("Expected 1 converted value, received %v", len(vals))
	}

	vals, err = convertUnmarshalledJSONToReflectValues(fnVal, []int{1, 2, 3})

	if err != nil {
		t.Errorf("Unable to convert %v", err)
	}

	if len(vals) != 1 {
		t.Errorf("Expected 1 converted value, received %v", len(vals))
	}

	fnStruct := func(a testUnmarshalType) {}
	fnVal = reflect.ValueOf(fnStruct)

	vals, err = convertUnmarshalledJSONToReflectValues(fnVal, map[string]interface{}{
		"first":  "the best",
		"second": 5,
	})

	if err != nil {
		t.Errorf("Unable to convert %v", err)
	}

	if len(vals) != 1 {
		t.Errorf("Expected 1 converted value, received %v", len(vals))
	}

	switch first := vals[0].Interface().(type) {
	case testUnmarshalType:
		if first.First != "the best" {
			t.Errorf("Expected .First to be 'the best', got %v", first.First)
		}

		if first.Second != 5 {
			t.Errorf("Expected .Second to be 5, got %v", first.Second)
		}
	default:
		t.Errorf("Expected testUnmarshalType, got %T", first)
	}

	fnMismatchedLengthData := func(a string, b int32, c testUnmarshalType) {}
	fnVal = reflect.ValueOf(fnMismatchedLengthData)

	vals, err = convertUnmarshalledJSONToReflectValues(fnVal, []interface{}{"hello"})

	if err != nil {
		t.Errorf("Unable to convert %v", err)
	}

	if len(vals) != 3 {
		t.Errorf("Expected 3 converted values, received %v", len(vals))
	}

	switch first := vals[0].Interface().(type) {
	case string:
		if first != "hello" {
			t.Errorf("Expected first to be 'hello', got %v", first)
		}
	default:
		t.Errorf("Expected string, got %T", first)
	}

	switch second := vals[1].Interface().(type) {
	case int32:
		if second != 0 {
			t.Errorf("Expected second to be 0, got %v", second)
		}
	default:
		t.Errorf("Expected int32, got %T", second)
	}

	switch third := vals[2].Interface().(type) {
	case testUnmarshalType:
	default:
		t.Errorf("Expected testUnmarshalType, got %T", third)
	}

	fnStructForUnmarshalJSON := func(a testUnmarshalWithUnmarshalJSON) {}
	fnVal = reflect.ValueOf(fnStructForUnmarshalJSON)

	vals, err = convertUnmarshalledJSONToReflectValues(fnVal, map[string]interface{}{
		"TestAlias": "unknown",
	})

	if err != nil {
		t.Errorf("Unable to convert %v", err)
	}

	if len(vals) != 1 {
		t.Errorf("Expected 1 converted value, received %v", len(vals))
	}

	switch first := vals[0].Interface().(type) {
	case testUnmarshalWithUnmarshalJSON:
		if first.TestAlias != aliasTypeUnknown {
			t.Errorf("Expected .TestAlias to be 'aliasTypeUnknown', got %v", first.TestAlias)
		}
	default:
		t.Errorf("Expected testUnmarshalWithUnmarshalJSON, got %T", first)
	}

}
