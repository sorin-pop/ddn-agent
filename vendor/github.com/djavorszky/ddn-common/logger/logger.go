package logger

import (
	"fmt"
	"log"
	"os"
)

// LogLevel is used to determine what to log.
type LogLevel int

func (l LogLevel) String() string {
	switch l {
	case 31:
		return "fatal"
	case 15:
		return "error"
	case 7:
		return "warn"
	case 3:
		return "info"
	case 1:
		return "debug"
	default:
		return "unknown"
	}
}

// The available log levels
const (
	FATAL LogLevel = 31
	ERROR LogLevel = 15
	WARN  LogLevel = 7
	INFO  LogLevel = 3
	DEBUG LogLevel = 1
)

// Level is to be used to control the log level of the application.
var Level = INFO

// Fatal should be used to log a critical incident and exit the application
func Fatal(msg string, args ...interface{}) {
	defer os.Exit(1)

	log.Printf("[%s] %s", FATAL, fmt.Sprintf(msg, args...))
}

// Error should be used for application errors that should be resolved
func Error(msg string, args ...interface{}) {
	if shouldLog(ERROR) {
		log.Printf("[%s] %s", ERROR, fmt.Sprintf(msg, args...))
	}
}

// Warn should be used for events that can be dangerous
func Warn(msg string, args ...interface{}) {
	if shouldLog(WARN) {
		log.Printf("[%s]  %s", WARN, fmt.Sprintf(msg, args...))
	}
}

// Info should be used to share data.
func Info(msg string, args ...interface{}) {
	if shouldLog(INFO) {
		log.Printf("[%s]  %s", INFO, fmt.Sprintf(msg, args...))
	}
}

// Debug should be used for debugging purposes only.
func Debug(msg string, args ...interface{}) {
	if shouldLog(DEBUG) {
		log.Printf("[%s] %s", DEBUG, fmt.Sprintf(msg, args...))
	}
}

func shouldLog(lvl LogLevel) bool {
	if Level&lvl == Level {
		return true
	}

	return false
}
