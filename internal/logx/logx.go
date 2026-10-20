// Package logx is a tiny levelled logger with the same line format as the Python agent:
// "HH:MM:SS LEVEL   message".
package logx

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Level is a log severity.
type Level int

const (
	DEBUG Level = 10
	INFO  Level = 20
	WARN  Level = 30
	ERROR Level = 40
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARNING"
	case ERROR:
		return "ERROR"
	}
	return "LEVEL"
}

var (
	mu      sync.Mutex
	level   = INFO
	outputs = []io.Writer{os.Stderr}
	hook    func(Level, string) // tests
)

// Setup sets the level and outputs (stderr plus an optional log file).
func Setup(verbose bool, logFile string) error {
	mu.Lock()
	defer mu.Unlock()
	level = INFO
	if verbose {
		level = DEBUG
	}
	outputs = []io.Writer{os.Stderr}
	if logFile != "" {
		if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		outputs = append(outputs, f)
	}
	return nil
}

// SetHook installs a function that receives every record regardless of level (for tests).
// It returns a function restoring the previous hook.
func SetHook(h func(Level, string)) func() {
	mu.Lock()
	prev := hook
	hook = h
	mu.Unlock()
	return func() {
		mu.Lock()
		hook = prev
		mu.Unlock()
	}
}

// Enabled reports whether messages at l are written.
func Enabled(l Level) bool {
	mu.Lock()
	defer mu.Unlock()
	return l >= level
}

func logf(l Level, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	mu.Lock()
	defer mu.Unlock()
	if hook != nil {
		hook(l, msg)
	}
	if l < level {
		return
	}
	line := fmt.Sprintf("%s %-7s %s\n", time.Now().Format("15:04:05"), l, msg)
	for _, w := range outputs {
		_, _ = io.WriteString(w, line)
	}
}

func Debugf(format string, args ...any) { logf(DEBUG, format, args...) }
func Infof(format string, args ...any)  { logf(INFO, format, args...) }
func Warnf(format string, args ...any)  { logf(WARN, format, args...) }
func Errorf(format string, args ...any) { logf(ERROR, format, args...) }
