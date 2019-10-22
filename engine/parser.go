package engine

import (
	"encoding/base64"
	"errors"
	"strconv"
)

// ErrPacketTooShort is returned when a packet does not contain enough bytes to be parsed
var ErrPacketTooShort = errors.New("Packet is too short")

// ErrInvalidType is returned when a packet contains an invalid type
var ErrInvalidType = errors.New("Invalid packet type")

// DecodeBinaryPacket returns a BinaryPacket from the contents of the byte array
func DecodeBinaryPacket(packet []byte) (Packet, error) {
	if packet == nil || len(packet) < 2 {
		return &BinaryPacket{}, ErrPacketTooShort
	}

	return &BinaryPacket{
		Type: PacketType(packet[0]),
		Data: packet[1:],
	}, nil
}

// DecodeStringPacket returns a StringPacket or BinaryPacket from the contents of the string
func DecodeStringPacket(packet string) (Packet, error) {
	if len(packet) < 1 {
		return &StringPacket{}, ErrPacketTooShort
	}

	arePacketContentsBinary := packet[0] == 'b'

	if arePacketContentsBinary {
		packetTypeByte, err := strconv.ParseInt(string(packet[1]), 10, 32)

		if err != nil {
			return &BinaryPacket{}, ErrInvalidType
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
		return &BinaryPacket{}, ErrInvalidType
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
