package log

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
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

func TestLogFormat(t *testing.T) {
	var format LogFormat
	for _, want := range Formats {
		require.NoError(t, format.Set(want.String()))
		require.Equal(t, want, format)
		require.Equal(t, "LogFormat", format.Type())
		require.NotEmpty(t, LogFormatHelpMap[want.String()])
	}
	require.Error(t, format.Set("invalid"))
}

func TestLogColor(t *testing.T) {
	var color LogColor
	for _, want := range Colors {
		require.NoError(t, color.Set(want.String()))
		require.Equal(t, want, color)
		require.Equal(t, "LogColor", color.Type())
		require.NotEmpty(t, LogColorHelpMap[want.String()])
	}
	require.Error(t, color.Set("invalid"))
}

func TestInit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "magellan.log")
	require.NoError(t, Init(DEBUG, FORMAT_JSON, COLOR_AUTO, path))
	require.NotNil(t, LogFile)
	require.NoError(t, LogFile.Close())
	require.Error(t, Init(LogLevel("invalid"), FORMAT_JSON, COLOR_AUTO, ""))
}

func TestInitErrors(t *testing.T) {
	require.Error(t, Init(INFO, LogFormat("bogus"), COLOR_AUTO, ""))
	require.Error(t, Init(INFO, FORMAT_JSON, LogColor("bogus"), ""))
	require.Error(t, SetEarlyLevel(LogLevel("bogus")))
	require.NoError(t, SetEarlyLevel(INFO))
}

func TestNewStderrWriterJSON(t *testing.T) {
	var buf bytes.Buffer
	w, err := newStderrWriter(FORMAT_JSON, COLOR_ON, &buf)
	require.NoError(t, err)
	l := zerolog.New(w).With().Timestamp().Caller().Logger()
	l.Info().Str("k", "v").Msg("hello")
	require.True(t, json.Valid(buf.Bytes()), "json format must emit raw JSON, got: %s", buf.String())
	require.NotContains(t, buf.String(), "\x1b[", "json format must never be colorized")
	require.Contains(t, buf.String(), `"message":"hello"`)
}

func TestNewStderrWriterRFC3339(t *testing.T) {
	var buf bytes.Buffer
	w, err := newStderrWriter(FORMAT_RFC3339, COLOR_OFF, &buf)
	require.NoError(t, err)
	l := zerolog.New(w).With().Timestamp().Caller().Logger()
	l.Info().Str("k", "v").Msg("hello")
	out := buf.String()
	require.False(t, json.Valid([]byte(out)), "rfc3339 format must render console text, got: %s", out)
	require.Contains(t, out, "hello")
	require.NotContains(t, out, "\x1b[", "COLOR_OFF must not colorize")
	require.Regexp(t, `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`, out, "rfc3339 format must include a timestamp")
}

func TestNewStderrWriterBasic(t *testing.T) {
	var buf bytes.Buffer
	w, err := newStderrWriter(FORMAT_BASIC, COLOR_OFF, &buf)
	require.NoError(t, err)
	l := zerolog.New(w).With().Timestamp().Caller().Logger()
	l.Info().Str("k", "v").Msg("hello")
	out := buf.String()
	require.Contains(t, out, "INFO  |", "basic format must use ochami-style level prefix")
	require.Contains(t, out, "hello")
	require.Contains(t, out, "log_test.go", "caller must be reduced to the base file name")
	require.NotContains(t, out, "\x1b[", "COLOR_OFF must not colorize")
	require.NotRegexp(t, `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`, out, "basic format must not render timestamps")
}

func TestNewStderrWriterColorOn(t *testing.T) {
	var buf bytes.Buffer
	w, err := newStderrWriter(FORMAT_BASIC, COLOR_ON, &buf)
	require.NoError(t, err)
	l := zerolog.New(w).With().Timestamp().Caller().Logger()
	l.Info().Msg("colored")
	require.Contains(t, buf.String(), "\x1b[", "COLOR_ON must colorize even on a non-terminal writer")
}

