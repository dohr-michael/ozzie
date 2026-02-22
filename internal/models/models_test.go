package models

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/dohr-michael/ozzie/internal/config"
)

func TestResolveAuth_DirectAPIKey(t *testing.T) {
	cfg := config.ProviderConfig{
		Driver: "anthropic",
		Auth:   config.AuthConfig{APIKey: "sk-ant-test-123"},
	}
	auth, err := ResolveAuth(cfg)
	if err != nil {
		t.Fatalf("ResolveAuth: %v", err)
	}
	if auth.Kind != AuthAPIKey {
		t.Fatalf("expected AuthAPIKey, got %d", auth.Kind)
	}
	if auth.Value != "sk-ant-test-123" {
		t.Fatalf("expected value %q, got %q", "sk-ant-test-123", auth.Value)
	}
}

func TestResolveAuth_DirectBearerToken(t *testing.T) {
	cfg := config.ProviderConfig{
		Driver: "anthropic",
		Auth: config.AuthConfig{
			APIKey: "sk-ant-test-123",
			Token:  "bearer-token-xyz",
		},
	}
	auth, err := ResolveAuth(cfg)
	if err != nil {
		t.Fatalf("ResolveAuth: %v", err)
	}
	// Bearer token takes priority over API key
	if auth.Kind != AuthBearerToken {
		t.Fatalf("expected AuthBearerToken, got %d", auth.Kind)
	}
	if auth.Value != "bearer-token-xyz" {
		t.Fatalf("expected value %q, got %q", "bearer-token-xyz", auth.Value)
	}
}

func TestResolveAuth_EnvVarSyntax(t *testing.T) {
	t.Setenv("MY_CUSTOM_KEY", "custom-api-key-value")

	cfg := config.ProviderConfig{
		Driver: "anthropic",
		Auth:   config.AuthConfig{APIKey: "${MY_CUSTOM_KEY}"},
	}
	auth, err := ResolveAuth(cfg)
	if err != nil {
		t.Fatalf("ResolveAuth: %v", err)
	}
	if auth.Kind != AuthAPIKey {
		t.Fatalf("expected AuthAPIKey, got %d", auth.Kind)
	}
	if auth.Value != "custom-api-key-value" {
		t.Fatalf("expected value %q, got %q", "custom-api-key-value", auth.Value)
	}
}

func TestResolveAuth_FallbackAnthropicEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "env-anthropic-key")

	cfg := config.ProviderConfig{Driver: "anthropic"}
	auth, err := ResolveAuth(cfg)
	if err != nil {
		t.Fatalf("ResolveAuth: %v", err)
	}
	if auth.Kind != AuthAPIKey {
		t.Fatalf("expected AuthAPIKey, got %d", auth.Kind)
	}
	if auth.Value != "env-anthropic-key" {
		t.Fatalf("expected value %q, got %q", "env-anthropic-key", auth.Value)
	}
}

func TestResolveAuth_FallbackOpenAIEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-openai-key")

	cfg := config.ProviderConfig{Driver: "openai"}
	auth, err := ResolveAuth(cfg)
	if err != nil {
		t.Fatalf("ResolveAuth: %v", err)
	}
	if auth.Kind != AuthAPIKey {
		t.Fatalf("expected AuthAPIKey, got %d", auth.Kind)
	}
	if auth.Value != "env-openai-key" {
		t.Fatalf("expected value %q, got %q", "env-openai-key", auth.Value)
	}
}

func TestResolveAuth_UnknownDriver(t *testing.T) {
	// Clear env to ensure no fallback works
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")

	cfg := config.ProviderConfig{Driver: "mistral"}
	_, err := ResolveAuth(cfg)
	if err == nil {
		t.Fatal("expected error for unknown driver")
	}
	if !strings.Contains(err.Error(), "unknown driver") {
		t.Fatalf("expected 'unknown driver' error, got %v", err)
	}
}

func TestResolveAuth_NothingSet(t *testing.T) {
	// Clear all env vars
	t.Setenv("ANTHROPIC_API_KEY", "")
	os.Unsetenv("ANTHROPIC_API_KEY")

	cfg := config.ProviderConfig{Driver: "anthropic"}
	_, err := ResolveAuth(cfg)
	if err == nil {
		t.Fatal("expected error when no auth is available")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY not set") {
		t.Fatalf("expected 'ANTHROPIC_API_KEY not set' error, got %v", err)
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	cfg := config.ModelsConfig{
		Default:   "main",
		Providers: map[string]config.ProviderConfig{},
	}
	reg := NewRegistry(cfg)

	_, err := reg.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got %v", err)
	}
}

func TestRegistry_DefaultName(t *testing.T) {
	cfg := config.ModelsConfig{
		Default: "claude-main",
		Providers: map[string]config.ProviderConfig{
			"claude-main": {Driver: "anthropic"},
		},
	}
	reg := NewRegistry(cfg)

	if reg.DefaultName() != "claude-main" {
		t.Fatalf("expected default name %q, got %q", "claude-main", reg.DefaultName())
	}
}

func TestCreateModel_UnknownDriver(t *testing.T) {
	cfg := config.ProviderConfig{Driver: "unknown-driver"}
	_, err := CreateModel(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for unknown driver")
	}
	if !strings.Contains(err.Error(), "unknown driver") {
		t.Fatalf("expected 'unknown driver' error, got %v", err)
	}
}
