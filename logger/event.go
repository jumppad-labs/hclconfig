package logger

import (
	"fmt"
	"strings"
)

// eventKey is the argument that always leads a logged line
const eventKey = "event"

// defaultEvent is the event written for a message logged without one
const defaultEvent = "log"

// splitEvent takes the event out of a message so it can lead the line. The
// event is an "event" argument, which is removed from args, or an
// event=<name> that already leads msg because a tagged logger put it there.
// A message logged without an event gets event=log. It returns the event as
// "event=<name>", the rest of msg and the remaining arguments.
func splitEvent(msg string, args []interface{}) (string, string, []interface{}) {
	for i := 0; i+1 < len(args); i += 2 {
		if key, ok := args[i].(string); ok && key == eventKey {
			event := fmt.Sprintf("%s=%v", eventKey, args[i+1])

			remaining := make([]interface{}, 0, len(args)-2)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+2:]...)

			return event, msg, remaining
		}
	}

	if strings.HasPrefix(msg, eventKey+"=") {
		event, rest, _ := strings.Cut(msg, " ")
		return event, rest, args
	}

	return eventKey + "=" + defaultEvent, msg, args
}

// joinMessage joins the non-empty parts of a message with spaces
func joinMessage(parts ...string) string {
	nonEmpty := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}

	return strings.Join(nonEmpty, " ")
}