func TestNewStderrWriterInvalid(t *testing.T) {
	_, err := newStderrWriter(LogFormat("bogus"), COLOR_AUTO, &bytes.Buffer{})
	require.Error(t, err)
}

func TestNoColor(t *testing.T) {
	// a non-terminal writer never receives colors in auto mode
	require.True(t, noColor(COLOR_AUTO, &bytes.Buffer{}))
	require.True(t, noColor(COLOR_OFF, &bytes.Buffer{}))
	require.False(t, noColor(COLOR_ON, &bytes.Buffer{}))
	// NO_COLOR (https://no-color.org) disables colors in auto mode
	t.Setenv("NO_COLOR", "1")
	require.True(t, noColor(COLOR_AUTO, os.Stderr))
}

func TestFormatCaller(t *testing.T) {
	f := formatCaller(true)
	require.Equal(t, "", f(nil))
	require.Equal(t, "", f(123))
	require.Equal(t, "", f(""))
	require.Equal(t, "root.go:42 >", f("/home/user/magellan/cmd/root.go:42"))
	require.Equal(t, "root.go:42 >", f("cmd/root.go:42"))
}

// TestLogFileAlwaysJSON verifies that even when stderr uses a console format,
// messages appended to the log file remain raw JSON events.
func TestLogFileAlwaysJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "magellan.log")
	require.NoError(t, Init(INFO, FORMAT_BASIC, COLOR_OFF, path))
	zlog.Info().Str("k", "v").Msg("to file")
	require.NoError(t, LogFile.Sync())
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.True(t, json.Valid(data), "log file must contain raw JSON, got: %s", string(data))
	require.Contains(t, string(data), `"message":"to file"`)
	require.NotContains(t, string(data), "\x1b[", "log file must not contain color codes")
	// the timestamp stays in the event even though basic format hides it on stderr
	require.Contains(t, string(data), `"time"`)
	require.NoError(t, LogFile.Close())
}

// TestInitClosesPreviousFile verifies that re-initializing the logger with a
// new log file path closes the previously opened file.
func TestInitClosesPreviousFile(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.log")
	second := filepath.Join(dir, "second.log")
	require.NoError(t, Init(INFO, FORMAT_JSON, COLOR_OFF, first))
	firstFile := LogFile
	require.NoError(t, Init(INFO, FORMAT_JSON, COLOR_OFF, second))
	require.NotSame(t, firstFile, LogFile)
	// writing to the closed first file must fail
	_, err := firstFile.Write([]byte("x"))
	require.Error(t, err)
	require.NoError(t, LogFile.Close())
}

func TestSetEarlyLevelFiltersDefaultLogger(t *testing.T) {
	// early messages (before Init) go through zerolog's default logger
	var buf bytes.Buffer
	orig := zlog.Logger
	zlog.Logger = zerolog.New(&buf)
	defer func() { zlog.Logger = orig }()
	require.NoError(t, SetEarlyLevel(INFO))
	zlog.Debug().Msg("hidden")
	zlog.Error().Msg("shown")
	require.NotContains(t, buf.String(), "hidden")
	require.Contains(t, buf.String(), "shown")
	require.NotContains(t, buf.String(), "\x1b[", "pre-init messages are raw JSON")
}

// TestInitInstallsGlobalLogger ensures Init installs a working global logger
// and that SetEarlyLevel applies the global level filter.
func TestInitInstallsGlobalLogger(t *testing.T) {
	require.NoError(t, Init(INFO, FORMAT_JSON, COLOR_OFF, ""))
	require.Equal(t, zerolog.InfoLevel, zlog.Logger.GetLevel())
	require.NoError(t, SetEarlyLevel(DEBUG))
	require.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())
	require.NoError(t, SetEarlyLevel(INFO))
}
