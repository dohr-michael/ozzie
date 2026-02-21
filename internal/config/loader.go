package config

import (
	"fmt"
	"os"
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
	// Auth resolution is deferred to models.ResolveAuth() at model init time.
}
