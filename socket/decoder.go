package socket

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
