package transport

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// PacketType is the type of engine.io the packet being encoded or decoded
type PacketType uint8

const ParserProtocol int = 3

const (
	Open PacketType = iota
	Close
	Ping
	Pong
	Message
	Upgrade
	NoOp
)

// EnginePacket represents a generic engine.io packet
type EnginePacket interface {
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

func (p *StringPacket) GetData() []byte {
	if p.Data == nil {
		return nil
	}

	return []byte(*p.Data)
}

// DecodeBinaryPacket returns a BinaryPacket from the contents of the byte array
func DecodeBinaryPacket(packet []byte) (EnginePacket, error) {
	if packet == nil || len(packet) < 2 {
		return &BinaryPacket{}, errors.New("Invalid packet")
	}

	return &BinaryPacket{
		Type: PacketType(packet[0]),
		Data: packet[1:],
	}, nil
}

// DecodeStringPacket returns a StringPacket or BinaryPacket from the contents of the string
func DecodeStringPacket(packet string) (EnginePacket, error) {
	if len(packet) < 1 {
		return &StringPacket{}, errors.New("Invalid packet")
	}

	arePacketContentsBinary := packet[0] == 'b'

	if arePacketContentsBinary {
		packetTypeByte, err := strconv.ParseInt(string(packet[1]), 10, 32)

		if err != nil {
			return &BinaryPacket{}, errors.New("Unable to parse type byte")
		}

		decoded, err := base64.StdEncoding.DecodeString(string(packet[2:]))

		if err != nil {
			return &BinaryPacket{}, err
		}

		return &BinaryPacket{
			Type: PacketType(packetTypeByte),
			Data: decoded,
		}, nil
	}

	packetTypeByte, err := strconv.ParseInt(string(packet[0]), 10, 32)

	if err != nil {
		return &BinaryPacket{}, errors.New("Unable to parse type byte")
	}

	if len(packet) > 1 {
		data := string(packet[1:])

		return &StringPacket{
			Type: PacketType(packetTypeByte),
			Data: &data,
		}, nil
	}

	return &StringPacket{
		Type: PacketType(packetTypeByte),
	}, nil
}

// DecodeBinaryPayload returns an array of String or Binary packets from the contents of the byte array
func DecodeBinaryPayload(payload []byte) ([]EnginePacket, error) {
	var packets []EnginePacket
	remaining := payload

	for {
		fmt.Println(len(remaining))
		stringLength := ""

		if len(remaining) == 0 {
			break
		}

		isString := remaining[0] == 0
		var i int
		var val byte

		for i, val = range remaining {
			if val == 255 {
				break
			}

			if len(stringLength) > 310 {
				return nil, errors.New("String length too long")
			}

			stringLength += fmt.Sprintf("%v", val)
		}

		messageLength, err := strconv.ParseInt(stringLength, 10, 32)

		if err != nil {
			return nil, err
		}

		msgStart := i + 1
		msgEnd := msgStart + int(messageLength)

		msg := remaining[msgStart:msgEnd]

		if isString {

			packet, err := DecodeStringPacket(string(msg))

			if err != nil {
				return nil, err
			}

			packets = append(packets, packet)
		} else {
			packet, err := DecodeBinaryPacket(msg)

			if err != nil {
				return nil, err
			}

			packets = append(packets, packet)
		}

		if msgEnd == len(remaining) {
			break
		}

		remaining = remaining[msgEnd:]
	}

	return packets, nil
}

// DecodeStringPayload returns an array of String or Binary packets from the contents of the byte array
func DecodeStringPayload(payload string) ([]EnginePacket, error) {
	if len(payload) == 0 {
		return nil, errors.New("Invalid payload")
	}

	var packets []EnginePacket

	remaining := payload

	for {
		idx := strings.Index(remaining, ":")

		if idx == -1 {
			break
		}

		fmt.Println("Index", idx)

		length, err := strconv.ParseInt(remaining[0:idx], 10, 32)

		if err != nil {
			return nil, err
		}

		fmt.Println("Length", length)

		msgStart := idx + 1
		msgEnd := msgStart + int(length)

		msg := remaining[msgStart:msgEnd]

		fmt.Println("Message", msg)
		if int(length) != len(msg) {
			return nil, errors.New("Invalid payload")
		}

		if len(msg) > 0 {
			packet, err := DecodeStringPacket(msg)

			if err != nil {
				return nil, err
			}

			packets = append(packets, packet)
		}

		remaining = remaining[msgEnd:]
	}

	return packets, nil
}
