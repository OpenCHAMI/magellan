package log

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rs/zerolog"
)

// string representation that directly corresponds to zerolog.Level
type LogLevel string

const (
	DEBUG    LogLevel = "debug"
	INFO     LogLevel = "info"
	WARN     LogLevel = "warn"
	ERROR    LogLevel = "error"
	DISABLED LogLevel = "disabled"
	TRACE    LogLevel = "trace"
)

var levels = [6]LogLevel{DEBUG, INFO, WARN, ERROR, DISABLED, TRACE}

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

func strToLogLevel(ll LogLevel) (zerolog.Level, error) {
	var tostr = func(lls []LogLevel) []string {
		s := []string{}
		for _, l := range lls {
			s = append(s, string(l))
		}
		return s
	}
	if index := slices.Index(levels[:], ll); index >= 0 {
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
		"invalid log level (options: %s)", strings.Join(tostr(levels[:]), ", "),
	) // use 'info' by default
}

func Init(logLevel LogLevel, logFormat string) error {
	// set the logging level
	level, err := strToLogLevel(logLevel)
	if err != nil {
		return fmt.Errorf("failed to convert log level: %v", err)
	}
	zerolog.SetGlobalLevel(level)
	return nil
}
