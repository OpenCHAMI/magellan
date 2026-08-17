package client

import (
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (errorReader) Close() error             { return nil }

type testClient struct{ internal *http.Client }

func (c *testClient) Init()                               {}
func (c *testClient) Name() string                        { return "test" }
func (c *testClient) RootEndpoint(endpoint string) string { return endpoint }
func (c *testClient) GetInternalClient() *http.Client     { return c.internal }
func (c *testClient) Add(HTTPBody, HTTPHeader) error      { return nil }
func (c *testClient) Update(HTTPBody, HTTPHeader) error   { return nil }

func TestHTTPHeader(t *testing.T) {
	h := HTTPHeader{}
	h.Authorization("")
	require.NotContains(t, h, "Authorization")
	h.Authorization("token").ContentType("application/json")
	require.Equal(t, "Bearer token", h["Authorization"])
	require.Equal(t, "application/json", h["Content-Type"])
}

func TestGetNextIP(t *testing.T) {
	ip := net.ParseIP("192.0.2.255")
	require.Equal(t, "192.0.3.0", GetNextIP(&ip, 1).String())
	require.Empty(t, *GetNextIP(nil, 1))
	invalid := net.IP{1, 2}
	require.Empty(t, *GetNextIP(&invalid, 1))
}

func TestMakeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "magellan", r.Header.Get("User-Agent"))
		require.Equal(t, "value", r.Header.Get("X-Test"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"hello":"world"}`, string(body))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("response"))
	}))
	defer server.Close()

	res, body, err := MakeRequest(server.Client(), server.URL, http.MethodPost, []byte(`{"hello":"world"}`), HTTPHeader{"X-Test": "value"})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, res.StatusCode)
	require.Equal(t, "response", string(body))

	_, _, err = MakeRequest(nil, "://bad", http.MethodGet, nil, nil)
	require.Error(t, err)
	badClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("transport failed")
	})}
	_, _, err = MakeRequest(badClient, "http://example.com", http.MethodGet, nil, nil)
	require.Error(t, err)
	readClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: errorReader{}, Header: make(http.Header)}, nil
	})}
	_, _, err = MakeRequest(readClient, "http://example.com", http.MethodGet, nil, nil)
	require.Error(t, err)
}

func TestCertificates(t *testing.T) {
	pool := x509.NewCertPool()
	c := &testClient{internal: &http.Client{}}
	require.NoError(t, LoadCertificateFromPool(c, pool))
	transport, ok := c.internal.Transport.(*http.Transport)
	require.True(t, ok)
	require.Same(t, pool, transport.TLSClientConfig.RootCAs)
	require.Error(t, LoadCertificateFromPool(c, nil))
	require.Error(t, LoadCertificateFromPool(&testClient{}, pool))

	missing := filepath.Join(t.TempDir(), "missing.pem")
	require.Error(t, LoadCertificateFromPath(c, missing))
	path := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(path, []byte("not a certificate"), 0o600))
	require.Error(t, LoadCertificateFromPath(c, path))
}

func TestSmdClient(t *testing.T) {
	var method, path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := NewSmdClient()
	c.URI = server.URL
	require.Equal(t, "smd", c.Name())
	require.Equal(t, server.URL+"/hsm/v2/test", c.RootEndpoint("/test"))
	require.NotNil(t, c.GetInternalClient())
	require.Error(t, c.Add(nil, nil))
	require.NoError(t, c.Add([]byte(`{}`), nil))
	require.Equal(t, http.MethodPost, method)
	require.Equal(t, "/hsm/v2/Inventory/RedfishEndpoints", path)
	c.Xname = "x0c0s0b0"
	require.NoError(t, c.Update([]byte(`{}`), nil))
	require.Equal(t, http.MethodPut, method)
	require.Contains(t, path, c.Xname)

	require.NoError(t, c.SetXnameFromJSON([]byte(`{"ID":"x1"}`), "ID"))
	require.Equal(t, "x1", c.Xname)
	require.Error(t, c.SetXnameFromJSON([]byte(`{`), "ID"))
	require.Error(t, c.SetXnameFromJSON([]byte(`{}`), "ID"))
	require.Error(t, c.SetXnameFromJSON([]byte(`{"ID":1}`), "ID"))
}

func TestDefaultClientBasics(t *testing.T) {
	c := &DefaultClient{Client: &http.Client{}}
	require.Equal(t, "default", c.Name())
	require.Error(t, c.Add(nil, nil))
	require.Error(t, c.Update(nil, nil))
}
