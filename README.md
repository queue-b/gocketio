# gocketio

Go (golang) socket.io client

Supports

- Non-binary Events
- Binary Events
- Non-binary Acks
- Binary Acks
- Automatic reconnection with exponential backoff
- Multiplexing multiple namespaces over a single connection
- Websocket transport

## Installation

```sh
go get github.com/queue-b/gocketio
```

## Usage

Additional examples are available as godocs.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/queue-b/gocketio"
)

func main() {
	// Connect to a socket.io server at example.com
	manager, err := gocketio.DialContext(context.Background(), "https://example.com/", gocketio.DefaultManagerConfig())

	if err != nil {
		log.Fatal(err)
	}

	// Create a socket for the root namespace
	socket, err := manager.Namespace("/")

	if err != nil {
		log.Fatal(err)
	}

	c := make(chan struct{}, 1)

	// Add a handler for "hello" events
	socket.On("hello", func(from string) {
		fmt.Printf("Hello from %v\n", from)
		c <- struct{}{}
	})

	// Block until someone sends a hello message
	<-c
}
```

## License

Apache

## Authors

[https://github.com/queue-b](Alex Kube)
