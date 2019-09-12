package socket

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	Type            MessageType
	ID              *int
	Namespace       string
	AttachmentCount int
	Data            interface{}
}

var messageTypeToMessageName = map[MessageType]string{
	Connect:     "Connect",
	Disconnect:  "Disconnect",
	Event:       "Event",
	Ack:         "Ack",
	Error:       "Error",
	BinaryEvent: "BinaryEvent",
	BinaryAck:   "BinaryAck",
}

func DecodeMessage(message string) (*Message, error) {
	// Make sure the message received was at least 1 character (just the type)
	if len(message) < 1 {
		return nil, errors.New("Message too short")
	}

	decoded := &Message{}

	t, err := strconv.ParseInt(string(message[0]), 10, 64)

	if err != nil {
		return nil, err
	}

	decoded.Type = MessageType(t)

	fmt.Printf("Type %v\n", decoded.Type)

	// A Message is valid if it only includes a type
	if len(message) == 1 {
		return decoded, nil
	}

	remaining := message[1:]
	fmt.Printf("Searching %v\n", remaining)

	// Look for a dash; the characters (hopefully base 10 digits) before the dash
	// indicate the number of binary attachments for this binary message
	if decoded.Type == BinaryEvent || decoded.Type == BinaryAck {
		di := strings.Index(remaining, "-")

		if di >= 0 {
			attachmentCount, err := strconv.ParseInt(remaining[:di], 10, 64)

			if err != nil {
				return nil, err
			}

			decoded.AttachmentCount = int(attachmentCount)

			// Remove the attachment count, including the dash
			remaining = remaining[di+1:]
		}
	}

	fmt.Printf("Searching %v\n", remaining)

	ni := strings.Index(remaining, ",")
	fmt.Printf("Found comma at %v\n", ni)

	if ni != -1 {
		decoded.Namespace = remaining[:ni]
		fmt.Printf("Namespace %v", decoded.Namespace)

		if ni < len(remaining)-1 {
			remaining = remaining[ni+1:]
		}
	}

	fmt.Printf("Searching %v\n", remaining)

	// TODO: Don't assume UTF-8
	var idBytes []rune

	for _, v := range remaining {
		_, err := strconv.ParseInt(string(v), 10, 64)

		if err != nil {
			break
		}

		idBytes = append(idBytes, v)
	}

	id, err := strconv.ParseInt(string(idBytes), 10, 64)

	if err == nil {
		usableID := int(id)
		decoded.ID = &usableID
	}

	remaining = remaining[len(idBytes):]

	err = json.Unmarshal([]byte(remaining), &decoded.Data)

	if err != nil {
		return nil, err
	}

	return decoded, nil
}

func encodeAsString(m *Message) ([][]byte, error) {
	encoded := fmt.Sprintf("%v", m.Type)

	if m.AttachmentCount > 0 {
		encoded += fmt.Sprintf("%v-", m.AttachmentCount)
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
	m.AttachmentCount = len(attachments)

	encoded, err := encodeAsString(m)

	if err != nil {
		return nil, err
	}

	attachmentsAndPacket := make([][]byte, m.AttachmentCount+1)
	attachmentsAndPacket[0] = encoded[0]
	copy(attachmentsAndPacket[1:], attachments)

	return attachmentsAndPacket, nil
}

// Encode encodes a Message into an array of []byte buffers
func (m *Message) Encode() ([][]byte, error) {
	if m.Type == BinaryEvent || m.Type == BinaryAck {
		return encodeAsBinary(m)
	}

	return encodeAsString(m)
}
