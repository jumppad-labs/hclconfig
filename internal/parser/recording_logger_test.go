package parser

import (
	"sync"

	"github.com/jumppad-labs/xcl/logger"
)

// loggedMessage is a single call made to a recordingLogger
type loggedMessage struct {
	msg  string
	args []any
}

// recordingLogger is a logger.Logger that records every Warn call so tests can
// assert on it. The walker logs from parallel goroutines, so it is guarded by mu.
type recordingLogger struct {
	mu    sync.Mutex
	warns []loggedMessage
}

var _ logger.Logger = (*recordingLogger)(nil)

func (l *recordingLogger) Info(msg string, args ...any) {}

func (l *recordingLogger) Debug(msg string, args ...any) {}

func (l *recordingLogger) Error(msg string, args ...any) {}

func (l *recordingLogger) Warn(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.warns = append(l.warns, loggedMessage{msg: msg, args: append([]any{}, args...)})
}

// warnings returns a copy of every recorded Warn call, in order
func (l *recordingLogger) warnings() []loggedMessage {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]loggedMessage{}, l.warns...)
}

// warningsWithMessage returns the args of every recorded Warn call with the
// given message, in order
func (l *recordingLogger) warningsWithMessage(msg string) [][]any {
	matching := [][]any{}
	for _, warning := range l.warnings() {
		if warning.msg == msg {
			matching = append(matching, warning.args)
		}
	}

	return matching
}
