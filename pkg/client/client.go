package client

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/OpenCHAMI/magellan/internal/util"
)

type Option[T Client] func(client T)

// The 'Client' struct is a wrapper around the default http.Client
// that provides an extended API to work with functional options.
// It also provides functions that work with `collect` data.
type Client interface {
	Name() string
	GetClient() *http.Client
	RootEndpoint(endpoint string) string

	// functions needed to make request
	Add(data util.HTTPBody, headers util.HTTPHeader) error
	Update(data util.HTTPBody, headers util.HTTPHeader) error
}

// NewClient() creates a new client
func NewClient[T Client](opts ...func(T)) T {
	client := new(T)
	for _, opt := range opts {
		opt(*client)
	}
	return *client
}

func WithCertPool[T Client](certPool *x509.CertPool) func(T) {
	if certPool == nil {
		return func(client T) {}
	}
	return func(client T) {
		client.GetClient().Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            certPool,
				InsecureSkipVerify: true,
			},
			DisableKeepAlives: true,
			Dial: (&net.Dialer{
				Timeout:   120 * time.Second,
				KeepAlive: 120 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   120 * time.Second,
			ResponseHeaderTimeout: 120 * time.Second,
		}
	}
}

func WithSecureTLS[T Client](certPath string) func(T) {
	cacert, err := os.ReadFile(certPath)
	if err != nil {
		return func(client T) {}
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(cacert)
	return WithCertPool[T](certPool)
}

// Post() is a simplified wrapper function that packages all of the
// that marshals a mapper into a JSON-formatted byte array, and then performs
// a request to the specified URL.
func (c *MagellanClient) Post(url string, data map[string]any, header util.HTTPHeader) (*http.Response, util.HTTPBody, error) {
	// serialize data into byte array
	body, err := json.Marshal(data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal data for request: %v", err)
	}
	return util.MakeRequest(c.Client, url, http.MethodPost, body, header)
}
