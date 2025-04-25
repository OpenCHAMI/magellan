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

func LoadCertificateFromPath(client Client, path string) error {
	cacert, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read certificate at path: %s", path)
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(cacert)
	err = LoadCertificateFromPool(client, certPool)
	if err != nil {
		return fmt.Errorf("could not initialize certificate from pool: %v", err)
	}
	return nil
}

func LoadCertificateFromPool(client Client, certPool *x509.CertPool) error {
	// make sure we have a valid cert pool
	if certPool == nil {
		return fmt.Errorf("invalid cert pool")
	}

	// make sure that we can access the internal client
	internalClient := client.GetInternalClient()
	if internalClient == nil {
		return fmt.Errorf("invalid HTTP client")
	}
	internalClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:            certPool,
			InsecureSkipVerify: false,
		},
		DisableKeepAlives: true,
		Dial: (&net.Dialer{
			Timeout:   120 * time.Second,
			KeepAlive: 120 * time.Second,
		}).Dial,
		TLSHandshakeTimeout:   120 * time.Second,
		ResponseHeaderTimeout: 120 * time.Second,
	}
	return nil
}

// Post() is a simplified wrapper function that packages all of the
// that marshals a mapper into a JSON-formatted byte array, and then performs
// a request to the specified URL.
func (c *DefaultClient) Post(url string, data map[string]any, header HTTPHeader) (*http.Response, HTTPBody, error) {
	// serialize data into byte array
	body, err := json.Marshal(data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal data for request: %v", err)
	}
	return MakeRequest(c.Client, url, http.MethodPost, body, header)
}
