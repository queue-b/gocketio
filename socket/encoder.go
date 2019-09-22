package socket

import (
	"reflect"
)

// BinaryPlaceholder represents the position of a particular binary
// attachement in a string-encoded packet
type BinaryPlaceholder struct {
	Placeholder bool `json:"_placeholder"`
	Number      int  `json:"num"`
}

// HasBinary returns true if the data contains []byte or fixed-length byte arrays, false otherwise
func HasBinary(data interface{}) bool {
	if data == nil {
		return false
	}

	switch data.(type) {
	case []byte:
		return true
	case int:
		return false
	case int8:
		return false
	case int16:
		return false
	case int32:
		return false
	case int64:
		return false
	case uint:
		return false
	case uint8:
		return false
	case uint16:
		return false
	case uint32:
		return false
	case uint64:
		return false
	case float32:
		return false
	case float64:
		return false
	case bool:
		return false
	case complex64:
		return false
	case complex128:
		return false
	case string:
		return false
	default:
		rt := reflect.TypeOf(data)
		rv := reflect.ValueOf(data)

		switch rt.Kind() {
		case reflect.Slice:
			fallthrough
		case reflect.Array:
			switch rt.Elem().Kind() {
			case reflect.Uint8:
				return true
			default:
				var beforeEncoding []interface{}

				// Convert fixed length array to []interface{}
				for i := 0; i < rv.Len(); i++ {
					beforeEncoding = append(beforeEncoding, rv.Index(i).Interface())
				}

				for _, v := range beforeEncoding {
					if HasBinary(v) {
						return true
					}
				}

				return false
			}
		case reflect.Struct:
			for i := 0; i < rt.NumField(); i++ {
				if HasBinary(rv.Field(i).Interface()) {
					return true
				}
			}
		}

		return false
	}
}

func replaceByteArraysWithPlaceholders(data interface{}, attachments [][]byte) (interface{}, [][]byte) {
	switch d := data.(type) {
	// For []byte, generate a placeholder and append the []byte to the buffers array
	case []byte:
		placeholder := BinaryPlaceholder{true, len(attachments)}
		attachments = append(attachments, d)
		return placeholder, attachments
	// For basic value types, just return the value
	case int:
		return d, attachments
	case int8:
		return d, attachments
	case int16:
		return d, attachments
	case int32:
		return d, attachments
	case int64:
		return d, attachments
	case uint:
		return d, attachments
	case uint8:
		return d, attachments
	case uint16:
		return d, attachments
	case uint32:
		return d, attachments
	case uint64:
		return d, attachments
	case float32:
		return d, attachments
	case float64:
		return d, attachments
	case bool:
		return d, attachments
	case complex64: // TODO: Do these have a direct JS equivalent?
		return d, attachments
	case complex128: // TODO: Do these have a direct JS equivalent?
		return d, attachments
	case string:
		return d, attachments
	default:
		rt := reflect.TypeOf(data)
		rv := reflect.ValueOf(data)

		switch rt.Kind() {
		case reflect.Slice:
			// If we receive a slice, loop through all the values and run the binary encoding method again
			// This ensures that any array member that is a []byte has the []byte appended to the attachments list
			var encoded []interface{}
			var encodedVal interface{}

			for _, val := range data.([]interface{}) {
				encodedVal, attachments = replaceByteArraysWithPlaceholders(val, attachments)
				encoded = append(encoded, encodedVal)
			}

			return encoded, attachments
		case reflect.Array:
			// If we receive an array, loop through all the values and run the binary encoding method again
			// This ensures that any array member that is a []byte has the []byte appended to the attachments list
			var encoded []interface{}
			var encodedVal interface{}

			// Special case for a fixed length []byte ([]uint8) array. Treat the same way as a []byte slice
			switch rt.Elem().Kind() {
			// Handle array of bytes or uints
			case reflect.Uint8:
				var binary []byte
				for i := 0; i < rv.Len(); i++ {
					binary = append(binary, rv.Index(i).Interface().(uint8))
				}

				encodedVal, attachments = replaceByteArraysWithPlaceholders(binary, attachments)
				return encodedVal, attachments
			}

			var beforeEncoding []interface{}

			// Convert fixed length array to []interface{}
			for i := 0; i < rv.Len(); i++ {
				beforeEncoding = append(beforeEncoding, rv.Index(i).Interface())
			}

			for _, val := range beforeEncoding {
				encodedVal, attachments = replaceByteArraysWithPlaceholders(val, attachments)
				encoded = append(encoded, encodedVal)
			}

			return encoded, attachments
		case reflect.Struct:
			// If we receive a struct, loop through all the keys and run the binary encoding method again
			// This ensures that any struct field that is a []byte has the []byte appended to the attachments list
			// and the value replaced by a placeholder
			encoded := make(map[string]interface{})
			var encodedVal interface{}

			for i := 0; i < rt.NumField(); i++ {
				f := rt.Field(i)
				// TODO: Add support for JSON annotations
				encodedVal, attachments = replaceByteArraysWithPlaceholders(rv.Field(i).Interface(), attachments)
				encoded[f.Name] = encodedVal
			}

			return encoded, attachments
		}

		return nil, nil
	}
}
