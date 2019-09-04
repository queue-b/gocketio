package gocket

import (
	"encoding/json"
	"fmt"
	"reflect"
)

type MessageType int

const (
	Connect MessageType = iota
	Disconnect
	Event
	Ack
	Error
	BinaryEvent
	BinaryAck
)

type Message struct {
	Type      MessageType
	ID        *int
	Namespace string
	Data      interface{}
}

type binaryPackedPacket struct {
	Message           Message
	AdditionalBuffers [][]byte
}

// BinaryPlaceholder represents the position of a particular binary
// attachement in a string-encoded packet
type BinaryPlaceholder struct {
	Placeholder bool `json:"_placeholder"`
	Number      int  `json:"num"`
}

func replaceByteArraysWithPlaceholders(data interface{}, attachments [][]byte) (interface{}, [][]byte) {
	fmt.Println(reflect.TypeOf(data).Kind())
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
				fmt.Printf("Encoding %v\n", f.Name)
				// TODO: Add support for JSON annotations
				encodedVal, attachments = replaceByteArraysWithPlaceholders(rv.Field(i).Interface(), attachments)
				encoded[f.Name] = encodedVal
			}

			return encoded, attachments
		}
	}

	return nil, nil
}

func encodeAsString(m *Message, attachmentCount *int) ([][]byte, error) {
	encoded := fmt.Sprintf("%v", m.Type)

	if attachmentCount != nil && *attachmentCount > 0 {
		encoded += fmt.Sprintf("%v-", *attachmentCount)
	}

	if len(m.Namespace) != 0 && m.Namespace != "/" {
		encoded += m.Namespace + ","
	}

	if m.ID != nil {
		encoded += fmt.Sprintf("%v", *m.ID)
	}

	if m.Data != nil {
		d, err := json.Marshal(m.Data)

		if err != nil {
			return nil, err
		}

		encoded += string(d)
	}

	return [][]byte{[]byte(encoded)}, nil
}

func encodeAsBinary(m *Message) ([][]byte, error) {
	attachments := make([][]byte, 0)

	encodedData, attachments := replaceByteArraysWithPlaceholders(m.Data, attachments)

	m.Data = encodedData

	attachmentCount := len(attachments)

	encoded, err := encodeAsString(m, &attachmentCount)

	if err != nil {
		return nil, err
	}

	attachmentsAndPacket := make([][]byte, attachmentCount+1)
	attachmentsAndPacket[0] = encoded[0]
	copy(attachmentsAndPacket[1:], attachments)

	return attachmentsAndPacket, nil
}

// Encode encodes a Message into an array of []byte buffers
func (m *Message) Encode() ([][]byte, error) {
	if m.Type == BinaryEvent || m.Type == BinaryAck {
		return encodeAsBinary(m)
	}

	return encodeAsString(m, nil)
}
