package gocketio

import (
	"reflect"
	"testing"
)

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

// func TestConvertUnmarshalledJSONToReflectValuesWithTypeAlias(t *testing.T) {
// 	fn := func(a webrtc.SessionDescription) {}

// 	fnVal := reflect.ValueOf(fn)

// 	session := webrtc.SessionDescription{
// 		Type: webrtc.SDPTypeOffer,
// 		SDP:  "sdp",
// 	}

// 	sessionBytes, err := json.Marshal(session)

// 	if err != nil {
// 		t.Fatalf("Unable to marshal %v\n", err)
// 	}

// 	fmt.Println(string(sessionBytes))

// 	var test interface{}

// 	json.Unmarshal(sessionBytes, &test)

// 	fmt.Println(test)

// 	_, err = convertUnmarshalledJSONToReflectValues(fnVal, []interface{}{test})

// 	if err != nil {
// 		t.Fatalf("Unable to unmarshal %v\n", err)
// 	}
// }

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

}
