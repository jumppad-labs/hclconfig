package logger

import (
	"testing"
)

func TestTestLogger_BuffersLogsAndOutputsOnFailure(t *testing.T) {
	// Create a test logger
	logger := NewTestLogger(t)
	
	// Log some messages
	logger.Info("This is an info message", "key", "value")
	logger.Debug("This is a debug message")
	logger.Warn("This is a warning message", "warning", "level")
	logger.Error("This is an error message", "error", "critical")
	
	// Since this test passes, logs should NOT be output
	// (This is the normal behavior we want)
}

func TestTestLogger_OutputsLogsOnFailure(t *testing.T) {
	// Create a test logger
	logger := NewTestLogger(t)
	
	// Log some messages
	logger.Info("This info should appear on failure", "test", "failing")
	logger.Debug("This debug should appear on failure")
	logger.Warn("This warning should appear on failure", "reason", "test_failure")
	logger.Error("This error should appear on failure", "severity", "high")
	
	// Force the test to fail so we can see the logs
	// Comment out the next line to see the difference
	// t.Fail()
}

func TestTestLogger_ThreadSafety(t *testing.T) {
	logger := NewTestLogger(t)
	
	// Test concurrent logging
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			logger.Info("Concurrent message", "goroutine", id)
			logger.Debug("Debug from goroutine", "id", id)
			done <- true
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// This test should pass, so no logs should be output
}

func TestTestLogger_FormatsArgsCorrectly(t *testing.T) {
	logger := NewTestLogger(t)
	
	// Test different argument patterns
	logger.Info("Message with pairs", "key1", "value1", "key2", "value2")
	logger.Debug("Message with odd args", "key1", "value1", "single")
	logger.Warn("Message with no args")
	logger.Error("Message with mixed types", "string", "text", "number", 42, "bool", true)
	
	// Check that our logger has buffered the entries
	if len(logger.buffer) != 4 {
		t.Errorf("Expected 4 buffered entries, got %d", len(logger.buffer))
	}
	
	// Check that the first entry has the right message
	if logger.buffer[0].message != "Message with pairs" {
		t.Errorf("Expected first message to be 'Message with pairs', got '%s'", logger.buffer[0].message)
	}
	
	// Check that args are properly stored
	if len(logger.buffer[0].args) != 4 {
		t.Errorf("Expected 4 args for first message, got %d", len(logger.buffer[0].args))
	}
}