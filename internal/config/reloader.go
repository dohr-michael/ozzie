package config

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
)

// Reloader provides hot config reload with atomic swap and listener notification.
type Reloader struct {
	configPath string
	dotenvPath string
	current    atomic.Pointer[Config]
	mu         sync.Mutex       // serializes reload
	listeners  []func(*Config)
}

// NewReloader creates a Reloader with the given initial config.
func NewReloader(configPath, dotenvPath string, initial *Config) *Reloader {
	r := &Reloader{
		configPath: configPath,
		dotenvPath: dotenvPath,
	}
	r.current.Store(initial)
	return r
}

// Current returns the current config (lock-free atomic read).
func (r *Reloader) Current() *Config {
	return r.current.Load()
}

// OnReload registers a callback invoked after successful reload.
func (r *Reloader) OnReload(fn func(*Config)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listeners = append(r.listeners, fn)
}

// Reload re-reads the .env file, reloads the config, and notifies listeners.
func (r *Reloader) Reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Reload .env (override mode)
	if err := ReloadDotenv(r.dotenvPath); err != nil {
		return fmt.Errorf("reload dotenv: %w", err)
	}

	// Reload config (re-expands env templates)
	cfg, err := Load(r.configPath)
	if err != nil {
		return fmt.Errorf("reload config: %w", err)
	}

	r.current.Store(cfg)
	slog.Info("config reloaded")

	for _, fn := range r.listeners {
		fn(cfg)
	}
	return nil
}
