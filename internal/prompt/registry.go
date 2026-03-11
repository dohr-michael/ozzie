// Package prompt provides a central registry of prompt templates and an
// auditable composer for assembling system prompts at runtime.
package prompt

import (
	"fmt"
	"sort"
	"sync"
)

// Template is a named prompt fragment registered in the Registry.
type Template struct {
	ID   string // e.g. "persona.default", "instructions.agent"
	Name string // human-readable: "Default persona"
	Text string // raw content
}

// Registry holds named prompt templates for lookup and enumeration.
type Registry struct {
	mu        sync.RWMutex
	templates map[string]*Template
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{templates: make(map[string]*Template)}
}

// Register adds or replaces a template in the registry.
func (r *Registry) Register(id, name, text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templates[id] = &Template{ID: id, Name: name, Text: text}
}

// Get returns the template with the given ID, or nil if not found.
func (r *Registry) Get(id string) *Template {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.templates[id]
}

// MustGet returns the template or panics if not found.
func (r *Registry) MustGet(id string) *Template {
	t := r.Get(id)
	if t == nil {
		panic(fmt.Sprintf("prompt: template %q not registered", id))
	}
	return t
}

// All returns all templates sorted by ID.
func (r *Registry) All() []*Template {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Template, 0, len(r.templates))
	for _, t := range r.templates {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
