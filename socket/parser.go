package socket

import (
	"encoding/json"
	"fmt"
)

// PacketType is the Socket.IO-defined type of this packet
type PacketType int

const (
	// Connect is a packet that is sent when the remote connection is
	// initially established
	Connect PacketType = iota
	// Disconnect is a packet that is sent when the remote connection is broken
	// either intentionally or unintentionally
	Disconnect
	// Event is a packet that contains event information. Packets of this type
	// may be sent by either side at any time after the connection is established
	Event
	// Ack is a packet that contains a payload acknowledging receipt of a previously
	// sent Event
	Ack
	// Error is a packet that contains error information
	Error
	// BinaryEvent is a packet that contains event information. Packets of this type
	// may be sent by either side at any time after the connection is established
	BinaryEvent
	// BinaryAck is a packet that contains a payload acknowledging receipt of a previously
	// sent Event
	BinaryAck
)

// Packet is a Socket.IO packet
type Packet struct {
	// Type is the socket.io defined type of the packet
	Type PacketType
	// ID is set on outgoing Packets that require an Ack,
	// and on incoming packets that Ack the receipt of a Packet.
	ID *int64
	// Namespace is used to define which sockets will receive the Packet
	Namespace string
	// AttachmentCount is the number of binary attachments that will be sent
	// or received in addition to the main Packet
	AttachmentCount int
	// Data is the data contained in the packet
	Data interface{}
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

// Encode encodes a Packet into a format suitable for transmission
// using an engine.io transport.
// It returns a slice of byte slices which contain the socket.io encoded
// contents of the packet.
//
// The length of the slice returned by Encode is always at least 1, unless an
// error occurred during encoding. The exact length of the slice returned by Encode
// is dependant on the value of the Type & Data fields of the packet and the value
// of the supportsBinary argument. If supportsBinary is false, Encode returns a slice
// with a single element. If supportsBinary is true, and Type is either BinaryEvent
// or BinaryAck, Encode returns a slice with 1 + n additional elements, where
// n is the total number of byte slices and byte arrays present in the Data
// field, or are present at any level in any struct, array, or slice in the
// Data field.
//
// The first element is the raw bytes of a string-encoded Socket.IO packet.
// Additional elements are the raw bytes of a binary-encoded Socket.IO packet.
func (p *Packet) Encode(supportsBinary bool) ([][]byte, error) {
	if supportsBinary && p.Type == BinaryEvent || p.Type == BinaryAck {
		return encodeAsBinary(p)
	}

	return encodeAsString(p)
}
