package crawler

import (
	"testing"

	"github.com/stmcginnis/gofish/schemas"
)

func TestFirstIPAddress(t *testing.T) {
	tests := []struct {
		name string
		v4   []string
		v6   []string
		want string
	}{
		{name: "nil interface", want: ""},
		{name: "no addresses at all", want: ""},
		{name: "ipv4 preferred", v4: []string{"10.0.0.5"}, v6: []string{"fe80::1"}, want: "10.0.0.5"},
		{name: "falls back to ipv6", v6: []string{"fe80::1"}, want: "fe80::1"},
		// A BMC that reports its management NIC as IPv4 plus the IPv6
		// unspecified placeholder must still yield the IPv4 address.
		{name: "ipv6 placeholder ignored", v4: []string{"172.24.0.3"}, v6: []string{"::"}, want: "172.24.0.3"},
		{name: "only ipv6 placeholder", v6: []string{"::"}, want: ""},
		{name: "only ipv4 placeholder", v4: []string{"0.0.0.0"}, want: ""},
		{name: "blank entries skipped", v4: []string{"", "  ", "10.0.0.9"}, want: "10.0.0.9"},
		{name: "unparseable entries skipped", v4: []string{"not-an-ip", "10.0.0.9"}, want: "10.0.0.9"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "nil interface" {
				if got := firstIPAddress(nil); got != tt.want {
					t.Fatalf("firstIPAddress(nil) = %q, want %q", got, tt.want)
				}
				return
			}

			iface := &schemas.EthernetInterface{}
			for _, a := range tt.v4 {
				iface.IPv4Addresses = append(iface.IPv4Addresses, schemas.IPv4Address{Address: a})
			}
			for _, a := range tt.v6 {
				iface.IPv6Addresses = append(iface.IPv6Addresses, schemas.IPv6Address{Address: a})
			}

			if got := firstIPAddress(iface); got != tt.want {
				t.Fatalf("firstIPAddress() = %q, want %q", got, tt.want)
			}
		})
	}
}
