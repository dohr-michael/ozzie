package config

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestReloader_Current(t *testing.T) {
	cfg := &Config{}
	cfg.Gateway.Port = 9999

	r := NewReloader("", "", cfg)
	got := r.Current()
	if got.Gateway.Port != 9999 {
		t.Errorf("Current().Gateway.Port = %d, want 9999", got.Gateway.Port)
	}
}

func TestReloader_Reload(t *testing.T) {
	dir := t.TempDir()
	dotenvPath := filepath.Join(dir, ".env")
	configPath := filepath.Join(dir, "config.jsonc")

	// Write initial .env
	if err := os.WriteFile(dotenvPath, []byte("MY_VAR=initial\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Write minimal config
	configContent := `{
		"gateway": {"host": "127.0.0.1", "port": 18420},
		"models": {"default": "test", "providers": {}},
		"events": {"buffer_size": 1024}
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	initial := &Config{}
	r := NewReloader(configPath, dotenvPath, initial)

	// Track listener invocations
	var callCount atomic.Int32
	r.OnReload(func(cfg *Config) {
		callCount.Add(1)
	})

	// Update .env
	if err := os.WriteFile(dotenvPath, []byte("MY_VAR=reloaded\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := r.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	if os.Getenv("MY_VAR") != "reloaded" {
		t.Errorf("MY_VAR = %q, want 'reloaded'", os.Getenv("MY_VAR"))
	}

	if callCount.Load() != 1 {
		t.Errorf("listener called %d times, want 1", callCount.Load())
	}

	// New config is available
	got := r.Current()
	if got == initial {
		t.Error("Current() still returns initial config after reload")
	}
}

func TestReloader_ReloadMissingDotenv(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.jsonc")
	dotenvPath := filepath.Join(dir, ".env") // does not exist

	configContent := `{"gateway": {"host": "127.0.0.1", "port": 18420}, "models": {"default": "", "providers": {}}, "events": {"buffer_size": 1024}}`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	initial := &Config{}
	r := NewReloader(configPath, dotenvPath, initial)

	// Should not error — missing .env is ok
	if err := r.Reload(); err != nil {
		t.Fatalf("Reload with missing .env: %v", err)
	}
}
