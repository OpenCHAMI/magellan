package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/magellan/internal/insecure"
	logger "github.com/openchami/magellan/internal/log"
	"github.com/openchami/magellan/pkg/secrets"
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

// TestSecretsBackendFlagRegistered verifies that the --secrets-backend flag
// exists as a persistent flag with the local backend as its default and
// working shell completion.
func TestSecretsBackendFlagRegistered(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("secrets-backend")
	require.NotNil(t, f)
	require.Equal(t, "local", f.DefValue)
	require.Equal(t, "Backend", f.Value.Type())
	require.Empty(t, f.Shorthand)

	vals, directive := completionSecretsBackend(nil, nil, "")
	require.Equal(t, cobra.ShellCompDirectiveDefault, directive)
	require.Len(t, vals, len(secrets.BackendHelpMap))
	require.Contains(t, vals, "local\tEncrypted JSON file on disk (requires MASTER_KEY)")
	require.Contains(t, vals, "bao\tOpenBao KV v2 secrets engine (requires BAO_ADDR and BAO_TOKEN or AppRole env vars)")
	require.Contains(t, vals, "vault\tHashiCorp Vault KV v2 secrets engine (requires VAULT_ADDR and VAULT_TOKEN or AppRole env vars)")
}

// TestSecretsBackendFromEnvironment verifies that SECRETS_BACKEND is applied
// by InitializeConfig when the flag is not set on the command line, and that
// invalid values are rejected without clobbering the default.
func TestSecretsBackendFromEnvironment(t *testing.T) {
	resetRootLoggerState(t)
	t.Cleanup(func() { secretsBackend = secrets.BackendLocal })
	secretsBackend = secrets.BackendLocal
	t.Setenv("SECRETS_BACKEND", "bao")

	InitializeConfig()
	require.Equal(t, secrets.BackendBao, secretsBackend)
}

func TestInvalidSecretsBackendFromEnvironment(t *testing.T) {
	resetRootLoggerState(t)
	t.Cleanup(func() { secretsBackend = secrets.BackendLocal })
	secretsBackend = secrets.BackendLocal
	t.Setenv("SECRETS_BACKEND", "consul")

	InitializeConfig()
	require.Equal(t, secrets.BackendLocal, secretsBackend)
	require.NotEmpty(t, configDiagnostics)
	d := configDiagnostics[len(configDiagnostics)-1]
	require.Equal(t, logger.WARN, d.level)
	require.Error(t, d.err)
	require.Equal(t, "failed to set flag from config", d.msg)
	require.Equal(t, "secrets-backend", d.fields["key"])
}

// TestInsecureFlagScopes verifies every registration of the scoped --insecure
// flag: type, default, and the bare form resolving to the bmc scope.
func TestInsecureFlagScopes(t *testing.T) {
	commands := map[string]*cobra.Command{
		"scan":           ScanCmd,
		"crawl":          CrawlCmd,
		"collect":        CollectCmd,
		"power":          PowerCmd,
		"serve":          ServeCmd,
		"update":         updateCmd,
		"settings list":  SettingsListCmd,
		"settings get":   SettingsGetCmd,
		"settings set":   SettingsSetCmd,
		"settings reset": SettingsResetCmd,
	}
	for name, cmd := range commands {
		f := cmd.Flags().Lookup("insecure")
		require.NotNil(t, f, "%s must register --insecure", name)
		require.Equal(t, "Scope", f.Value.Type(), name)
		require.Equal(t, "none", f.DefValue, name)
		require.Equal(t, "i", f.Shorthand, name)
		require.Equal(t, "bmc", f.NoOptDefVal, "%s: bare --insecure must mean bmc", name)
	}
}

// TestInsecureFlagParsing verifies that the flag parses to the expected
// scopes, including the boolean aliases kept for backwards compatibility.
func TestInsecureFlagParsing(t *testing.T) {
	tests := []struct {
		args    []string
		want    insecure.Scope
		wantBMC bool
		wantSec bool
	}{
		{args: []string{"-i"}, want: insecure.BMC, wantBMC: true},
		{args: []string{"--insecure"}, want: insecure.BMC, wantBMC: true},
		{args: []string{"--insecure=bmc"}, want: insecure.BMC, wantBMC: true},
		{args: []string{"--insecure=secrets"}, want: insecure.Secrets, wantSec: true},
		{args: []string{"--insecure=both"}, want: insecure.Both, wantBMC: true, wantSec: true},
		{args: []string{"--insecure=true"}, want: insecure.BMC, wantBMC: true},
		{args: []string{"--insecure=false"}, want: insecure.None},
	}
	f := ScanCmd.Flags().Lookup("insecure")
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			t.Cleanup(func() {
				f.Changed = false
				insecureScope = insecure.None
			})
			insecureScope = insecure.None
			require.NoError(t, ScanCmd.Flags().Parse(tt.args))
			require.Equal(t, tt.want, insecureScope)
			require.Equal(t, tt.wantBMC, insecureScope.BMC())
			require.Equal(t, tt.wantSec, insecureScope.Secrets())
		})
	}

	t.Run("invalid value is rejected", func(t *testing.T) {
		err := ScanCmd.Flags().Parse([]string{"--insecure=maybe"})
		require.ErrorContains(t, err, "invalid insecure scope")
	})
}

// TestCompletionInsecureScope verifies the --insecure completion function.
func TestCompletionInsecureScope(t *testing.T) {
	vals, directive := completionInsecureScope(nil, nil, "")
	require.Equal(t, cobra.ShellCompDirectiveDefault, directive)
	require.Len(t, vals, len(insecure.ScopeHelpMap))
	require.Contains(t, vals, "none\tVerify TLS certificates everywhere (default)")
	require.Contains(t, vals, "bmc\tSkip TLS verification for BMC connections")
	require.Contains(t, vals, "secrets\tSkip TLS verification for the secret manager connection")
	require.Contains(t, vals, "both\tSkip TLS verification for BMC and secret manager connections")
}
