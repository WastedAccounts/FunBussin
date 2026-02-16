package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type Logger struct {
	level       int
	errorLog    string
	activityLog string
	serviceLog  string
}

var once sync.Once
var logger *Logger

// GetLogger returns the singleton logger instance.
func GetLogger() *Logger {
	once.Do(func() {
		logger = &Logger{}
	})
	return logger
}

// SetLogLevel sets the log level. 1=ALL, 3=DEBUG, 5=ERROR.
func (l *Logger) SetLogLevel(level int) {
	if level == 0 {
		l.level = 1
	} else {
		l.level = level
	}
}

// SetLogFileNames sets the paths for activity, error, and service log files.
func (l *Logger) SetLogFileNames(actlog, errlog, svclog string) {
	l.activityLog = actlog
	l.errorLog = errlog
	l.serviceLog = svclog
}

func (l *Logger) writeLog(path, prefix, pkg, msg string) {
	if path == "" {
		return
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("logging: failed to create log dir %s: %v", dir, err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("logging: failed to open %s: %v", path, err)
		return
	}
	defer f.Close()
	logger := log.New(f, prefix, log.Ldate|log.Ltime|log.Lmicroseconds|log.LUTC)
	logger.Printf("-- %s -- %s", pkg, msg)
}

// ErrorLogging logs errors. lvl: 3=DEBUG, 5=ERROR.
func (l *Logger) ErrorLogging(lvl int, pkg, msg string) {
	if l.level <= 3 && lvl <= 3 {
		l.writeLog(l.errorLog, "[DEBUG] -- ", pkg, msg)
	}
	if lvl >= 5 {
		l.writeLog(l.errorLog, "[ERROR] -- ", pkg, msg)
	}
}

// ActivityLogging logs application activity.
func (l *Logger) ActivityLogging(pkg, msg string) {
	l.writeLog(l.activityLog, "[ACTIVITY] -- ", pkg, msg)
}

// ServiceLogging logs service states.
func (l *Logger) ServiceLogging(pkg, msg string) {
	l.writeLog(l.serviceLog, "[SERVICE] -- ", pkg, msg)
}

// Info writes to stdout for immediate feedback (e.g. startup, import progress).
func (l *Logger) Info(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

// Fatal logs the error and exits with code 1.
func (l *Logger) Fatal(pkg, msg string) {
	l.ErrorLogging(5, pkg, msg)
	os.Exit(1)
}

// Fatalf logs a formatted error and exits with code 1.
func (l *Logger) Fatalf(pkg, format string, args ...interface{}) {
	l.ErrorLogging(5, pkg, fmt.Sprintf(format, args...))
	os.Exit(1)
}
