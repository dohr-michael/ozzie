package plugins

import (
	"encoding/json"
	"testing"

	"github.com/dohr-michael/ozzie/internal/config"
)

// --- ResolveCapabilities tests ---

func TestResolveCapabilities_DenyByDefault(t *testing.T) {
	// HTTP requested but no auth → hosts empty
	caps := PluginCapabilities{HTTP: true}
	resolved := ResolveCapabilities(caps, nil, ResourceLimits{})

	if resolved.HTTP != nil {
		t.Errorf("HTTP = %v, want nil (deny-by-default)", resolved.HTTP)
	}
}

func TestResolveCapabilities_WithAuth(t *testing.T) {
	caps := PluginCapabilities{HTTP: true}
	auth := &PluginAuthorization{
		HTTP: &HTTPAuth{AllowedHosts: []string{"example.com", "api.example.com"}},
	}
	resolved := ResolveCapabilities(caps, auth, ResourceLimits{})

	if resolved.HTTP == nil {
		t.Fatal("HTTP = nil, want non-nil")
	}
	if len(resolved.HTTP.AllowedHosts) != 2 {
		t.Errorf("AllowedHosts len = %d, want 2", len(resolved.HTTP.AllowedHosts))
	}
	if resolved.HTTP.AllowedHosts[0] != "example.com" {
		t.Errorf("AllowedHosts[0] = %q, want %q", resolved.HTTP.AllowedHosts[0], "example.com")
	}
}

func TestResolveCapabilities_DenyOverride(t *testing.T) {
	caps := PluginCapabilities{Exec: true, KV: true, HTTP: true}
	auth := &PluginAuthorization{
		Deny: []string{"exec", "http"},
		HTTP: &HTTPAuth{AllowedHosts: []string{"example.com"}},
	}
	resolved := ResolveCapabilities(caps, auth, ResourceLimits{})

	if resolved.Exec {
		t.Error("Exec = true, want false (denied)")
	}
	if !resolved.KV {
		t.Error("KV = false, want true (not denied)")
	}
	if resolved.HTTP != nil {
		t.Errorf("HTTP = %v, want nil (denied)", resolved.HTTP)
	}
}

func TestResolveCapabilities_ReadOnlyCannotUpgrade(t *testing.T) {
	// Plugin declares read_only=true, auth tries read-write → stays read_only
	caps := PluginCapabilities{
		Filesystem: &FSCapabilityIntent{ReadOnly: true},
	}
	auth := &PluginAuthorization{
		Filesystem: &FSAuth{
			AllowedPaths: map[string]string{".": "/"},
			ReadOnly:     false,
		},
	}
	resolved := ResolveCapabilities(caps, auth, ResourceLimits{})

	if resolved.Filesystem == nil {
		t.Fatal("Filesystem = nil, want non-nil")
	}
	if !resolved.Filesystem.ReadOnly {
		t.Error("Filesystem.ReadOnly = false, want true (cannot upgrade from plugin's read_only)")
	}
	if len(resolved.Filesystem.AllowedPaths) != 1 {
		t.Errorf("AllowedPaths len = %d, want 1", len(resolved.Filesystem.AllowedPaths))
	}
}

func TestResolveCapabilities_AuthCanRestrictToReadOnly(t *testing.T) {
	// Plugin declares read-write, auth restricts to read-only
	caps := PluginCapabilities{
		Filesystem: &FSCapabilityIntent{ReadOnly: false},
	}
	auth := &PluginAuthorization{
		Filesystem: &FSAuth{
			AllowedPaths: map[string]string{".": "/"},
			ReadOnly:     true,
		},
	}
	resolved := ResolveCapabilities(caps, auth, ResourceLimits{})

	if resolved.Filesystem == nil {
		t.Fatal("Filesystem = nil, want non-nil")
	}
	if !resolved.Filesystem.ReadOnly {
		t.Error("Filesystem.ReadOnly = false, want true (auth restricted to read-only)")
	}
}

func TestResolveCapabilities_SecretIntersection(t *testing.T) {
	caps := PluginCapabilities{
		Secrets: []string{"a", "b", "c"},
	}
	auth := &PluginAuthorization{
		Secrets: &SecretsAuth{Allowed: []string{"b", "c", "d"}},
	}
	resolved := ResolveCapabilities(caps, auth, ResourceLimits{})

	if len(resolved.Secrets) != 2 {
		t.Fatalf("Secrets len = %d, want 2", len(resolved.Secrets))
	}
	// Order follows caps order
	if resolved.Secrets[0] != "b" || resolved.Secrets[1] != "c" {
		t.Errorf("Secrets = %v, want [b c]", resolved.Secrets)
	}
}

