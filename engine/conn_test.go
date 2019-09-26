package engine

import (
	"testing"
)

func TestFixupAddress(t *testing.T) {
	address := "http://localhost:3030"

	parsedAddress, err := fixupAddress(address)

	if err != nil {
		t.Fatalf("Unable to fixup %v %v\n", address, err)
	}

	if parsedAddress.String() != "ws://localhost:3030/engine.io/" {
		t.Fatalf("Expected %v got %v", "ws://localhost:3030/engine.io/", parsedAddress.String())
	}

	address = "https://localhost:3030/socket.io/"

	parsedAddress, err = fixupAddress(address)

	if err != nil {
		t.Fatalf("Unable to fixup %v %v\n", address, err)
	}

	if parsedAddress.String() != "wss://localhost:3030/socket.io/" {
		t.Fatalf("Expected %v got %v", "wss://localhost:3030/socket.io/", parsedAddress.String())
	}

}
