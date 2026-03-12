package heartbeat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteReadCycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "heartbeat.json")

	w := NewWriter(path)
	w.Start()
	defer w.Stop()

	// Give writer time to write initial heartbeat
	time.Sleep(100 * time.Millisecond)

	status, hb, err := Check(path, 2*time.Minute)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if status != StatusAlive {
		t.Errorf("expected alive, got %s", status)
	}
	if hb == nil {
		t.Fatal("expected heartbeat, got nil")
	}
	if hb.PID != os.Getpid() {
		t.Errorf("PID: got %d, want %d", hb.PID, os.Getpid())
	}
	if hb.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
}

func TestStaleDetection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "heartbeat.json")

	// Write a heartbeat file with an old timestamp directly
	old := Heartbeat{
		PID:       os.Getpid(),
		StartedAt: time.Now().Add(-2 * time.Hour),
		Timestamp: time.Now().Add(-1 * time.Hour),
		Uptime:    "1h0m0s",
	}
	data, _ := json.Marshal(old)
	os.WriteFile(path, data, 0o644)

	// Check with maxAge shorter than the timestamp age
	status, hb, err := Check(path, 30*time.Minute)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if status != StatusStale {
		t.Errorf("expected stale, got %s", status)
	}
	if hb == nil {
		t.Fatal("expected heartbeat, got nil")
	}
}

func TestDeadDetection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "heartbeat.json")

	status, hb, err := Check(path, 2*time.Minute)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if status != StatusDead {
		t.Errorf("expected dead, got %s", status)
	}
	if hb != nil {
		t.Errorf("expected nil heartbeat, got %+v", hb)
	}
}

func TestStopRemovesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "heartbeat.json")

	w := NewWriter(path)
	w.Start()
	time.Sleep(100 * time.Millisecond)
	w.Stop()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected heartbeat file to be removed after Stop")
	}
}
