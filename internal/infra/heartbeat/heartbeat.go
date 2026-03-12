// Package heartbeat provides liveness detection for the Ozzie gateway.
package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Status represents the liveness state of the gateway.
type Status string

const (
	StatusAlive Status = "alive"
	StatusStale Status = "stale"
	StatusDead  Status = "dead"
)

// Heartbeat is the data written to the heartbeat file.
type Heartbeat struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    string    `json:"uptime"`
}

// Writer periodically writes a heartbeat file to disk.
type Writer struct {
	path     string
	interval time.Duration
	started  time.Time

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewWriter creates a heartbeat writer that writes to path every 30s.
func NewWriter(path string) *Writer {
	return &Writer{
		path:     path,
		interval: 30 * time.Second,
	}
}

// Start begins writing heartbeat files in a background goroutine.
func (w *Writer) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cancel != nil {
		return // already running
	}

	w.started = time.Now()
	w.done = make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel

	// Write initial heartbeat immediately
	w.write()

	go func() {
		defer close(w.done)
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.write()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop stops writing and removes the heartbeat file.
func (w *Writer) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cancel == nil {
		return
	}

	w.cancel()
	<-w.done
	w.cancel = nil

	os.Remove(w.path)
}

func (w *Writer) write() {
	hb := Heartbeat{
		PID:       os.Getpid(),
		StartedAt: w.started,
		Timestamp: time.Now(),
		Uptime:    time.Since(w.started).Truncate(time.Second).String(),
	}

	data, err := json.MarshalIndent(hb, "", "  ")
	if err != nil {
		return
	}

	// Atomic write: tmp + rename
	tmp := w.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	os.Rename(tmp, w.path)
}

// Check reads a heartbeat file and returns the liveness status.
// maxAge determines how old a heartbeat can be before it's considered stale.
func Check(path string, maxAge time.Duration) (Status, *Heartbeat, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return StatusDead, nil, nil
		}
		return StatusDead, nil, fmt.Errorf("read heartbeat: %w", err)
	}

	var hb Heartbeat
	if err := json.Unmarshal(data, &hb); err != nil {
		return StatusDead, nil, fmt.Errorf("unmarshal heartbeat: %w", err)
	}

	age := time.Since(hb.Timestamp)
	if age > maxAge {
		return StatusStale, &hb, nil
	}

	return StatusAlive, &hb, nil
}
