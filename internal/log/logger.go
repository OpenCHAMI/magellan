package log

import (
	"github.com/sirupsen/logrus"
)

type Logger struct {
	Log  *logrus.Logger
	Path string
}

func NewLogger(l *logrus.Logger, level logrus.Level) *Logger {
	l.SetLevel(level)
	return &Logger{
		Log:  logrus.New(),
		Path: "",
	}
}

func (l *Logger) WriteFile(path string) {

}
