package skills

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadSkillDir loads a skill from a directory containing SKILL.md and optional YAML files.
func LoadSkillDir(dir string) (*SkillMD, error) {
	skillPath := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md in %s: %w", dir, err)
	}

	skill, err := ParseSkillMD(data)
	if err != nil {
		return nil, fmt.Errorf("parse SKILL.md in %s: %w", dir, err)
	}
	skill.Dir = dir

	// Load optional workflow.yaml
	wfPath := filepath.Join(dir, "workflow.yaml")
	if wfData, err := os.ReadFile(wfPath); err == nil {
		var wf WorkflowDef
		if err := yaml.Unmarshal(wfData, &wf); err != nil {
			return nil, fmt.Errorf("parse workflow.yaml in %s: %w", dir, err)
		}
		skill.Workflow = &wf
	}

	// Load optional triggers.yaml
	trPath := filepath.Join(dir, "triggers.yaml")
	if trData, err := os.ReadFile(trPath); err == nil {
		var tr TriggersDef
		if err := yaml.Unmarshal(trData, &tr); err != nil {
			return nil, fmt.Errorf("parse triggers.yaml in %s: %w", dir, err)
		}
		skill.Triggers = &tr
	}

	if err := skill.Validate(); err != nil {
		return nil, err
	}

	return skill, nil
}

// ParseSkillMD parses a SKILL.md file with YAML frontmatter and Markdown body.
func ParseSkillMD(data []byte) (*SkillMD, error) {
	fm, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}

	// First parse as raw map to handle allowed-tools string format
	var raw map[string]interface{}
	if err := yaml.Unmarshal(fm, &raw); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	// Normalize allowed-tools: convert space-separated string to list before struct parse
	if val, ok := raw["allowed-tools"]; ok {
		if s, ok := val.(string); ok && s != "" {
			raw["allowed-tools"] = strings.Fields(s)
		}
	}

	// Re-marshal and unmarshal into struct
	normalized, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("normalize frontmatter: %w", err)
	}

	var skill SkillMD
	if err := yaml.Unmarshal(normalized, &skill); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	skill.Body = strings.TrimSpace(body)
	return &skill, nil
}

// splitFrontmatter separates YAML frontmatter from Markdown body.
// Frontmatter is delimited by --- lines.
func splitFrontmatter(data []byte) ([]byte, string, error) {
	content := bytes.TrimLeft(data, "\n\r\t ")
	if !bytes.HasPrefix(content, []byte("---")) {
		return nil, "", fmt.Errorf("SKILL.md must start with --- frontmatter delimiter")
	}

	// Find the closing ---
	rest := content[3:]
	rest = bytes.TrimLeft(rest, " \t")
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	idx := bytes.Index(rest, []byte("\n---"))
	if idx < 0 {
		return nil, "", fmt.Errorf("SKILL.md: missing closing --- frontmatter delimiter")
	}

	frontmatter := rest[:idx]
	body := rest[idx+4:] // skip \n---
	// Skip trailing whitespace on the --- line
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	} else if len(body) > 1 && body[0] == '\r' && body[1] == '\n' {
		body = body[2:]
	}

	return frontmatter, string(body), nil
}

