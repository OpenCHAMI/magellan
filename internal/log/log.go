package log

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
)

var Logger zerolog.Logger

func Init(logLevel, logFormat string) error {
	var (
		ll zerolog.Level
		cw = zerolog.ConsoleWriter{Out: os.Stderr}
	)
	switch logLevel {
	case "warning":
		ll = zerolog.WarnLevel
	case "info":
		ll = zerolog.InfoLevel
	case "debug":
		ll = zerolog.DebugLevel
	default:
		return fmt.Errorf("unknown log level: %s", ll)
	}

	switch logFormat {
	case "basic":
		cw.TimeFormat = time.RFC3339
		// cw.FormatCaller
	case "json":
		Logger = zerolog.New(cw).Level(ll).With().Timestamp().Logger()
	default:
		return fmt.Errorf("unknown log format %s", logFormat)
	}

	return nil
}
