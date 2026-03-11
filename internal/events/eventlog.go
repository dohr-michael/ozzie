package events

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// EventLogger persists bus events to JSONL files organized by session.
type EventLogger struct {
	dir         string
	bus         EventBus
	unsubscribe func()
}

// NewEventLogger creates an EventLogger that subscribes to all bus events
// and writes them as JSONL to dir, one file per session.
func NewEventLogger(dir string, bus EventBus) *EventLogger {
	el := &EventLogger{
		dir: dir,
		bus: bus,
	}
	el.unsubscribe = bus.Subscribe(el.handleEvent)
	return el
}

// Close unsubscribes the logger from the event bus.
func (el *EventLogger) Close() {
	if el.unsubscribe != nil {
		el.unsubscribe()
	}
}

func (el *EventLogger) handleEvent(e Event) {
	// Filter out stream deltas — too noisy, redundant with assistant.message.
	if e.Type == EventAssistantStream {
		return
	}
	_ = el.writeEvent(e)
}

func (el *EventLogger) writeEvent(e Event) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	path := el.logPath(e.SessionID)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

func (el *EventLogger) logPath(sessionID string) string {
	if sessionID == "" {
		return filepath.Join(el.dir, "_global.jsonl")
	}
	return filepath.Join(el.dir, sessionID+".jsonl")
}
