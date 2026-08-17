package jaws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCrawlPDU(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "user", user)
		require.Equal(t, "pass", pass)
		require.Equal(t, "/jaws/monitor/outlets", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":"BA35","name":"Outlet","state":"ON","socket_type":"C13"}]`))
	}))
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "https://")

	got, err := CrawlPDU(CrawlerConfig{URI: host, Username: "user", Password: "pass", Insecure: true, Timeout: time.Second})
	require.NoError(t, err)
	require.Equal(t, host, got.Hostname)
	require.Len(t, got.Outlets, 1)
	require.Equal(t, "BA35", got.Outlets[0].ID)
	require.Equal(t, "ON", got.Outlets[0].PowerState)
}

func TestCrawlPDUErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"status", `{}`, http.StatusUnauthorized},
		{"json", `{`, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			_, err := CrawlPDU(CrawlerConfig{URI: strings.TrimPrefix(server.URL, "https://"), Insecure: true, Timeout: time.Second})
			require.Error(t, err)
		})
	}
	_, err := CrawlPDU(CrawlerConfig{URI: "127.0.0.1:1", Insecure: true, Timeout: time.Millisecond})
	require.Error(t, err)
}
