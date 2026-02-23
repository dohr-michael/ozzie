package models

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/model"

	"github.com/dohr-michael/ozzie/internal/config"
)

// defaultContextWindows maps known model prefixes to their context window sizes.
var defaultContextWindows = map[string]int{
	"claude-opus-4":    200000,
	"claude-sonnet-4":  200000,
	"claude-sonnet-3":  200000,
	"claude-haiku-3":   200000,
	"gpt-4o":           128000,
	"gpt-4-turbo":      128000,
	"gpt-4":            8192,
	"gpt-3.5-turbo":    16385,
	"o1":               200000,
	"o3":               200000,
	"mistral-large":    128000,
	"mistral-small":    128000,
	"codestral":        256000,
	"open-mistral-nemo": 128000,
	"pixtral":          128000,
}

const fallbackContextWindow = 100000

// ProviderEntry holds a lazily-initialized model instance.
type ProviderEntry struct {
	Config config.ProviderConfig
	model  model.ToolCallingChatModel
	once   sync.Once
	err    error
}

// Registry manages named model providers with lazy initialization.
type Registry struct {
	mu          sync.RWMutex
	providers   map[string]*ProviderEntry
	defaultName string
}

// NewRegistry creates a model registry from config.
func NewRegistry(cfg config.ModelsConfig) *Registry {
	r := &Registry{
		providers:   make(map[string]*ProviderEntry),
		defaultName: cfg.Default,
	}

	for name, provCfg := range cfg.Providers {
		r.providers[name] = &ProviderEntry{Config: provCfg}
	}

	return r
}

// Get returns the named model, initializing it lazily.
func (r *Registry) Get(ctx context.Context, name string) (model.ToolCallingChatModel, error) {
	r.mu.RLock()
	entry, ok := r.providers[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("model provider %q not found", name)
	}

	entry.once.Do(func() {
		entry.model, entry.err = CreateModel(ctx, entry.Config)
	})

	return entry.model, entry.err
}

// Default returns the default model.
func (r *Registry) Default(ctx context.Context) (model.ToolCallingChatModel, error) {
	if r.defaultName == "" {
		return nil, fmt.Errorf("no default model configured")
	}
	return r.Get(ctx, r.defaultName)
}

// DefaultName returns the name of the default provider.
func (r *Registry) DefaultName() string {
	return r.defaultName
}

// DefaultContextWindow returns the context window size for the default provider.
func (r *Registry) DefaultContextWindow() int {
	return r.ContextWindow(r.defaultName)
}

// ContextWindow returns the context window size for the named provider.
func (r *Registry) ContextWindow(name string) int {
	r.mu.RLock()
	entry, ok := r.providers[name]
	r.mu.RUnlock()

	if !ok {
		return fallbackContextWindow
	}
	return resolveContextWindow(entry.Config)
}

// resolveContextWindow determines context window: explicit config > model prefix > driver default > fallback.
func resolveContextWindow(cfg config.ProviderConfig) int {
	if cfg.ContextWindow > 0 {
		return cfg.ContextWindow
	}

	// Match by model prefix
	for prefix, size := range defaultContextWindows {
		if strings.HasPrefix(cfg.Model, prefix) {
			return size
		}
	}

	// Driver-specific defaults
	if cfg.Driver == "ollama" {
		return 8192
	}

	return fallbackContextWindow
}
