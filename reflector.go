package gocketio

import (
	"errors"
	"reflect"

	"github.com/mitchellh/mapstructure"
)

func isFunction(handler interface{}) error {
	if handler == nil {
		return errors.New("Handler is nil")
	}

	v := reflect.ValueOf(handler)

	if v.Kind() != reflect.Func {
		return errors.New("Handler is not a function")
	}

	return nil
}

func convertUnmarshalledJSONToReflectValues(callable reflect.Value, data interface{}) ([]reflect.Value, error) {
	if data == nil {
		return nil, errors.New("Data is nil")
	}

	var callableData []reflect.Value

	funcSignature := callable.Type()

	// TODO: Make sure data isn't an array initially. For how this code is used, this won't be an issue
	// because JSON.Unmarshal never returns fixed-length arrays
	// It may, however, return a variety of single values e.g. "hello"; if that's the case,
	// wrap them in an interface slice to simplify the code
	switch data.(type) {
	case []interface{}:
	default:
		data = []interface{}{data}
	}

	dataSlice := data.([]interface{})

	for i := 0; i < funcSignature.NumIn(); i++ {
		inType := funcSignature.In(i)

		// If there's still data available in the input data, try use that to fill the callable array
		if i < len(dataSlice) {
			// If the parameter is a struct, try to use the json map[string]interface{} to populate its fields
			if inType.Kind() == reflect.Struct {
				// This is a pointer to a "zero" value for the type
				val := reflect.New(inType).Interface()

				err := mapstructure.Decode(dataSlice[i], val)

				// If decoding fails, return an error
				if err != nil {
					return nil, err
				}

				// Append the decoded structure to the callable data
				callableData = append(callableData, reflect.ValueOf(val).Elem())

				continue
			}

			// If it's not a struct, just append the value
			callableData = append(callableData, reflect.ValueOf(dataSlice[i]))
		} else { // Otherwise pad the callable data with additional elements for the rest of the function parameters
			val := reflect.New(inType).Elem().Interface()
			callableData = append(callableData, reflect.ValueOf(val))
		}
	}

	return callableData, nil
}