func TestResolveCapabilities_SecretsNoAuth(t *testing.T) {
	// Secrets requested but no auth → empty
	caps := PluginCapabilities{
		Secrets: []string{"a", "b"},
	}
	resolved := ResolveCapabilities(caps, nil, ResourceLimits{})

	if len(resolved.Secrets) != 0 {
		t.Errorf("Secrets = %v, want empty (deny-by-default)", resolved.Secrets)
	}
}

func TestResolveCapabilities_ResourceLimits(t *testing.T) {
	caps := PluginCapabilities{}
	limits := ResourceLimits{
		Memory:  &MemoryLimit{MaxPages: 16},
		Timeout: 5000,
	}
	resolved := ResolveCapabilities(caps, nil, limits)

	if resolved.Resources.Memory == nil || resolved.Resources.Memory.MaxPages != 16 {
		t.Errorf("Resources.Memory.MaxPages = %v, want 16", resolved.Resources.Memory)
	}
	if resolved.Resources.Timeout != 5000 {
		t.Errorf("Resources.Timeout = %d, want 5000", resolved.Resources.Timeout)
	}
}

func TestResolveCapabilities_ResourceOverrideFromAuth(t *testing.T) {
	caps := PluginCapabilities{}
	limits := ResourceLimits{
		Memory:  &MemoryLimit{MaxPages: 16},
		Timeout: 5000,
	}
	auth := &PluginAuthorization{
		Resources: &ResourceLimits{
			Memory:  &MemoryLimit{MaxPages: 8},
			Timeout: 3000,
		},
	}
	resolved := ResolveCapabilities(caps, auth, limits)

	if resolved.Resources.Memory.MaxPages != 8 {
		t.Errorf("Resources.Memory.MaxPages = %d, want 8 (overridden by auth)", resolved.Resources.Memory.MaxPages)
	}
	if resolved.Resources.Timeout != 3000 {
		t.Errorf("Resources.Timeout = %d, want 3000 (overridden by auth)", resolved.Resources.Timeout)
	}
}

func TestResolveCapabilities_FilesystemNoAuth(t *testing.T) {
	// Filesystem requested but no auth → no paths
	caps := PluginCapabilities{
		Filesystem: &FSCapabilityIntent{ReadOnly: false},
	}
	resolved := ResolveCapabilities(caps, nil, ResourceLimits{})

	if resolved.Filesystem == nil {
		t.Fatal("Filesystem = nil, want non-nil (requested)")
	}
	if len(resolved.Filesystem.AllowedPaths) != 0 {
		t.Errorf("AllowedPaths = %v, want empty (no auth)", resolved.Filesystem.AllowedPaths)
	}
}

// --- ValidateAuthorization tests ---

func TestValidateAuthorization_Warnings(t *testing.T) {
	// Auth references HTTP, filesystem, secrets — but plugin doesn't request any
	caps := PluginCapabilities{KV: true}
	auth := &PluginAuthorization{
		HTTP:       &HTTPAuth{AllowedHosts: []string{"*"}},
		Filesystem: &FSAuth{AllowedPaths: map[string]string{".": "/"}},
		Secrets:    &SecretsAuth{Allowed: []string{"SECRET_KEY"}},
	}

	warnings := ValidateAuthorization("test_plugin", caps, auth)

	if len(warnings) != 3 {
		t.Fatalf("warnings len = %d, want 3: %v", len(warnings), warnings)
	}
}

func TestValidateAuthorization_NoWarnings(t *testing.T) {
	caps := PluginCapabilities{
		HTTP:       true,
		Filesystem: &FSCapabilityIntent{ReadOnly: false},
		Secrets:    []string{"API_KEY"},
	}
	auth := &PluginAuthorization{
		HTTP:       &HTTPAuth{AllowedHosts: []string{"*"}},
		Filesystem: &FSAuth{AllowedPaths: map[string]string{".": "/"}},
		Secrets:    &SecretsAuth{Allowed: []string{"API_KEY"}},
	}

	warnings := ValidateAuthorization("test_plugin", caps, auth)

	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want empty", warnings)
	}
}

func TestValidateAuthorization_NilAuth(t *testing.T) {
	caps := PluginCapabilities{HTTP: true}
	warnings := ValidateAuthorization("test_plugin", caps, nil)

	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want empty for nil auth", warnings)
	}
}

