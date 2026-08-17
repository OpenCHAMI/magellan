package power

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenCHAMI/magellan/internal/format"
	"github.com/stretchr/testify/require"
)

func TestParseInventory(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name, contents string
		dataFormat     format.DataFormat
	}{
		{"json", `[{"ID":"x0c0s0b0","FQDN":"bmc.example","Systems":[{"node_id":"Node0"}] }]`, format.FORMAT_JSON},
		{"yaml", "- ID: x0c0s0b0\n  FQDN: bmc.example\n  Systems:\n    - nodeid: Node0\n", format.FORMAT_YAML},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.name)
			require.NoError(t, os.WriteFile(path, []byte(tt.contents), 0o600))
			nodes, err := ParseInventory(path, tt.dataFormat)
			require.NoError(t, err)
			require.Equal(t, "x0c0s0b0n0", nodes[0].ClusterID)
			require.Equal(t, "bmc.example", nodes[0].BmcIP)
			require.Equal(t, "Node0", nodes[0].NodeID)
		})
	}
	_, err := ParseInventory(filepath.Join(dir, "missing"), format.FORMAT_JSON)
	require.Error(t, err)
	bad := filepath.Join(dir, "bad")
	require.NoError(t, os.WriteFile(bad, []byte("{"), 0o600))
	_, err = ParseInventory(bad, format.FORMAT_JSON)
	require.Error(t, err)
	_, err = ParseInventory(bad, format.FORMAT_LIST)
	require.Error(t, err)
}
