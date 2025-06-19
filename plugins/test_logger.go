package plugins

import "log"

// TestLogger implements the Logger interface and writes to stdout
// This logger is useful for testing and can be imported by other packages
type TestLogger struct{}

// Ensure TestLogger implements the Logger interface
var _ Logger = (*TestLogger)(nil)

func (l *TestLogger) Info(msg string, args ...interface{}) {
	log.Printf("Remote Plugin::[INFO] %s", msg)
	if len(args) > 0 {
		log.Printf(" %v", args)
	}
}

func (l *TestLogger) Debug(msg string, args ...interface{}) {
	log.Printf("Remote Plugin::[DEBUG] %s", msg)
	if len(args) > 0 {
		log.Printf(" %v", args)
	}
}

func (l *TestLogger) Warn(msg string, args ...interface{}) {
	log.Printf("[WARN] %s", msg)
	if len(args) > 0 {
		log.Printf(" %v", args)
	}
}

func (l *TestLogger) Error(msg string, args ...interface{}) {
	log.Printf("Remote Plugin::[ERROR] %s", msg)
	if len(args) > 0 {
		log.Printf(" %v", args)
	}
}
