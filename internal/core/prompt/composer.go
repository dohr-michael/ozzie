package prompt

import (
	"log/slog"
	"strings"
)

// Section is a single block in a composed prompt.
type Section struct {
	TemplateID string // non-empty when sourced from a registered template
	Label      string // human-readable label (e.g. "Agent Instructions")
	Content    string
}

// Composer assembles prompt sections and logs the resulting manifest.
type Composer struct {
	sections []Section
}

// NewComposer creates an empty Composer.
func NewComposer() *Composer {
	return &Composer{}
}

// UseTemplate appends a section backed by a registered template.
func (c *Composer) UseTemplate(t *Template) *Composer {
	c.sections = append(c.sections, Section{
		TemplateID: t.ID,
		Label:      t.Name,
		Content:    t.Text,
	})
	return c
}

// AddSection appends a free-form section (not backed by a template).
func (c *Composer) AddSection(label, content string) *Composer {
	if content == "" {
		return c
	}
	c.sections = append(c.sections, Section{
		Label:   label,
		Content: content,
	})
	return c
}

// String joins all sections with double newlines.
func (c *Composer) String() string {
	parts := make([]string, len(c.sections))
	for i, s := range c.sections {
		parts[i] = s.Content
	}
	return strings.Join(parts, "\n\n")
}

// Sections returns a copy of the section list for inspection.
func (c *Composer) Sections() []Section {
	out := make([]Section, len(c.sections))
	copy(out, c.sections)
	return out
}

// LogManifest emits a structured slog.Debug entry listing every section.
func (c *Composer) LogManifest(msg string) {
	type entry struct {
		ID    string `json:"id,omitempty"`
		Label string `json:"label"`
		Len   int    `json:"len"`
	}
	entries := make([]entry, len(c.sections))
	for i, s := range c.sections {
		entries[i] = entry{ID: s.TemplateID, Label: s.Label, Len: len(s.Content)}
	}
	slog.Debug(msg, "sections", entries)
}
