package client

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/rs/zerolog/log"
)

// HTTP aliases for readibility
type HTTPHeader map[string]string
type HTTPBody []byte

func (h HTTPHeader) Authorization(accessToken string) HTTPHeader {
	if accessToken != "" {
		h["Authorization"] = fmt.Sprintf("Bearer %s", accessToken)
	}
	return h
}

func (h HTTPHeader) ContentType(contentType string) HTTPHeader {
	h["Content-Type"] = contentType
	return h
}

// GetNextIP() returns the next IP address, but does not account
// for net masks.
func GetNextIP(ip *net.IP, inc uint) *net.IP {
	if ip == nil {
		return &net.IP{}
	}
	i := ip.To4()
	v := uint(i[0])<<24 + uint(i[1])<<16 + uint(i[2])<<8 + uint(i[3])
	v += inc
	v3 := byte(v & 0xFF)
	v2 := byte((v >> 8) & 0xFF)
	v1 := byte((v >> 16) & 0xFF)
	v0 := byte((v >> 24) & 0xFF)
	// return &net.IP{[]byte{v0, v1, v2, v3}}
	r := net.IPv4(v0, v1, v2, v3)
	return &r
}

// MakeRequest() is a wrapper function that condenses simple HTTP
// requests done to a single call. It expects an optional HTTP client,
// URL, HTTP method, request body, and request headers. This function
// is useful when making many requests where only these few arguments
// are changing.
//
// Returns a HTTP response object, response body as byte array, and any
// error that may have occurred with making the request.
func MakeRequest(client *http.Client, url string, httpMethod string, body HTTPBody, header HTTPHeader) (*http.Response, HTTPBody, error) {
	// use defaults if no client provided
	if client == nil {
		client = http.DefaultClient
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	req, err := http.NewRequest(httpMethod, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create new HTTP request: %v", err)
	}
	req.Header.Add("User-Agent", "magellan")
	for k, v := range header {
		req.Header.Add(k, v)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make request: %v", err)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if err := res.Body.Close(); err != nil {
		log.Warn().Err(err).Msg("could not close response resource")
	}
	return res, b, err
}
