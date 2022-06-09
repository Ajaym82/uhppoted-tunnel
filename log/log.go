package log

import (
	"fmt"
	syslog "log"
)

type LogLevel int

const (
	none LogLevel = iota
	debug
	info
	warn
	errors
)

var debugging = false
var level = info

func SetDebug(enabled bool) {
	debugging = enabled
}

func SetLevel(l string) {
	switch l {
	case "none":
		level = none
	case "debug":
		level = debug
	case "info":
		level = info
	case "warn":
		level = warn
	case "error":
		level = errors
	}
}

func Debugf(format string, args ...any) {
	if debugging || level < info {
		syslog.Printf("%-5v  %v", "DEBUG", fmt.Sprintf(format, args...))
	}
}

func Infof(format string, args ...any) {
	if level < warn {
		syslog.Printf("%-5v  %v", "INFO", fmt.Sprintf(format, args...))
	}
}

func Warnf(format string, args ...any) {
	if level < errors {
		syslog.Printf("%-5v  %v", "WARN", fmt.Sprintf(format, args...))
	}
}

func Errorf(format string, args ...any) {
	syslog.Printf("%-5v  %v", "ERROR", fmt.Sprintf(format, args...))
}

func Fatalf(format string, args ...any) {
	syslog.Fatalf("%-5v  %v", "FATAL", fmt.Sprintf(format, args...))
}
