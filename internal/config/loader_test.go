package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	content := `{
	// This is a JSONC comment
	"gateway": {
		"host": "0.0.0.0",
		"port": 9999
	},
	"models": {
		"default": "claude",
		"providers": {
			"claude": {
				"driver": "anthropic",
				"model": "claude-sonnet-4-20250514",
				"auth": {
					"api_key": "${{ .Env.ANTHROPIC_API_KEY }}"
				},
				"max_tokens": 4096
			}
		}
	}
}`

	dir := t.TempDir()
	path := filepath.Join(dir, "config.jsonc")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ANTHROPIC_API_KEY", "test-key-123")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Gateway.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Gateway.Host)
	}
	if cfg.Gateway.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Gateway.Port)
	}
	if cfg.Models.Default != "claude" {
		t.Errorf("expected default claude, got %s", cfg.Models.Default)
	}

	p, ok := cfg.Models.Providers["claude"]
	if !ok {
		t.Fatal("expected claude provider")
	}
	if p.Auth.APIKey != "test-key-123" {
		t.Errorf("expected api_key test-key-123, got %s", p.Auth.APIKey)
	}
	if p.MaxTokens != 4096 {
		t.Errorf("expected max_tokens 4096, got %d", p.MaxTokens)
	}
}

func TestLoadDefaults(t *testing.T) {
	content := `{}`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.jsonc")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Gateway.Host != "127.0.0.1" {
		t.Errorf("expected default host 127.0.0.1, got %s", cfg.Gateway.Host)
	}
	if cfg.Gateway.Port != 18420 {
		t.Errorf("expected default port 18420, got %d", cfg.Gateway.Port)
	}
	if cfg.Events.BufferSize != 1024 {
		t.Errorf("expected default buffer 1024, got %d", cfg.Events.BufferSize)
	}
}

func TestLoadDefaults_CoordinatorDefaultLevel(t *testing.T) {
	content := `{}`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.jsonc")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Agent.Coordinator.DefaultLevel != "disabled" {
		t.Errorf("expected default_level 'disabled', got %q", cfg.Agent.Coordinator.DefaultLevel)
	}
	if cfg.Agent.Coordinator.MaxValidationRounds != 3 {
		t.Errorf("expected max_validation_rounds 3, got %d", cfg.Agent.Coordinator.MaxValidationRounds)
	}
}

func TestLoadDefaults_LogLevel(t *testing.T) {
	content := `{}`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.jsonc")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Events.LogLevel != "info" {
		t.Errorf("expected default log_level 'info', got %q", cfg.Events.LogLevel)
	}
}

func TestExpandEnvTemplates(t *testing.T) {
	t.Setenv("TEST_KEY", "my-secret")
	result := expandEnvTemplates(`{"key": "${{ .Env.TEST_KEY }}"}`)
	expected := `{"key": "my-secret"}`
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}
