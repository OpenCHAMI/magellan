package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/term"
)

// string representation that directly corresponds to zerolog.Level
type (
	LogFilter    string
	LogLevel     string
	LogLevelList []LogLevel
)

const (
	DEBUG    LogLevel = "debug"
	INFO     LogLevel = "info"
	WARN     LogLevel = "warn"
	ERROR    LogLevel = "error"
	DISABLED LogLevel = "disabled"
	TRACE    LogLevel = "trace"
)

var Levels = [6]LogLevel{DEBUG, INFO, WARN, ERROR, DISABLED, TRACE}
var LogFile *os.File

func (ll LogLevel) String() string {
	return string(ll)
}

func (ll *LogLevel) Set(v string) error {
	switch LogLevel(v) {
	case DEBUG, INFO, WARN, ERROR, DISABLED, TRACE:
		*ll = LogLevel(v)
		return nil
	default:
		return fmt.Errorf("must be one of %v", []LogLevel{
			DEBUG,
			INFO,
			WARN,
			ERROR,
			DISABLED,
			TRACE,
		})
	}
}

func (df LogLevel) Type() string {
	return "LogLevel"
}

// LogFormat controls the rendering of log messages on stderr. It has no
// effect on the contents of the log file (see Init), which is always raw
// JSON, nor on any data output formats of magellan commands.
type LogFormat string

const (
	FORMAT_JSON    LogFormat = "json"
	FORMAT_RFC3339 LogFormat = "rfc3339"
	FORMAT_BASIC   LogFormat = "basic"
)

var Formats = [3]LogFormat{FORMAT_JSON, FORMAT_RFC3339, FORMAT_BASIC}

var LogFormatHelpMap = map[string]string{
	string(FORMAT_JSON):    "JSON format",
	string(FORMAT_RFC3339): "RFC3339 timestamped format",
	string(FORMAT_BASIC):   "Basic format without timestamps",
}

func (lf LogFormat) String() string {
	return string(lf)
}

func (lf *LogFormat) Set(v string) error {
	switch LogFormat(v) {
	case FORMAT_JSON, FORMAT_RFC3339, FORMAT_BASIC:
		*lf = LogFormat(v)
		return nil
	default:
		return fmt.Errorf("must be one of %v", Formats)
	}
}

func (lf LogFormat) Type() string {
	return "LogFormat"
}

// LogColor controls whether ANSI color codes are added to log messages on
// stderr for the console formats (rfc3339 and basic).
type LogColor string

const (
	COLOR_AUTO LogColor = "auto"
	COLOR_ON   LogColor = "on"
	COLOR_OFF  LogColor = "off"
)

var Colors = [3]LogColor{COLOR_AUTO, COLOR_ON, COLOR_OFF}

var LogColorHelpMap = map[string]string{
	string(COLOR_AUTO): "Color when stderr is a terminal",
	string(COLOR_ON):   "Always colorize",
	string(COLOR_OFF):  "Never colorize",
}

func (lc LogColor) String() string {
	return string(lc)
}

func (lc *LogColor) Set(v string) error {
	switch LogColor(v) {
	case COLOR_AUTO, COLOR_ON, COLOR_OFF:
		*lc = LogColor(v)
		return nil
	default:
		return fmt.Errorf("must be one of %v", Colors)
	}
}

func (lc LogColor) Type() string {
	return "LogColor"
}

// Init initializes the global logging object so it can be used for logging
// by any package that imports this internal log package. Log messages are
// written to stderr using the given format and colorization settings. If
// logPath is non-empty, a copy of every message is appended to that file as
// raw JSON regardless of format.
func Init(level LogLevel, format LogFormat, color LogColor, logPath string) error {
	var (
		writers []io.Writer
		err     error
	)

	// set the logging level
	zl, err := strToLogLevel(level)
	if err != nil {
		return fmt.Errorf("failed to convert log level: %v", err)
	}

	// add the default stderr writer
	stderrW, err := newStderrWriter(format, color, os.Stderr)
	if err != nil {
		return err
	}
	writers = append(writers, &zerolog.FilteredLevelWriter{
		Writer: stderrW,
		Level:  zl,
	})

	// add another writer to write to a log file
	if logPath != "" {
		// close any previously opened log file to avoid leaking its
		// file descriptor across re-initializations (e.g. in tests)
		if LogFile != nil {
			_ = LogFile.Close()
			LogFile = nil
		}
		LogFile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0664)
		if err != nil {
			return fmt.Errorf("failed to open log file: %v", err)
		}

		// log files always record raw JSON events with timestamps,
		// independent of the stderr format
		writers = append(writers, &zerolog.FilteredLevelWriter{
			Writer: zerolog.LevelWriterAdapter{Writer: LogFile},
			Level:  zl,
		})
	}
	writer := zerolog.MultiLevelWriter(writers...)
	logger := zerolog.New(writer).Level(zl).With().Timestamp().Caller().Logger()
	zerolog.SetGlobalLevel(zl)
	log.Logger = logger
	return nil
}

