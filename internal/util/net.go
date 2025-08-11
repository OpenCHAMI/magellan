package util

import (
	"fmt"
	"net"
)

func IPAddrStrToInt(ipStr string)(int, error) {
	// Generate an integer from an IP address. This is not
	// sensitive to byte ordering, so the integer produced on
	// different systems may be different. It will be consistent
	// for any specific architecture.
	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return 0, fmt.Errorf("cannot convert invalid IPv4 address string '%s' to integer", ipStr)
	}
	return (int(ip[0]) * (1 << 24)) + (int(ip[1]) * (1 << 16)) + (int(ip[2]) * (1 << 8)) + int(ip[3]), nil
}
