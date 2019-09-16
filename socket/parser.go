package socket

import (
	"encoding/json"
	"fmt"
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
