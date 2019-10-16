package engine

import (
	"encoding/base64"
	"errors"
	"strconv"
)

// DecodeBinaryPacket returns a BinaryPacket from the contents of the byte array
func DecodeBinaryPacket(packet []byte) (Packet, error) {
	if packet == nil || len(packet) < 2 {
		return &BinaryPacket{}, errors.New("Invalid packet")
	}

	return &BinaryPacket{
		Type: PacketType(packet[0]),
		Data: packet[1:],
	}, nil
}

// DecodeStringPacket returns a StringPacket or BinaryPacket from the contents of the string
func DecodeStringPacket(packet string) (Packet, error) {
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
