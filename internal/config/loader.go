package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/marcozac/go-jsonc"
)

var envTemplateRe = regexp.MustCompile(`\$\{\{\s*\.Env\.(\w+)\s*\}\}`)

// Load reads a JSONC config file, strips comments, expands ${{ .Env.VAR }} templates,
// unmarshals it into Config, and applies defaults.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	// Expand environment variable templates (before stripping, since templates are in strings)
	expanded := expandEnvTemplates(string(data))

	// Strip JSONC comments and unmarshal
	var cfg Config
	if err := jsonc.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	applyDefaults(&cfg)
	return &cfg, nil
}

// expandEnvTemplates replaces ${{ .Env.VAR }} with the env var value.
func expandEnvTemplates(s string) string {
	return envTemplateRe.ReplaceAllStringFunc(s, func(match string) string {
		parts := envTemplateRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return os.Getenv(parts[1])
	})
}

// applyDefaults fills in zero-value fields with sensible defaults.
func applyDefaults(cfg *Config) {
	if cfg.Gateway.Host == "" {
		cfg.Gateway.Host = "127.0.0.1"
	}
	if cfg.Gateway.Port == 0 {
		cfg.Gateway.Port = 18420
	}
	if cfg.Events.BufferSize == 0 {
		cfg.Events.BufferSize = 1024
	}
	if cfg.Events.LogLevel == "" {
		cfg.Events.LogLevel = "info"
	}
	if len(cfg.Skills.Dirs) == 0 {
		cfg.Skills.Dirs = []string{filepath.Join(OzziePath(), "skills")}
	}
	if cfg.Agent.Coordinator.DefaultLevel == "" {
		cfg.Agent.Coordinator.DefaultLevel = "disabled"
	}
	if cfg.Agent.Coordinator.MaxValidationRounds == 0 {
		cfg.Agent.Coordinator.MaxValidationRounds = 3
	}
	// Runtime environment
	if cfg.Runtime.Environment == "" {
		if v := os.Getenv("OZZIE_RUNTIME"); v != "" {
			cfg.Runtime.Environment = v
		} else {
			cfg.Runtime.Environment = "local"
		}
	}
	if cfg.Runtime.SystemToolsFile == "" {
		cfg.Runtime.SystemToolsFile = "/etc/ozzie/system-tools.json"
	}

	// Default MaxConcurrent for providers
	for name, p := range cfg.Models.Providers {
		if p.MaxConcurrent <= 0 {
			p.MaxConcurrent = 1
			cfg.Models.Providers[name] = p
		}
	}
	// Auth resolution is deferred to models.ResolveAuth() at model init time.
}
