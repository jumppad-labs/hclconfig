package xcl

import (
	"time"

	"github.com/jumppad-labs/xcl/internal/parser"
)

// Event describes one step of the resource lifecycle during Apply or Destroy,
// such as a provider's Create or Destroy starting or succeeding for a resource
type Event struct {
	// Operation is the lifecycle operation, one of "parse", "create", "read",
	// "changed", "update" or "destroy". A parse event is fired for every block
	// as it is read from its file, before anything is created.
	Operation string

	// ResourceType is the "<type>.<name>" of the resource, i.e. "postgres.main"
	ResourceType string

	// ResourceID is the full path of the resource, i.e. "resource.postgres.main".
	// It is empty for a parse error that is not in a resource, i.e. a file
	// that is not valid syntax.
	ResourceID string

	// File is the file the resource was parsed from, set for parse events
	File string

	// Phase is "start" when the operation begins, then "success" or "error".
	// Parse events have no start, only a success or an error.
	Phase string

	// Duration is how long the operation took, set for success and error
	Duration time.Duration

	// Error is the reason the operation failed, set for error
	Error error

	// Data is the serialized resource, nil for builtin and registered types
	Data []byte
}

// EventHandler is called for every lifecycle event during Apply and Destroy,
// and for the parse events during Validate. Resources
// that do not depend on each other are processed concurrently, so a handler
// may be called from several goroutines at once.
type EventHandler func(Event)

// parserEventHandler adapts an EventHandler to the parser's event callback,
// it returns nil when handler is nil
func parserEventHandler(handler EventHandler) func(parser.ParserEvent) {
	if handler == nil {
		return nil
	}

	return func(e parser.ParserEvent) {
		handler(Event{
			Operation:    e.Operation,
			ResourceType: e.ResourceType,
			ResourceID:   e.ResourceID,
			File:         e.File,
			Phase:        e.Phase,
			Duration:     e.Duration,
			Error:        e.Error,
			Data:         e.Data,
		})
	}
}
