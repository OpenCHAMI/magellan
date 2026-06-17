package bmc

import "testing"

func TestDetectIdentifierType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected IdentifierType
	}{
		// IP addresses - IPv4
		{"IPv4 standard", "10.0.0.101", IdentifierIP},
		{"IPv4 localhost", "127.0.0.1", IdentifierIP},
		{"IPv4 broadcast", "255.255.255.255", IdentifierIP},
		{"IPv4 zero", "0.0.0.0", IdentifierIP},

		// IP addresses - IPv6
		{"IPv6 standard", "2001:db8::1", IdentifierIP},
		{"IPv6 localhost", "::1", IdentifierIP},
		{"IPv6 full", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", IdentifierIP},
		{"IPv6 compressed", "2001:db8::8a2e:370:7334", IdentifierIP},

		// UUIDs
		{"UUID lowercase", "3894755a-8e4c-41d6-a6eb-3c5f4b7d2e10", IdentifierUUID},
		{"UUID uppercase", "3894755A-8E4C-41D6-A6EB-3C5F4B7D2E10", IdentifierUUID},
		{"UUID mixed case", "3894755a-8E4c-41D6-a6eb-3c5F4b7d2e10", IdentifierUUID},
		{"UUID all zeros", "00000000-0000-0000-0000-000000000000", IdentifierUUID},
		{"UUID all fs", "ffffffff-ffff-ffff-ffff-ffffffffffff", IdentifierUUID},

		// MAC addresses
		{"MAC colon separator lowercase", "aa:bb:cc:dd:ee:ff", IdentifierMAC},
		{"MAC colon separator uppercase", "AA:BB:CC:DD:EE:FF", IdentifierMAC},
		{"MAC colon separator mixed", "Aa:Bb:Cc:Dd:Ee:Ff", IdentifierMAC},
		{"MAC dash separator lowercase", "aa-bb-cc-dd-ee-ff", IdentifierMAC},
		{"MAC dash separator uppercase", "AA-BB-CC-DD-EE-FF", IdentifierMAC},
		{"MAC all zeros", "00:00:00:00:00:00", IdentifierMAC},
		{"MAC broadcast", "ff:ff:ff:ff:ff:ff", IdentifierMAC},

		// XNames (Cray format)
		{"XName with node", "x1000c0s0b3n0", IdentifierXName},
		{"XName without node", "x1000c0s0b3", IdentifierXName},
		{"XName complex with node", "x5506c0s172b105n1", IdentifierXName},
		{"XName complex without node", "x5506c0s172b105", IdentifierXName},
		{"XName minimal", "x1c2s3b4", IdentifierXName},
		{"XName uppercase", "X1000C0S0B3N0", IdentifierXName},
		{"XName mixed case", "X1000c0S0b3N0", IdentifierXName},

		// Serial numbers
		{"Serial alphanumeric", "CN75120A3G", IdentifierSerial},
		{"Serial numeric only", "1234567890", IdentifierSerial},
		{"Serial alpha only", "ABCDEFGH", IdentifierSerial},
		{"Serial mixed case", "Cn75120a3G", IdentifierSerial},
		{"Serial long", "ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890", IdentifierSerial},

		// Edge cases
		{"Empty string", "", IdentifierUnknown},
		{"Whitespace only", "   ", IdentifierUnknown},
		{"Special characters", "x1000@c0s0", IdentifierUnknown},
		{"UUID without dashes", "3894755a8e4c41d6a6eb3c5f4b7d2e10", IdentifierSerial}, // Falls back to serial
		{"MAC without separators", "aabbccddeeff", IdentifierSerial},                  // Falls back to serial
		{"Invalid XName missing components", "x1000c0", IdentifierSerial},             // Falls back to serial
		{"Invalid XName wrong prefix", "y1000c0s0b3n0", IdentifierSerial},             // Falls back to serial
		{"Partial UUID", "3894755a-8e4c-41d6", IdentifierUnknown},
		{"Invalid MAC too short", "aa:bb:cc:dd:ee", IdentifierUnknown},
		{"Invalid MAC too long", "aa:bb:cc:dd:ee:ff:gg", IdentifierUnknown},
		{"Mixed separators MAC", "aa:bb-cc:dd-ee:ff", IdentifierMAC}, // Regex allows either separator at each position
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectIdentifierType(tt.input)
			if result != tt.expected {
				t.Errorf("DetectIdentifierType(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIdentifierTypeString(t *testing.T) {
	tests := []struct {
		identifierType IdentifierType
		expected       string
	}{
		{IdentifierUnknown, "Unknown"},
		{IdentifierXName, "XName"},
		{IdentifierIP, "IP"},
		{IdentifierUUID, "UUID"},
		{IdentifierSerial, "Serial"},
		{IdentifierMAC, "MAC"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.identifierType.String()
			if result != tt.expected {
				t.Errorf("IdentifierType.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}
