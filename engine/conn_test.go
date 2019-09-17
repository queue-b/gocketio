package engine

import "testing"

func TestDial(t *testing.T) {
	_, err := Dial("ws://localhost:8080")

	if err != nil {
		t.Errorf("Unable to dial client %v", err)
	}
}
