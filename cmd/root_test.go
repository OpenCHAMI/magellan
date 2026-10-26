package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	logger "github.com/openchami/magellan/internal/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// TestLogFlagsRegistered verifies that the logging flags exist as persistent
// flags with the expected defaults.
func TestLogFlagsRegistered(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("log-level")
	require.NotNil(t, f)
	require.Equal(t, "info", f.DefValue)
	f = rootCmd.PersistentFlags().Lookup("log-format")
	require.NotNil(t, f)
	require.Equal(t, "json", f.DefValue)
	f = rootCmd.PersistentFlags().Lookup("log-color")
	require.NotNil(t, f)
	require.Equal(t, "auto", f.DefValue)
}

// TestLogFlagsHaveNoShorthand guards against reintroducing a shorthand for
// the log format/color flags. pflag panics on duplicate shorthands, and -L
// is already taken by power --list-reset-types (see TestPowerCommandExecutes).
func TestLogFlagsHaveNoShorthand(t *testing.T) {
	for _, name := range []string{"log-format", "log-color", "log-file"} {
		f := rootCmd.PersistentFlags().Lookup(name)
		require.NotNil(t, f)
		require.Empty(t, f.Shorthand, "%s must not use a shorthand", name)
	}
	// log-level keeps its historical -l shorthand
	require.Equal(t, "l", rootCmd.PersistentFlags().Lookup("log-level").Shorthand)
}

func TestCompletionLogFormat(t *testing.T) {
	vals, directive := completionLogFormat(nil, nil, "")
	require.Equal(t, cobra.ShellCompDirectiveDefault, directive)
	require.Len(t, vals, len(logger.LogFormatHelpMap))
	require.Contains(t, vals, "json\tJSON format")
	require.Contains(t, vals, "rfc3339\tRFC3339 timestamped format")
	require.Contains(t, vals, "basic\tBasic format without timestamps")
}

func TestCompletionLogColor(t *testing.T) {
	vals, directive := completionLogColor(nil, nil, "")
	require.Equal(t, cobra.ShellCompDirectiveDefault, directive)
	require.Len(t, vals, len(logger.LogColorHelpMap))
	require.Contains(t, vals, "auto\tColor when stderr is a terminal")
	require.Contains(t, vals, "on\tAlways colorize")
	require.Contains(t, vals, "off\tNever colorize")
}

// resetRootLoggerState restores the package-level logger state mutated by
// InitializeConfig so tests don't leak into each other.
func resetRootLoggerState(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		viper.Reset()
		logLevel = logger.INFO
		logFormat = logger.FORMAT_JSON
		logColor = logger.COLOR_AUTO
		configPath = ""
		logFile = ""
		configDiagnostics = nil
	})
	viper.Reset()
	configDiagnostics = nil
	configPath = filepath.Join(t.TempDir(), "nonexistent.yaml")
}

// TestLogFlagsFromEnvironment verifies that LOG_LEVEL/LOG_FORMAT/LOG_COLOR
// environment variables are applied by InitializeConfig when the
// corresponding flags are not set on the command line.
func TestLogFlagsFromEnvironment(t *testing.T) {
	resetRootLoggerState(t)
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "rfc3339")
	t.Setenv("LOG_COLOR", "off")

	InitializeConfig()

	require.Equal(t, logger.DEBUG, logLevel)
	require.Equal(t, logger.FORMAT_RFC3339, logFormat)
	require.Equal(t, logger.COLOR_OFF, logColor)
}

// TestLogFlagOverridesEnvironment verifies that an explicitly set flag wins
// over the corresponding environment variable.
func TestLogFlagOverridesEnvironment(t *testing.T) {
	resetRootLoggerState(t)
	t.Setenv("LOG_FORMAT", "rfc3339")
	t.Setenv("LOG_COLOR", "off")

	f := rootCmd.PersistentFlags().Lookup("log-format")
	require.NoError(t, rootCmd.PersistentFlags().Set("log-format", "basic"))
	t.Cleanup(func() {
		f.Changed = false
		logFormat = logger.FORMAT_JSON
	})

	InitializeConfig()

	require.Equal(t, logger.FORMAT_BASIC, logFormat, "explicit flag must win over env")
	require.Equal(t, logger.COLOR_OFF, logColor)
}

// TestInvalidLogEnvValueKeepsDefault verifies that an invalid LOG_FORMAT
// value is reported as a warning and the previous value is kept.
func TestInvalidLogEnvValueKeepsDefault(t *testing.T) {
	resetRootLoggerState(t)
	t.Setenv("LOG_FORMAT", "bogus")
	t.Setenv("LOG_COLOR", "bogus")

	InitializeConfig()

	require.Equal(t, logger.FORMAT_JSON, logFormat)
	require.Equal(t, logger.COLOR_AUTO, logColor)
}

// TestConfigDiagnosticsDeferredUntilLoggerReady verifies that a diagnostic
// emitted while configuration loads (a missing config file) is collected rather
// than written immediately, then replayed through the configured logger so it
// honors both --log-format and --log-file (see ochami#167).
func TestConfigDiagnosticsDeferredUntilLoggerReady(t *testing.T) {
	resetRootLoggerState(t)
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "basic")

	// point at a config file that doesn't exist so ReadInConfig fails and a
	// debug diagnostic is collected
	configPath = filepath.Join(t.TempDir(), "missing.yaml")

	logPath := filepath.Join(t.TempDir(), "startup.log")
	logFile = logPath

	// capture stderr by swapping os.Stderr before InitializeLogger builds the
	// stderr writer
	r, w, err := os.Pipe()
	require.NoError(t, err)
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	InitializeConfig()
	// the diagnostic must be collected, not emitted through the default logger
	require.NotEmpty(t, configDiagnostics, "missing config must produce a collected diagnostic")

	InitializeLogger()
	emitConfigDiagnostics()

	require.NoError(t, w.Close())
	os.Stderr = origStderr
	stderrBytes, err := io.ReadAll(r)
	require.NoError(t, err)
	stderrOut := string(stderrBytes)

	// the diagnostic was rendered in the configured basic format (not raw JSON)
	require.Contains(t, stderrOut, "failed to load config")
	require.NotContains(t, stderrOut, `{"level"`, "diagnostic must use the configured format, not raw JSON")

	// and it was appended to the log file as raw JSON
	require.NoError(t, logger.LogFile.Sync())
	fileBytes, err := os.ReadFile(logPath)
	require.NoError(t, err)
	require.Contains(t, string(fileBytes), "failed to load config")
	require.Contains(t, string(fileBytes), `"level":"debug"`)
}

// TestConfigDiagnosticsHonourDisabledLevel verifies that config diagnostics are
// subject to the resolved log level: with LOG_LEVEL=disabled, a collected
// warning is not emitted even though it was recorded during config load.
func TestConfigDiagnosticsHonourDisabledLevel(t *testing.T) {
	resetRootLoggerState(t)
	t.Setenv("LOG_LEVEL", "disabled")
	t.Setenv("LOG_FORMAT", "bogus") // triggers a "failed to set flag from config" warning

	r, w, err := os.Pipe()
	require.NoError(t, err)
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	InitializeConfig()
	require.NotEmpty(t, configDiagnostics, "invalid LOG_FORMAT must produce a collected diagnostic")

	InitializeLogger()
	emitConfigDiagnostics()

	require.NoError(t, w.Close())
	os.Stderr = origStderr
	stderrBytes, err := io.ReadAll(r)
	require.NoError(t, err)

	require.NotContains(t, string(stderrBytes), "failed to set flag from config",
		"LOG_LEVEL=disabled must suppress collected warnings")
}
