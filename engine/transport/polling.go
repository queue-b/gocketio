package transport

import (
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

type Transport interface {
	Read() ([]byte, bool, error)
	Write([]byte, bool) error
	Close() error
}

// PollingConfig provides a way to configure the Polling transport
type PollingConfig struct {
	// Timeout specifies a time limit for requests made with the Polling transport
	Timeout time.Duration
	// TimestampRequests specifies whether to timestamp requests made with the Polling transport
	// This is used primarily as a cache-busting technique
	TimestampRequests bool
	// TimestampParamName specifies the name of the timestamp parameter
	TimestampParamName string
	// AdditionalHeaders specifies any additional headers that will be added to the HTTP requests
	// made by the Polling transport
	AdditionalHeaders http.Header
	// Client specifies the http.Client used by the Polling transport when making HTTP requests
	// This can be used to customize things like TLS, redirect handling, etc
	Client *http.Client
}

type Polling struct {
	sync.RWMutex
	address string
	client  *http.Client
	config  PollingConfig
}

func NewPolling(address string, config PollingConfig) *Polling {
	p := &Polling{}
	p.config = config
	p.address = address

	if config.Client != nil {
		p.client = config.Client
	} else {
		p.client = &http.Client{
			Timeout: config.Timeout,
		}
	}

	return p
}

func (p *Polling) Read() ([]byte, bool, error) {
	request, err := http.NewRequest("GET", "", nil)

	if err != nil {
		return nil, false, err
	}

	response, err := p.client.Do(request)

	if err != nil {
		return nil, false, err
	}

	body, err := ioutil.ReadAll(response.Body)

	if err != nil {
		return nil, false, err
	}

	return body, response.Header.Get("Content-Type") == "application/octet-stream", nil
}

func (p *Polling) Close() error {
	return nil
}

func (p *Polling) Write(packet []byte, binary bool) error {
	request, err := http.NewRequest("POST", "", nil)

	if err != nil {
		return err
	}

	_, err = p.client.Do(request)

	return err
}
