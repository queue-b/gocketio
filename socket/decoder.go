package socket

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/queue-b/gocketio/engine/transport"
)

// ErrWaitingForMorePackets is returned when the Decoder has not yet received enough
// BinaryPackets to fully decode a binary message
var ErrWaitingForMorePackets = errors.New("Waiting for more packets")

// ErrUnexpectedType is returned when the decoder is trying to replace placeholders with []byte
// and encounters an unexpected type
var ErrUnexpectedType = errors.New("Unexpected type")

// BinaryDecoder reconstructs BinaryEvents and BinaryAcks from multiple EnginePackets
type BinaryDecoder struct {
	message *Packet
	buffers [][]byte
}

// Reset clears the current state of the BinaryDecoder
func (d *BinaryDecoder) Reset() {
	d.buffers = [][]byte{}
	d.message = nil
}

// Decode returns either a Message, or ErrWaitingForMorePackets if additional Packets
// are required to fully reconstruct a BinaryEvent or BinaryAck
func (d *BinaryDecoder) Decode(packet transport.Packet) (Packet, error) {
	switch p := packet.(type) {
	case *transport.BinaryPacket:
		if p.Data != nil {
			d.buffers = append(d.buffers, p.Data)
		}
	case *transport.StringPacket:
		if p.Data != nil {
			d.Reset()
			message, err := decodeMessage(*p.Data)

			if err != nil {
				return Packet{}, err
			}

			d.message = message
		}
	}

	if d.message == nil {
		return Packet{}, errors.New("No message available")
	}

	// The decoder is not reconstructing a BinaryEvent or BinaryAck; return the current message
	if d.message.AttachmentCount == 0 {
		m := *d.message

		d.Reset()
		return m, nil
	}

	if len(d.buffers) == d.message.AttachmentCount {
		m := *d.message
		b := d.buffers

		d.Reset()

		replaced, err := replacePlaceholdersWithByteSlices(m.Data, b)

		if err != nil {
			return Packet{}, err
		}

		m.Data = replaced

		return m, nil
	}

	return Packet{}, ErrWaitingForMorePackets
}

func replacePlaceholdersWithByteSlices(data interface{}, buffers [][]byte) (interface{}, error) {
	// Handle JSON types:
	// object --> map[string]interface{}
	// number --> float64
	// string --> string
	// array --> []interface{}
	// boolean --> bool
	// After deserializing a JSON object to an interface, all non-primitive, non-array values
	// will be map[string]inteface{}. Those might be placeholders, or might be something else
	switch d := data.(type) {
	case map[string]interface{}:
		if len(d) == 2 {
			_, hasPlaceholder := d["_placeholder"]
			// TODO: Check attachment index is less than length, and return error
			attachmentIndex, hasAttachmentIndex := d["num"]

			if hasPlaceholder && hasAttachmentIndex {
				return buffers[int(attachmentIndex.(float64))], nil
			}
		}

		for k, v := range d {
			updated, err := replacePlaceholdersWithByteSlices(v, buffers)

			if err != nil {
				return nil, err
			}

			d[k] = updated
		}

		return d, nil
	case float64:
		return d, nil
	case bool:
		return d, nil
	case string:
		return d, nil
	case []interface{}:
		replaced := d[:0]
		for _, val := range d {
			updated, err := replacePlaceholdersWithByteSlices(val, buffers)

			if err != nil {
				return nil, err
			}

			replaced = append(replaced, updated)
		}
		return replaced, nil
	}

	return nil, ErrUnexpectedType
}

func decodeMessage(message string) (*Packet, error) {
	// Make sure the message received was at least 1 character (just the type)
	if len(message) < 1 {
		return nil, errors.New("Message too short")
	}

	decoded := &Packet{}

	t, err := strconv.ParseInt(string(message[0]), 10, 64)

	if err != nil {
		return nil, err
	}

	decoded.Type = PacketType(t)

	// A Message is valid if it only includes a type
	if len(message) == 1 {
		return decoded, nil
	}

	remaining := message[1:]

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

	// Socket.IO packets originating from socketio-client are array encoded
	// but this isn't done in the socketio-parser library; it's done in the
	// emit/send methods of socketio-client
	namespaceEnd := strings.Index(remaining, ",")

	hasNamespace := []rune(remaining)[0] == '/' && namespaceEnd > -1

	if hasNamespace {
		decoded.Namespace = remaining[:namespaceEnd]

		if namespaceEnd == len(remaining)-1 {
			return decoded, nil
		}

		remaining = remaining[namespaceEnd+1:]
	}

	// TODO: Don't assume UTF-8
	var idBytes []rune

	for _, v := range remaining {
		_, err := strconv.ParseInt(string(v), 10, 64)

		if err != nil {
			break
		}

		idBytes = append(idBytes, v)
	}

	if len(idBytes) > 0 {
		id, err := strconv.ParseInt(string(idBytes), 10, 64)

		if err == nil {
			decoded.ID = &id
		}
	}

	remaining = remaining[len(idBytes):]
	err = json.Unmarshal([]byte(remaining), &decoded.Data)

	if err != nil {
		return nil, err
	}

	return decoded, nil
}

// IsRootNamespace returns true if the namespace is / or "", false otherwise
func IsRootNamespace(namespace string) bool {
	if namespace == "" || namespace == "/" {
		return true
	}

	return false
}
