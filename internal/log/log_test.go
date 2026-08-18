package log

import (
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestLogLevel(t *testing.T) {
	var level LogLevel
	for _, want := range Levels {
		require.NoError(t, level.Set(want.String()))
		require.Equal(t, want, level)
		require.Equal(t, "LogLevel", level.Type())
		_, err := strToLogLevel(want)
		require.NoError(t, err)
	}
	require.Error(t, level.Set("invalid"))
	_, err := strToLogLevel(LogLevel("invalid"))
	require.Error(t, err)
	require.Equal(t, zerolog.Disabled, func() zerolog.Level { got, _ := strToLogLevel(DISABLED); return got }())
}

func TestInitWithLogLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "magellan.log")
	require.NoError(t, InitWithLogLevel(DEBUG, path))
	require.NotNil(t, LogFile)
	require.NoError(t, LogFile.Close())
	require.Error(t, InitWithLogLevel(LogLevel("invalid"), ""))
}
