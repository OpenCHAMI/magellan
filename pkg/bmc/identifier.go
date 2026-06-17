package bmc

import (
	"net"
	"regexp"
	"strings"
)

// IdentifierType represents the type of node identifier provided by the user
type IdentifierType int

const (
	IdentifierUnknown IdentifierType = iota
	IdentifierXName                  // x1000c0s0b3n0
	IdentifierIP                     // 10.0.0.101 or 2001:db8::1
	IdentifierUUID                   // 3894755a-8e4c-41d6-a6eb-3c5f4b7d2e10
	IdentifierSerial                 // CN75120A3G
	IdentifierMAC                    // aa:bb:cc:dd:ee:ff or aa-bb-cc-dd-ee-ff
)

// String returns the human-readable name of the identifier type
func (i IdentifierType) String() string {
	return [...]string{"Unknown", "XName", "IP", "UUID", "Serial", "MAC"}[i]
}

// DetectIdentifierType determines what type of identifier the input string represents.
// It uses pattern matching to auto-detect the identifier type, enabling flexible
// node targeting without requiring explicit type flags.
//
// Detection order:
//  1. IP address (IPv4/IPv6)
//  2. UUID (RFC 4122 format)
//  3. MAC address (colon or dash separated)
//  4. XName (Cray format: x<cabinet>c<chassis>s<slot>b<bmc>[n<node>])
//  5. Serial number (alphanumeric fallback)
//
// Examples:
//   - "10.0.0.101" → IdentifierIP
//   - "3894755a-8e4c-41d6-a6eb-3c5f4b7d2e10" → IdentifierUUID
//   - "aa:bb:cc:dd:ee:ff" → IdentifierMAC
//   - "x1000c0s0b3n0" → IdentifierXName
//   - "CN75120A3G" → IdentifierSerial
func DetectIdentifierType(input string) IdentifierType {
	if input == "" {
		return IdentifierUnknown
	}

	// IP: Try parsing as IP address (IPv4 or IPv6)
	if net.ParseIP(input) != nil {
		return IdentifierIP
	}

	// UUID: RFC 4122 format (8-4-4-4-12 hex digits)
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	if uuidRegex.MatchString(strings.ToLower(input)) {
		return IdentifierUUID
	}

	// MAC: 6 pairs of hex digits with : or - separator
	macRegex := regexp.MustCompile(`^([0-9a-f]{2}[:-]){5}[0-9a-f]{2}$`)
	if macRegex.MatchString(strings.ToLower(input)) {
		return IdentifierMAC
	}

	// XName: Cray format - x<cabinet>c<chassis>s<slot>b<bmc>[n<node>]
	// Examples: x1000c0s0b3n0, x5506c0s172b105n1, x1c2s3b4
	xnameRegex := regexp.MustCompile(`^x\d+c\d+s\d+b\d+(n\d+)?$`)
	if xnameRegex.MatchString(strings.ToLower(input)) {
		return IdentifierXName
	}

	// Serial: Fallback for any other alphanumeric string
	serialRegex := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	if serialRegex.MatchString(input) {
		return IdentifierSerial
	}

	return IdentifierUnknown
}
