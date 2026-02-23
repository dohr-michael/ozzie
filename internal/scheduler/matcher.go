package scheduler

import (
	"github.com/dohr-michael/ozzie/internal/events"
)

// MatchEvent returns true if the event matches the given trigger.
// Events emitted by the scheduler itself are always rejected to prevent loops.
func MatchEvent(e events.Event, trigger *EventTrigger) bool {
	if trigger == nil {
		return false
	}

	// Reject scheduler-originated events to prevent infinite loops
	if e.Source == events.SourceScheduler {
		return false
	}

	// Event type must match
	if string(e.Type) != trigger.Event {
		return false
	}

	// All filter key/value pairs must match in the payload
	for key, expected := range trigger.Filter {
		val, ok := e.Payload[key]
		if !ok {
			return false
		}
		strVal, ok := val.(string)
		if !ok || strVal != expected {
			return false
		}
	}

	return true
}
