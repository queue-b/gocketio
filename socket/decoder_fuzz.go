// +build gofuzz

package socket

func Fuzz(data []byte) int {
	p, err := decodeMessage(string(data))

	if err != nil {
		if p != nil {
			panic("Packet != nil on error")
		}

		return 0
	}

	return 1
}
