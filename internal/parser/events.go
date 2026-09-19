package parser

import "time"

// ParserEvent represents an event that occurs during parser operations
type ParserEvent struct {
	Operation    string        // "parse", "create", "read", "changed", "update", "destroy"
	ResourceType string        // "<type>.<name>", e.g. "container.web"
	ResourceID   string        // "resource.container.web", empty for a parse error that is not in a resource
	File         string        // the file the resource was parsed from, only for parse
	Phase        string        // "start", "success", "error"
	Duration     time.Duration // only for success/error phases
	Error        error         // only for error phase
	Data         []byte        // serialized resource data, nil for builtin types
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

// fireParseEvent fires a parse event for a block read from file, a success
// when err is nil, otherwise an error. resourceType and resourceID are empty
// when the problem can not be tied to a resource, i.e. a file that is not
// valid syntax.
func fireParseEvent(options *ParserOptions, resourceType, resourceID, file string, err error) {
	if options == nil || options.OnParserEvent == nil {
		return
	}

	phase := "success"
	if err != nil {
		phase = "error"
	}

	options.OnParserEvent(ParserEvent{
		Operation:    "parse",
		ResourceType: resourceType,
		ResourceID:   resourceID,
		File:         file,
		Phase:        phase,
		Error:        err,
	})
}
