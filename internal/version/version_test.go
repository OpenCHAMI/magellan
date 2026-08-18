package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionInfo(t *testing.T) {
	oldVersion, oldCommit := Version, GitCommit
	t.Cleanup(func() { Version, GitCommit = oldVersion, oldCommit })
	Version, GitCommit = "v1.2.3", "abc123"
	got := VersionInfo()
	require.Contains(t, got, "v1.2.3")
	require.Contains(t, got, "abc123")
}
