package socket

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/queue-b/gocket/engine"
)

// ErrNeedMoreAttachments
var ErrNeedMoreAttachments = errors.New("Waiting for additional attachments")

type Decoder struct {
	message *Message
	buffers [][]byte
}

func (d *Decoder) Reset() {
	d.buffers = nil
	d.message = nil
}

func (d *Decoder) Decode(packet engine.EnginePacket) (Message, error) {
	switch p := packet.(type) {
	case *engine.BinaryPacket:
		if p.Data != nil {
			d.buffers = append(d.buffers, p.Data)
		}
	case *engine.StringPacket:
		if p.Data != nil {
			d.Reset()
			message, err := decodeMessage(*p.Data)

			if err != nil {
				return Message{}, err
			}

			d.message = message
		}
	}

	if d.message == nil {
		return Message{}, errors.New("No message available")
	}

	if d.message.AttachmentCount == 0 {
		m := *d.message

		d.Reset()
		return m, nil
	}

	if d.buffers != nil && len(d.buffers) == d.message.AttachmentCount {
		m := *d.message
		b := d.buffers

		d.Reset()

		m.Data = replacePlaceholdersWithByteSlices(m.Data, b)

		return m, nil
	}

	return Message{}, ErrNeedMoreAttachments
}

func replacePlaceholdersWithByteSlices(data interface{}, buffers [][]byte) interface{} {
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
				return buffers[int(attachmentIndex.(float64))]
			}
		}

		for k, v := range d {
			d[k] = replacePlaceholdersWithByteSlices(v, buffers)
		}
	case float64:
		return d
	case bool:
		return d
	case string:
		return d
	case []interface{}:
		replaced := d[:0]
		for _, val := range d {
			replaced = append(replaced, replacePlaceholdersWithByteSlices(val, buffers))
		}
		return replaced
	default:
		return nil
	}

	return nil
}

func decodeMessage(message string) (*Message, error) {
	// Make sure the message received was at least 1 character (just the type)
	if len(message) < 1 {
		return nil, errors.New("Message too short")
	}

	decoded := &Message{}

	t, err := strconv.ParseInt(string(message[0]), 10, 64)

	if err != nil {
		return nil, err
	}

	decoded.Type = MessageType(t)

	fmt.Printf("Type %v\n", decoded.Type)

	// A Message is valid if it only includes a type
	if len(message) == 1 {
		return decoded, nil
	}

	remaining := message[1:]
	fmt.Printf("Searching %v\n", remaining)

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

	fmt.Printf("Searching %v\n", remaining)

	ni := strings.Index(remaining, ",")
	fmt.Printf("Found comma at %v\n", ni)

	if ni != -1 {
		decoded.Namespace = remaining[:ni]
		fmt.Printf("Namespace %v", decoded.Namespace)

		if ni < len(remaining)-1 {
			remaining = remaining[ni+1:]
		}
	}

	fmt.Printf("Searching %v\n", remaining)

	// TODO: Don't assume UTF-8
	var idBytes []rune

	for _, v := range remaining {
		_, err := strconv.ParseInt(string(v), 10, 64)

		if err != nil {
			break
		}

		idBytes = append(idBytes, v)
	}

	id, err := strconv.ParseInt(string(idBytes), 10, 64)

	if err == nil {
		usableID := int(id)
		decoded.ID = &usableID
	}

	remaining = remaining[len(idBytes):]

	err = json.Unmarshal([]byte(remaining), &decoded.Data)

	if err != nil {
		return nil, err
	}

	return decoded, nil
}
