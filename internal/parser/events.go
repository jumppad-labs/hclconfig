package parser

import "time"

// ParserEvent represents an event that occurs during parser operations
type ParserEvent struct {
	Operation    string        // "create", "destroy", "update", "refresh", "changed", "validate"
	ResourceType string        // "resource.container"
	ResourceID   string        // "resource.container.web"
	Phase        string        // "start", "success", "error"
	Duration     time.Duration // only for success/error phases
	Error        error         // only for error phase
	Data         []byte        // serialized resource data
}

// fireParserEvent fires a parser event if the callback is configured
func fireParserEvent(options *ParserOptions, operation, resourceType, resourceID, phase string, duration time.Duration, err error, data []byte) {
	if options != nil && options.OnParserEvent != nil {
		event := ParserEvent{
			Operation:    operation,
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Phase:        phase,
			Duration:     duration,
			Error:        err,
			Data:         data,
		}
		options.OnParserEvent(event)
	}
}
