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

func TestBuildTimeDefaultsToUnknown(t *testing.T) {
	require.Equal(t, "unknown", BuildTime)
}

func TestVersionInfoTableAlignsValues(t *testing.T) {
	require.Equal(t, `Version:    unknown
Git Commit: unknown
Build Time: unknown
Git Branch: unknown
Git Tag:    unknown
Git State:  unknown
Build Host: unknown
Go Version: unknown
Build User: unknown
`, versionInfoTable())
}
