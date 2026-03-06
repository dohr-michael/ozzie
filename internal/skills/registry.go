package skills

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
)

// Registry manages loaded skill definitions.
type Registry struct {
	skills map[string]*SkillMD
}

// NewRegistry creates a new skill registry.
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]*SkillMD),
	}
}

// LoadDir scans a directory for subdirectories containing SKILL.md files.
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
		if !entry.IsDir() {
			continue
		}

		subDir := filepath.Join(dir, entry.Name())
		skillPath := filepath.Join(subDir, "SKILL.md")
		if _, err := os.Stat(skillPath); err != nil {
			continue // no SKILL.md in this subdirectory
		}

		skill, err := LoadSkillDir(subDir)
		if err != nil {
			slog.Warn("failed to load skill", "dir", subDir, "error", err)
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
func (r *Registry) Register(skill *SkillMD) error {
	if _, exists := r.skills[skill.Name]; exists {
		return fmt.Errorf("skill %q already registered", skill.Name)
	}
	r.skills[skill.Name] = skill
	return nil
}

// Get returns the skill with the given name, or nil.
func (r *Registry) Get(name string) *SkillMD {
	return r.skills[name]
}

// All returns all registered skills sorted by name.
func (r *Registry) All() []*SkillMD {
	result := make([]*SkillMD, 0, len(r.skills))
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

// Catalog returns a map of skill name → description for progressive disclosure.
func (r *Registry) Catalog() map[string]string {
	result := make(map[string]string, len(r.skills))
	for name, s := range r.skills {
		result[name] = s.Description
	}
	return result
}

// SkillBody implements the SkillCatalog interface for plugins.
func (r *Registry) SkillBody(name string) (body string, allowedTools []string, hasWorkflow bool, dir string, err error) {
	s := r.Get(name)
	if s == nil {
		return "", nil, false, "", fmt.Errorf("skill not found: %s", name)
	}
	return s.Body, s.AllowedTools, s.HasWorkflow(), s.Dir, nil
}
