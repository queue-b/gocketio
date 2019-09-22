package socket

import (
	"encoding/json"
	"fmt"
)

// PacketType is the Socket.IO-defined type of this packet
type PacketType int

const (
	// Connect represents a packet that is sent when the remote connection is
	// initially established
	Connect PacketType = iota
	// Disconnect represents a packet that is sent when the remote connection is broken
	// either intentionally or unintentionally
	Disconnect
	// Event represents a packet that contains event information. Packets of this type
	// may be sent by either side at any time after the connection is established
	Event
	// Ack represents a packet that contains a payload acknowledging receipt of a previously
	// sent Event
	Ack
	// Error represents a packet that contains error information
	Error
	// BinaryEvent represents a packet that contains event information. Packets of this type
	// may be sent by either side at any time after the connection is established
	BinaryEvent
	// BinaryAck represents a packet that contains a payload acknowledging receipt of a previously
	// sent Event
	BinaryAck
)

// Packet is a Socket.IO packet
type Packet struct {
	Type            PacketType
	ID              *int
	Namespace       string
	AttachmentCount int
	Data            interface{}
}

func (p Packet) String() string {
	ns := p.Namespace

	if ns == "" {
		ns = "/"
	}

	if p.Type == BinaryEvent || p.Type == BinaryAck {
		if p.ID != nil {
			return fmt.Sprintf("{Type:%v ID:%v Namespace:%v AttachmentCount:%v Data:%v}", messageTypeToMessageName[p.Type], *p.ID, ns, p.AttachmentCount, p.Data)
		}

		return fmt.Sprintf("{Type:%v Namespace:%v AttachmentCount:%v Data:%v}", messageTypeToMessageName[p.Type], ns, p.AttachmentCount, p.Data)
	}

	if p.ID != nil {
		return fmt.Sprintf("{Type:%v Namespace:%v ID:%v Data:%v}", messageTypeToMessageName[p.Type], ns, *p.ID, p.Data)
	}

	return fmt.Sprintf("{Type:%v Namespace:%v Data:%v}", messageTypeToMessageName[p.Type], ns, p.Data)
}

var messageTypeToMessageName = map[PacketType]string{
	Connect:     "Connect",
	Disconnect:  "Disconnect",
	Event:       "Event",
	Ack:         "Ack",
	Error:       "Error",
	BinaryEvent: "BinaryEvent",
	BinaryAck:   "BinaryAck",
}

func encodeAsString(p *Packet) ([][]byte, error) {
	encoded := fmt.Sprintf("%v", p.Type)

	if p.AttachmentCount > 0 {
		encoded += fmt.Sprintf("%v-", p.AttachmentCount)
	}

	if len(p.Namespace) != 0 && p.Namespace != "/" {
		encoded += p.Namespace + ","
	}

	if p.ID != nil {
		encoded += fmt.Sprintf("%v", *p.ID)
	}

	if p.Data != nil {
		d, err := json.Marshal(p.Data)

		if err != nil {
			return nil, err
		}

		encoded += string(d)
	}

	return [][]byte{[]byte(encoded)}, nil
}

func encodeAsBinary(p *Packet) ([][]byte, error) {
	attachments := make([][]byte, 0)

	encodedData, attachments := replaceByteArraysWithPlaceholders(p.Data, attachments)

	p.Data = encodedData
	p.AttachmentCount = len(attachments)

	encoded, err := encodeAsString(p)

	if err != nil {
		return nil, err
	}

	attachmentsAndPacket := make([][]byte, p.AttachmentCount+1)
	attachmentsAndPacket[0] = encoded[0]
	copy(attachmentsAndPacket[1:], attachments)

	return attachmentsAndPacket, nil
}

// Encode encodes a Packet into an array of []byte buffers
func (p *Packet) Encode(supportsBinary bool) ([][]byte, error) {
	if supportsBinary && p.Type == BinaryEvent || p.Type == BinaryAck {
		return encodeAsBinary(p)
	}

	return encodeAsString(p)
}
