package plugins

import (
	"fmt"
)

// TestLogger implements the Logger interface and writes to stdout
// This logger is useful for testing and can be imported by other packages
type TestLogger struct{}

// Ensure TestLogger implements the Logger interface
var _ Logger = (*TestLogger)(nil)

func (l *TestLogger) Info(msg string, args ...interface{}) {
	fmt.Printf("[INFO] %s", msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}

func (l *TestLogger) Debug(msg string, args ...interface{}) {
	fmt.Printf("[DEBUG] %s", msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}

func (l *TestLogger) Warn(msg string, args ...interface{}) {
	fmt.Printf("[WARN] %s", msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}

func (l *TestLogger) Error(msg string, args ...interface{}) {
	fmt.Printf("[ERROR] %s", msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}