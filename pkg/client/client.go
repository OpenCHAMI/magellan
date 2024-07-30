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

type Option func(*Client)

// The 'Client' struct is a wrapper around the default http.Client
// that provides an extended API to work with functional options.
// It also provides functions that work with `collect` data.
type Client struct {
	*http.Client
}

// NewClient() creates a new client
func NewClient(opts ...Option) *Client {
	client := &Client{
		Client: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

func WithHttpClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.Client = httpClient
	}
}

func WithCertPool(certPool *x509.CertPool) Option {
	if certPool == nil {
		return func(c *Client) {}
	}
	return func(c *Client) {
		c.Client.Transport = &http.Transport{
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

func WithSecureTLS(certPath string) Option {
	cacert, err := os.ReadFile(certPath)
	if err != nil {
		return func(c *Client) {}
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(cacert)
	return WithCertPool(certPool)
}

// Post() is a simplified wrapper function that packages all of the
// that marshals a mapper into a JSON-formatted byte array, and then performs
// a request to the specified URL.
func (c *Client) Post(url string, data map[string]any, header util.HTTPHeader) (*http.Response, util.HTTPBody, error) {
	// serialize data into byte array
	body, err := json.Marshal(data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal data for request: %v", err)
	}
	return util.MakeRequest(c.Client, url, http.MethodPost, body, header)
}

func (c *Client) MakeRequest(url string, method string, body util.HTTPBody, header util.HTTPHeader) (*http.Response, util.HTTPBody, error) {
	return util.MakeRequest(c.Client, url, method, body, header)
}
