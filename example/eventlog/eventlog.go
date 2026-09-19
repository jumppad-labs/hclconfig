// Package eventlog logs the events xcl fires while it parses and applies a
// configuration. The configonly and plugin examples share it, so both log
// their events the same way.
package eventlog

import (
	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/logger"
)

// Handler returns an event handler that logs each event to log, i.e.
// "event=create resource=resource.postgres.main phase=start".
//
// A parse event is fired as each block is read, with the file it was read
// from. The other events arrive as xcl calls the providers: a start before
// each call, then a success or an error, with how long the call took, when it
// returns. Blocks with no provider, such as variables and registered types,
// only get a success.
//
// A failed operation is logged at error, everything else at debug.
func Handler(log logger.Logger) xcl.EventHandler {
	return func(e xcl.Event) {
		if e.Operation == "parse" {
			if e.Phase == "error" {
				log.Error("", "event", e.Operation, "resource", e.ResourceID, "file", e.File, "phase", e.Phase, "error", e.Error)
				return
			}

			log.Debug("", "event", e.Operation, "resource", e.ResourceID, "file", e.File, "phase", e.Phase)
			return
		}

		switch e.Phase {
		case "error":
			log.Error("", "event", e.Operation, "resource", e.ResourceID, "phase", e.Phase, "duration", e.Duration, "error", e.Error)
		case "start":
			log.Debug("", "event", e.Operation, "resource", e.ResourceID, "phase", e.Phase)
		default:
			log.Debug("", "event", e.Operation, "resource", e.ResourceID, "phase", e.Phase, "duration", e.Duration)
		}
	}
}
