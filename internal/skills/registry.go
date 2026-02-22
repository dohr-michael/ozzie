package skills

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Registry manages loaded skill definitions.
type Registry struct {
	skills map[string]*Skill
}

// NewRegistry creates a new skill registry.
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]*Skill),
	}
}

// LoadDir scans a directory for *.jsonc skill files and loads them.
func (r *Registry) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("skills directory not found, skipping", "dir", dir)
			return nil
		}
		return fmt.Errorf("read skills dir %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".jsonc") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		skill, err := LoadSkill(path)
		if err != nil {
			slog.Warn("failed to load skill", "path", path, "error", err)
			continue
		}

		if err := r.Register(skill); err != nil {
			slog.Warn("failed to register skill", "name", skill.Name, "error", err)
			continue
		}
	}

	return nil
}

// Register adds a skill to the registry.
func (r *Registry) Register(skill *Skill) error {
	if _, exists := r.skills[skill.Name]; exists {
		return fmt.Errorf("skill %q already registered", skill.Name)
	}
	r.skills[skill.Name] = skill
	return nil
}

// Get returns the skill with the given name, or nil.
func (r *Registry) Get(name string) *Skill {
	return r.skills[name]
}

// All returns all registered skills sorted by name.
func (r *Registry) All() []*Skill {
	result := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Names returns all registered skill names sorted alphabetically.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
