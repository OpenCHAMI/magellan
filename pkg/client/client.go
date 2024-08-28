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

	"github.com/rs/zerolog/log"
)

type Option[T Client] func(client *T)

// The 'Client' struct is a wrapper around the default http.Client
// that provides an extended API to work with functional options.
// It also provides functions that work with `collect` data.
type Client interface {
	Init()
	Name() string
	RootEndpoint(endpoint string) string
	GetInternalClient() *http.Client

	// functions needed to make request
	Add(data HTTPBody, headers HTTPHeader) error
	Update(data HTTPBody, headers HTTPHeader) error
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
	// make sure we have a valid cert pool
	if certPool == nil {
		return func(client T) {}
	}
	return func(client T) {
		// make sure that we can access the internal client
		if client.GetInternalClient() == nil {
			log.Warn().Any("client", client.GetInternalClient()).Msg("invalid internal HTTP client ()")
			return
		}
		client.GetInternalClient().Transport = &http.Transport{
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
func (c *MagellanClient) Post(url string, data map[string]any, header HTTPHeader) (*http.Response, HTTPBody, error) {
	// serialize data into byte array
	body, err := json.Marshal(data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal data for request: %v", err)
	}
	return MakeRequest(c.Client, url, http.MethodPost, body, header)
}
