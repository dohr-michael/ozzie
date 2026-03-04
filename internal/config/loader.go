package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/marcozac/go-jsonc"
)

var envTemplateRe = regexp.MustCompile(`\$\{\{\s*\.Env\.(\w+)\s*\}\}`)

// DecryptFunc decrypts an ENC[age:...] value. Nil means no decryption.
type DecryptFunc func(string) (string, error)

// LoadOption configures Load behavior.
type LoadOption func(*loadOptions)

type loadOptions struct {
	decrypt DecryptFunc
}

// WithDecrypt adds a decryption function for ENC[age:...] values.
func WithDecrypt(fn DecryptFunc) LoadOption {
	return func(o *loadOptions) { o.decrypt = fn }
}

// Load reads a JSONC config file, strips comments, expands ${{ .Env.VAR }} templates,
// unmarshals it into Config, and applies defaults.
func Load(path string, opts ...LoadOption) (*Config, error) {
	var o loadOptions
	for _, opt := range opts {
		opt(&o)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	// Expand environment variable templates (before stripping, since templates are in strings)
	expanded := expandEnvTemplates(string(data), o.decrypt)

	// Strip JSONC comments and unmarshal
	var cfg Config
	if err := jsonc.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	applyDefaults(&cfg)
	return &cfg, nil
}

// expandEnvTemplates replaces ${{ .Env.VAR }} with the env var value,
// optionally decrypting ENC[age:...] blobs.
func expandEnvTemplates(s string, decrypt DecryptFunc) string {
	return envTemplateRe.ReplaceAllStringFunc(s, func(match string) string {
		parts := envTemplateRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		value := os.Getenv(parts[1])
		if decrypt != nil {
			if decrypted, err := decrypt(value); err == nil {
				return decrypted
			}
		}
		return value
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

	// Layered context defaults
	if cfg.LayeredContext.MaxArchives == 0 {
		cfg.LayeredContext.MaxArchives = 12
	}
	if cfg.LayeredContext.MaxRecentMessages == 0 {
		cfg.LayeredContext.MaxRecentMessages = 24
	}
	if cfg.LayeredContext.ArchiveChunkSize == 0 {
		cfg.LayeredContext.ArchiveChunkSize = 8
	}

	// MCP server defaults
	for name, srv := range cfg.MCP.Servers {
		if srv.Timeout <= 0 {
			srv.Timeout = 30000
			cfg.MCP.Servers[name] = srv
		}
	}

	// Default MaxConcurrent for providers
	for name, p := range cfg.Models.Providers {
		if p.MaxConcurrent <= 0 {
			p.MaxConcurrent = 1
			cfg.Models.Providers[name] = p
		}
	}
	// Auth resolution is deferred to models.ResolveAuth() at model init time.
	// Capability validation is deferred to gateway startup (avoids import cycle).
}