// --- AuthFromConfig tests ---

func TestAuthFromConfig(t *testing.T) {
	cfg := &config.PluginAuthorizationConfig{
		HTTP: &config.HTTPAuthConfig{AllowedHosts: []string{"example.com"}},
		Filesystem: &config.FSAuthConfig{
			AllowedPaths: map[string]string{".": "/app"},
			ReadOnly:     true,
		},
		Secrets:   &config.SecretsAuthConfig{Allowed: []string{"MY_SECRET"}},
		Deny:      []string{"exec"},
		Resources: &config.ResourceLimitsConfig{MemoryMaxPages: 32, Timeout: 10000},
	}

	auth := AuthFromConfig(cfg)

	if auth == nil {
		t.Fatal("AuthFromConfig returned nil")
	}
	if auth.HTTP == nil || len(auth.HTTP.AllowedHosts) != 1 || auth.HTTP.AllowedHosts[0] != "example.com" {
		t.Errorf("HTTP.AllowedHosts = %v, want [example.com]", auth.HTTP)
	}
	if auth.Filesystem == nil || auth.Filesystem.AllowedPaths["."] != "/app" || !auth.Filesystem.ReadOnly {
		t.Errorf("Filesystem = %+v, want AllowedPaths{.:/app}, ReadOnly=true", auth.Filesystem)
	}
	if auth.Secrets == nil || len(auth.Secrets.Allowed) != 1 || auth.Secrets.Allowed[0] != "MY_SECRET" {
		t.Errorf("Secrets = %+v, want [MY_SECRET]", auth.Secrets)
	}
	if len(auth.Deny) != 1 || auth.Deny[0] != "exec" {
		t.Errorf("Deny = %v, want [exec]", auth.Deny)
	}
	if auth.Resources == nil || auth.Resources.Memory == nil || auth.Resources.Memory.MaxPages != 32 {
		t.Errorf("Resources.Memory.MaxPages = %v, want 32", auth.Resources)
	}
	if auth.Resources.Timeout != 10000 {
		t.Errorf("Resources.Timeout = %d, want 10000", auth.Resources.Timeout)
	}
}

func TestAuthFromConfig_Nil(t *testing.T) {
	auth := AuthFromConfig(nil)
	if auth != nil {
		t.Errorf("AuthFromConfig(nil) = %v, want nil", auth)
	}
}

// --- UnmarshalJSON tests for PluginCapabilities ---

func TestPluginCapabilities_UnmarshalJSON_BoolFilesystem(t *testing.T) {
	data := []byte(`{"http": true, "filesystem": true, "kv": true}`)
	var caps PluginCapabilities
	if err := json.Unmarshal(data, &caps); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !caps.HTTP {
		t.Error("HTTP = false, want true")
	}
	if !caps.KV {
		t.Error("KV = false, want true")
	}
	if caps.Filesystem == nil {
		t.Fatal("Filesystem = nil, want non-nil")
	}
	if caps.Filesystem.ReadOnly {
		t.Error("Filesystem.ReadOnly = true, want false (bool shorthand)")
	}
}

func TestPluginCapabilities_UnmarshalJSON_ObjectFilesystem(t *testing.T) {
	data := []byte(`{"filesystem": {"read_only": true}}`)
	var caps PluginCapabilities
	if err := json.Unmarshal(data, &caps); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if caps.Filesystem == nil {
		t.Fatal("Filesystem = nil, want non-nil")
	}
	if !caps.Filesystem.ReadOnly {
		t.Error("Filesystem.ReadOnly = false, want true")
	}
}

func TestPluginCapabilities_UnmarshalJSON_FalseFilesystem(t *testing.T) {
	data := []byte(`{"filesystem": false}`)
	var caps PluginCapabilities
	if err := json.Unmarshal(data, &caps); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if caps.Filesystem != nil {
		t.Errorf("Filesystem = %+v, want nil (false means not needed)", caps.Filesystem)
	}
}

func TestPluginCapabilities_UnmarshalJSON_Empty(t *testing.T) {
	data := []byte(`{}`)
	var caps PluginCapabilities
	if err := json.Unmarshal(data, &caps); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if caps.HTTP || caps.KV || caps.Log || caps.Exec || caps.Elevated {
		t.Error("expected all bool fields to be false for empty object")
	}
	if caps.Filesystem != nil {
		t.Error("Filesystem should be nil for empty object")
	}
}
