package logger

// taggedLogger writes every message to next with its tag in front, so a
// message "create" tagged provider=postgres is written as
// "provider=postgres create"
type taggedLogger struct {
	next Logger
	tag  string
}

// Ensure taggedLogger implements the Logger interface
var _ Logger = (*taggedLogger)(nil)

// WithTag returns a Logger that writes to l, starting every message with
// key=value. Tags added by nested calls read outermost first:
// WithTag(WithTag(l, "provider", "postgres"), "plugin", "example") writes
// "plugin=example provider=postgres <message>". A nil l returns nil.
func WithTag(l Logger, key, value string) Logger {
	if l == nil {
		return nil
	}

	return &taggedLogger{next: l, tag: key + "=" + value}
}

func (l *taggedLogger) message(msg string) string {
	if msg == "" {
		return l.tag
	}

	return l.tag + " " + msg
}

// Info logs an informational message
func (l *taggedLogger) Info(msg string, args ...interface{}) {
	l.next.Info(l.message(msg), args...)
}

// Debug logs a debug message
func (l *taggedLogger) Debug(msg string, args ...interface{}) {
	l.next.Debug(l.message(msg), args...)
}

// Warn logs a warning message
func (l *taggedLogger) Warn(msg string, args ...interface{}) {
	l.next.Warn(l.message(msg), args...)
}

// Error logs an error message
func (l *taggedLogger) Error(msg string, args ...interface{}) {
	l.next.Error(l.message(msg), args...)
}
