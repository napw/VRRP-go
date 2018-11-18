package logger

import (
	"io"
	"log"
	"os"
)

type Logger struct {
	level  LogLevel
	output *log.Logger
}

func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
	if level == DEBUG {
		l.output.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
	}
}

func (l *Logger) SetPrefix(pre string) {
	l.output.SetPrefix(pre)
}

func (l *Logger) Printf(level LogLevel, format string, a ...interface{}) {
	if level < l.level {
		return
	} else {
		l.output.Printf(format, a)
	}

}

func NewLogger(o *io.Writer) *Logger {
	if o == nil {
		return &Logger{level: INFO, output: log.New(os.Stdout, "", log.LstdFlags)}
	} else {
		return &Logger{level: INFO, output: log.New(*o, "", log.LstdFlags)}
	}
}