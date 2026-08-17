package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestLoadAccessTokenPrecedence(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("ACCESS_TOKEN", "environment")
	path := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(path, []byte("file"), 0o600))
	viper.Set("access-token", "config")
	got, err := LoadAccessToken(path)
	require.NoError(t, err)
	require.Equal(t, "environment", got)

	t.Setenv("ACCESS_TOKEN", "")
	got, err = LoadAccessToken(path)
	require.NoError(t, err)
	require.Equal(t, "file", got)

	got, err = LoadAccessToken(filepath.Join(t.TempDir(), "missing"))
	require.NoError(t, err)
	require.Equal(t, "config", got)
	viper.Set("access-token", "")
	_, err = LoadAccessToken(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
}
