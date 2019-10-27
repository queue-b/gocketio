package transport

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func encodeNonBinaryPacket(packet Packet) ([]byte, error) {
	content, err := packet.Encode(false)

	if err != nil {
		return nil, err
	}

	return content, nil
}

func encodeBinaryPacket(packet Packet) ([]byte, error) {
	content, err := packet.Encode(true)

	if err != nil {
		return nil, err
	}

	encodedLength := fmt.Sprintf("%d", len(content))
	encodedLengthBytes := []byte(encodedLength)

	sizeBuffer := make([]byte, len(encodedLengthBytes)+2)

	switch packet.(type) {
	case *StringPacket:
		sizeBuffer[0] = 0
	case *BinaryPacket:
		sizeBuffer[0] = 1
	}

	sizeBuffer = append(sizeBuffer, encodedLengthBytes...)
	sizeBuffer = append(sizeBuffer, byte(255))

	return append(sizeBuffer, content...), nil
}

func encodeBinaryPayload(packets ...Packet) ([]byte, error) {
	var payload []byte

	for _, p := range packets {
		encoded, err := encodeBinaryPacket(p)

		if err != nil {
			return nil, err
		}

		payload = append(payload, encoded...)
	}

	return payload, nil
}

func encodeNonBinaryPayload(packets ...Packet) ([]byte, error) {
	var payload []byte

	for _, p := range packets {
		encoded, err := encodeNonBinaryPacket(p)

		if err != nil {
			return nil, err
		}

		strEncoded := string(encoded)

		content := []byte(fmt.Sprintf("%d:%s", len(strEncoded), strEncoded))

		payload = append(payload, content...)
	}

	return payload, nil
}

func hasBinary(packets ...Packet) bool {
	for _, p := range packets {
		if _, ok := p.(*BinaryPacket); ok {
			return true
		}
	}

	return false
}

func EncodePayload(supportsBinary bool, packets ...Packet) ([]byte, error) {
	if supportsBinary && hasBinary(packets...) {
		return encodeBinaryPayload(packets...)
	}

	if len(packets) == 0 {
		return []byte("0:"), nil
	}

	return encodeNonBinaryPayload(packets...)
}

// DecodeBinaryPayload returns an array of String or Binary packets from the contents of the byte array
func DecodeBinaryPayload(payload []byte) ([]Packet, error) {
	var packets []Packet
	remaining := payload

	for {
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

		length, err := strconv.ParseInt(remaining[0:idx], 10, 32)

		if err != nil {
			return nil, err
		}

		msgStart := idx + 1
		msgEnd := msgStart + int(length)

		msg := remaining[msgStart:msgEnd]

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
