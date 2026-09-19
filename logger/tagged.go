package logger

// taggedLogger writes every message to next with its tag in front, so a
// message "create" tagged provider=postgres is written as
// "provider=postgres create". An event always leads the line: an "event"
// argument, or a message that already starts with event=<name>, is written
// before the tag, so "calling provider" with the argument event=create is
// written as "event=create provider=postgres calling provider". A message
// logged without an event is written with event=log, so every line has one
type taggedLogger struct {
	next Logger
	tag  string
}

// Ensure taggedLogger implements the Logger interface
var _ Logger = (*taggedLogger)(nil)

// WithTag returns a Logger that writes to l, starting every message with
// key=value. Tags added by nested calls read outermost first:
// WithTag(WithTag(l, "provider", "postgres"), "plugin", "example") writes
// "plugin=example provider=postgres <message>". An event is written before
// every tag: "event=create plugin=example provider=postgres <message>". A nil
// l returns nil.
func WithTag(l Logger, key, value string) Logger {
	if l == nil {
		return nil
	}

	return &taggedLogger{next: l, tag: key + "=" + value}
}

// message returns msg with the event and the tag in front, and the remaining
// arguments
func (l *taggedLogger) message(msg string, args []interface{}) (string, []interface{}) {
	event, msg, args := splitEvent(msg, args)
	return joinMessage(event, l.tag, msg), args
}

// Info logs an informational message
func (l *taggedLogger) Info(msg string, args ...interface{}) {
	msg, args = l.message(msg, args)
	l.next.Info(msg, args...)
}

// Debug logs a debug message
func (l *taggedLogger) Debug(msg string, args ...interface{}) {
	msg, args = l.message(msg, args)
	l.next.Debug(msg, args...)
}

// Warn logs a warning message
func (l *taggedLogger) Warn(msg string, args ...interface{}) {
	msg, args = l.message(msg, args)
	l.next.Warn(msg, args...)
}

// Error logs an error message
func (l *taggedLogger) Error(msg string, args ...interface{}) {
	msg, args = l.message(msg, args)
	l.next.Error(msg, args...)
}
