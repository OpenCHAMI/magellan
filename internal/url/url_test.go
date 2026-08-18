package url

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeAndTrimScheme(t *testing.T) {
	got, err := Sanitize("https://example.com//redfish/v1//")
	require.NoError(t, err)
	require.Equal(t, "https://example.com/redfish/v1", got)
	_, err = Sanitize("://bad")
	require.Error(t, err)
	require.Equal(t, "example.com", TrimScheme("https://example.com"))
	require.Equal(t, "http://example.com", TrimScheme("http://example.com"))
}

func TestFormatHosts(t *testing.T) {
	got := FormatHosts([]string{"node.example", "https://node.example:8443/", "://bad"}, []int{443, 5000}, "https")
	require.Equal(t, [][]string{
		{"https://node.example:443", "https://node.example:5000"},
		{"https://node.example:8443"},
	}, got)

	got = FormatHosts([]string{"node.example"}, []int{443}, "")
	require.Equal(t, "https://node.example:443", got[0][0])
}

func TestFormatIPs(t *testing.T) {
	got := FormatIPs([]string{"192.0.2.1"}, nil, "", false)
	require.Equal(t, [][]string{{"https://192.0.2.1:443"}}, got)
	got = FormatIPs([]string{"2001:db8::1"}, []int{8443}, "http", false)
	require.Equal(t, [][]string{{"http://[2001:db8::1]:8443"}}, got)
}
