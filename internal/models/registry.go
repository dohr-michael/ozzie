package models

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/model"

	"github.com/dohr-michael/ozzie/internal/config"
)

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
