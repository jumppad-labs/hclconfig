package logger

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestLogger implements the Logger interface and buffers logs until test failure
// This logger only outputs logs if the test fails, keeping successful test output clean
type TestLogger struct {
	t      *testing.T
	buffer []logEntry
	mu     sync.Mutex
}

type logEntry struct {
	level     string
	message   string
	args      []interface{}
	timestamp time.Time
}

// NewTestLogger creates a new TestLogger that will output logs only on test failure
func NewTestLogger(t *testing.T) *TestLogger {
	logger := &TestLogger{
		t:      t,
		buffer: make([]logEntry, 0),
	}

	// Register cleanup function to flush logs if test failed
	t.Cleanup(func() {
		logger.flushIfFailed()
	})

	return logger
}

// Ensure TestLogger implements the Logger interface
var _ Logger = (*TestLogger)(nil)

func (l *TestLogger) Info(msg string, args ...interface{}) {
	l.addEntry("INFO", msg, args)
}

func (l *TestLogger) Debug(msg string, args ...interface{}) {
	l.addEntry("DEBUG", msg, args)
}

func (l *TestLogger) Warn(msg string, args ...interface{}) {
	l.addEntry("WARN", msg, args)
}

func (l *TestLogger) Error(msg string, args ...interface{}) {
	l.addEntry("ERROR", msg, args)
}

func (l *TestLogger) addEntry(level, msg string, args []interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.buffer = append(l.buffer, logEntry{
		level:     level,
		message:   msg,
		args:      args,
		timestamp: time.Now(),
	})
}

func (l *TestLogger) flushIfFailed() {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if test failed
	if l.t.Failed() {
		l.t.Log("=== Buffered Logs (test failed) ===")
		for _, entry := range l.buffer {
			timestamp := entry.timestamp.Format("15:04:05.000")
			msg := fmt.Sprintf("[%s] [%s] %s", timestamp, entry.level, entry.message)

			if len(entry.args) > 0 {
				// Format args as key-value pairs like charmbracelet/log
				var parts []string
				for i := 0; i < len(entry.args); i += 2 {
					if i+1 < len(entry.args) {
						parts = append(parts, fmt.Sprintf("%v=%v", entry.args[i], entry.args[i+1]))
					} else {
						parts = append(parts, fmt.Sprintf("%v", entry.args[i]))
					}
				}
				if len(parts) > 0 {
					msg += " " + strings.Join(parts, " ")
				}
			}

			l.t.Log(msg)
		}
		l.t.Log("=== End Buffered Logs ===")
	}

	// Clear buffer after flushing
	l.buffer = l.buffer[:0]
}
