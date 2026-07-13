package cmd

import (
	"testing"

	"github.com/OpenCHAMI/magellan/internal/format"
)

func TestScanFormatDefaultsToJSON(t *testing.T) {
	formatFlag := ScanCmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Fatal("format flag is not registered")
	}

	if formatFlag.DefValue != string(format.FORMAT_JSON) {
		t.Fatalf("format flag default = %q, want %q", formatFlag.DefValue, format.FORMAT_JSON)
	}
}
