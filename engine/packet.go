package engine

import (
	"encoding/base64"
	"fmt"
)

// PacketType is the type of engine.io the packet being encoded or decoded
type PacketType uint8

// ParserProtocol is the version of the Engine.IO parser protocol that this library implements
const ParserProtocol int = 3

const (
	// Open is a packet that is sent when a connection is first opened
	Open PacketType = iota
	// Close is a packet that is sent when a connection is closed
	Close
	// Ping is a packet that is sent to indicated that the connection is still active
	Ping
	// Pong is a packet that is sent to indicate that the remote ping was received
	Pong
	// Message is a packet that is sent to transfer data
	Message
	// Upgrade is a packet that is sent to indicate that an upgrade should occur
	Upgrade
	// NoOp is a packet that is sent that should cause no action when received
	NoOp
)

// Packet represents a generic engine.io packet
type Packet interface {
	GetType() PacketType
	Encode(bool) ([]byte, error)
	GetData() []byte
}

// BinaryPacket represents a engine.io packet with binary (byte) contents
type BinaryPacket struct {
	Type PacketType
	Data []byte
}

// GetType returns the engine.io packet type of the packet
func (p *BinaryPacket) GetType() PacketType {
	return p.Type
}

// Encode returns the encoded engine.io packet
func (p *BinaryPacket) Encode(binary bool) ([]byte, error) {
	if !binary {
		message := fmt.Sprintf("b%v", p.Type)

		if p.Data != nil {
			message += base64.StdEncoding.EncodeToString(p.Data)
		}

		return []byte(message), nil
	}

	packet := []byte{byte(p.Type)}
	packet = append(packet, p.Data...)

	return packet, nil
}

// GetData returns the data contained in the packet
func (p *BinaryPacket) GetData() []byte {
	return p.Data
}

// StringPacket represents an engine.io packet with UTF-8 string contents
type StringPacket struct {
	Type PacketType
	Data *string
}

// GetType returns the engine.io packet type of the packet
func (p *StringPacket) GetType() PacketType {
	return p.Type
}

// Encode returns the encoded engine.io packet
func (p *StringPacket) Encode(binary bool) ([]byte, error) {
	encoded := fmt.Sprintf("%v", p.Type)

	if p.Data != nil {
		encoded += *p.Data
	}

	return []byte(encoded), nil
}

// GetData returns the data contained in the packet
func (p *StringPacket) GetData() []byte {
	if p.Data == nil {
		return nil
	}

	return []byte(*p.Data)
}