// newStderrWriter builds the zerolog writer responsible for rendering log
// messages on out according to the given format and color settings. The io.Writer
// is injectable to allow unit testing of the rendered output.
func newStderrWriter(format LogFormat, color LogColor, out io.Writer) (zerolog.LevelWriter, error) {
	if !slices.Contains(Colors[:], color) {
		return nil, fmt.Errorf("unknown log color: %s (options: %s)", color, joinColors())
	}
	switch format {
	case FORMAT_JSON:
		// raw zerolog JSON; never colorized
		return &zerolog.LevelWriterAdapter{Writer: out}, nil
	case FORMAT_RFC3339:
		cw := zerolog.ConsoleWriter{
			Out:        out,
			TimeFormat: time.RFC3339,
			NoColor:    noColor(color, out),
		}
		cw.FormatCaller = formatCaller(cw.NoColor)
		return &zerolog.LevelWriterAdapter{Writer: cw}, nil
	case FORMAT_BASIC:
		cw := zerolog.ConsoleWriter{
			Out:     out,
			NoColor: noColor(color, out),
		}
		// blank the timestamp on the console; the timestamp remains in
		// the event so that log files still record it
		cw.FormatTimestamp = func(i interface{}) string { return "" }
		cw.FormatLevel = func(i interface{}) string {
			return strings.ToUpper(fmt.Sprintf("%-6s|", i))
		}
		cw.FormatCaller = formatCaller(cw.NoColor)
		return &zerolog.LevelWriterAdapter{Writer: cw}, nil
	default:
		return nil, fmt.Errorf("unknown log format: %s (options: %s)", format, joinFormats())
	}
}

// noColor decides whether ANSI color codes must be suppressed for console
// log output written to out.
//
// The NO_COLOR environment variable (see https://no-color.org) overrides every
// color mode, including COLOR_ON, so that a single decision controls every
// rendered part of a log event. This matters because zerolog's built-in
// ConsoleWriter formatters independently check NO_COLOR; honoring it here as
// well keeps the custom formatters (e.g. formatCaller) consistent with them.
func noColor(color LogColor, out io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	switch color {
	case COLOR_ON:
		return false
	case COLOR_OFF:
		return true
	default: // COLOR_AUTO
		f, ok := out.(*os.File)
		return !ok || !term.IsTerminal(int(f.Fd()))
	}
}

// formatCaller returns a zerolog Formatter for ConsoleWriter.FormatCaller
// that uses the base name of the source file where the log message
// originated from (taken from the caller field of the log event), ensuring
// that it is still colorized, if enabled.
func formatCaller(noColor bool) zerolog.Formatter {
	return func(i interface{}) string {
		s, ok := i.(string)
		if !ok || s == "" {
			return ""
		}
		// zerolog's caller field is formatted as "path/to/file.go:123";
		// reduce the path portion to its base name. Split on the last
		// colon since the path itself may contain colons.
		path, line := s, ""
		if idx := strings.LastIndex(s, ":"); idx >= 0 {
			path, line = s[:idx], s[idx+1:]
		}
		out := filepath.Base(path)
		if line != "" {
			out += ":" + line
		}
		return colorize(out, colorBold, noColor) + colorize(" >", colorCyan, noColor)
	}
}

func joinFormats() string {
	s := make([]string, len(Formats))
	for i, f := range Formats {
		s[i] = string(f)
	}
	return strings.Join(s, ", ")
}

func joinColors() string {
	s := make([]string, len(Colors))
	for i, c := range Colors {
		s[i] = string(c)
	}
	return strings.Join(s, ", ")
}

func strToLogLevel(ll LogLevel) (zerolog.Level, error) {
	var tostr = func(lls []LogLevel) []string {
		s := []string{}
		for _, l := range lls {
			s = append(s, string(l))
		}
		return s
	}

	if index := slices.Index(Levels[:], ll); index >= 0 {
		// handle special cases to map index to DISABLED and TRACE
		switch index {
		case 4:
			return zerolog.Disabled, nil
		case 5:
			return zerolog.TraceLevel, nil
		}
		return zerolog.Level(index), nil
	}
	return -100, fmt.Errorf(
		"invalid log level (options: %s)", strings.Join(tostr(Levels[:]), ", "),
	) // use 'info' by default
}
