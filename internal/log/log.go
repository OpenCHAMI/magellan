package log

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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

func InitWithLogLevel(logLevel LogLevel, logPath string) error {
	var (
		logger  zerolog.Logger
		level   zerolog.Level
		writer  zerolog.LevelWriter
		writers []io.Writer
		err     error
	)

	// set the logging level
	level, err = strToLogLevel(logLevel)
	if err != nil {
		return fmt.Errorf("failed to convert log level: %v", err)
	}

	// add the default stderr writer
	writers = append(writers, &zerolog.FilteredLevelWriter{
		Writer: &zerolog.LevelWriterAdapter{Writer: os.Stderr},
		Level:  level,
	})

	// add another writer to write to a log file
	if logPath != "" {
		LogFile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0664)
		if err != nil {
			return fmt.Errorf("failed to open log file: %v", err)
		}

		// add another write to write to the specified log file
		writers = append(writers, &zerolog.FilteredLevelWriter{
			Writer: zerolog.LevelWriterAdapter{Writer: LogFile},
			Level:  level,
		})
	}
	writer = zerolog.MultiLevelWriter(writers...)
	logger = zerolog.New(writer).Level(level).With().Timestamp().Caller().Logger()
	zerolog.SetGlobalLevel(level)
	log.Logger = logger
	return nil
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
