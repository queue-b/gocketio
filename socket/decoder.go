package socket

import (
	"errors"

	"github.com/queue-b/gocket/transport"
)

// ErrNeedMoreAttachments
var ErrNeedMoreAttachments = errors.New("Waiting for additional attachments")

type Decoder struct {
	attachmentCount int
	tempData        interface{}
	buffers         [][]byte
}

func (d *Decoder) Reset() {
	d.attachmentCount = 0
	d.buffers = nil
	d.tempData = nil
}

func (d *Decoder) Decode(packet transport.EnginePacket) (interface{}, error) {
	switch p := packet.(type) {
	case *transport.BinaryPacket:
		if p.Data != nil {
			d.buffers = append(d.buffers, p.Data)
		}
	case *transport.StringPacket:
		if p.Data != nil {
			d.Reset()
			message, err := DecodeMessage(*p.Data)

			if err != nil {
				return nil, err
			}

			d.tempData = message.Data
		}
	}

	if d.attachmentCount == 0 {
		oldData := d.tempData

		d.Reset()
		return oldData, nil
	}

	if d.buffers != nil && len(d.buffers) == d.attachmentCount {
		oldData := d.tempData
		oldBuffers := d.buffers

		d.Reset()

		return replacePlaceholdersWithByteSlices(oldData, oldBuffers), nil
	}

	return nil, ErrNeedMoreAttachments
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
