package engine

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
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

// DecodeBinaryPayload returns an array of String or Binary packets from the contents of the byte array
func DecodeBinaryPayload(payload []byte) ([]Packet, error) {
	var packets []Packet
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
func DecodeStringPayload(payload string) ([]Packet, error) {
	if len(payload) == 0 {
		return nil, errors.New("Invalid payload")
	}

	var packets []Packet

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
