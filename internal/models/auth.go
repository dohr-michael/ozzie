package models

import (
	"fmt"
	"os"
	"strings"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/secrets"
)

// AuthKind distinguishes between API key and Bearer token auth.
type AuthKind int

const (
	AuthAPIKey AuthKind = iota
	AuthBearerToken
)

// ResolvedAuth holds the resolved credentials and their kind.
type ResolvedAuth struct {
	Kind  AuthKind
	Value string
}

// ResolveAuth resolves the credentials for a provider.
// Resolution order: direct token → direct api_key → env_var → driver default env.
// If kr is non-nil, ENC[age:...] values are decrypted transparently.
func ResolveAuth(cfg config.ProviderConfig, kr *secrets.KeyRing) (ResolvedAuth, error) {
	resolve := func(token string) string {
		trimmed := strings.TrimSpace(token)
		if trimmed == "" {
			return ""
		}
		if strings.HasPrefix(trimmed, "${") && strings.HasSuffix(trimmed, "}") {
			return maybeDecrypt(os.Getenv(trimmed[2:len(trimmed)-1]), kr)
		}
		return maybeDecrypt(trimmed, kr)
	}
	// Direct Bearer token (Claude Code / OAuth)
	token := resolve(cfg.Auth.Token)
	if token != "" {
		return ResolvedAuth{Kind: AuthBearerToken, Value: token}, nil
	}

	// Direct API key from config
	apiKey := resolve(cfg.Auth.APIKey)
	if apiKey != "" {
		return ResolvedAuth{Kind: AuthAPIKey, Value: apiKey}, nil
	}

	// Default env vars per driver
	switch strings.ToLower(cfg.Driver) {
	case "anthropic":
		if key := maybeDecrypt(os.Getenv("ANTHROPIC_API_KEY"), kr); key != "" {
			return ResolvedAuth{Kind: AuthAPIKey, Value: key}, nil
		}
		return ResolvedAuth{}, fmt.Errorf("ANTHROPIC_API_KEY not set")
	case "openai":
		if key := maybeDecrypt(os.Getenv("OPENAI_API_KEY"), kr); key != "" {
			return ResolvedAuth{Kind: AuthAPIKey, Value: key}, nil
		}
		return ResolvedAuth{}, fmt.Errorf("OPENAI_API_KEY not set")
	case "openai-like":
		return ResolvedAuth{}, nil
	case "mistral":
		if key := maybeDecrypt(os.Getenv("MISTRAL_API_KEY"), kr); key != "" {
			return ResolvedAuth{Kind: AuthAPIKey, Value: key}, nil
		}
		return ResolvedAuth{}, fmt.Errorf("MISTRAL_API_KEY not set")
	case "gemini":
		if key := maybeDecrypt(os.Getenv("GOOGLE_API_KEY"), kr); key != "" {
			return ResolvedAuth{Kind: AuthAPIKey, Value: key}, nil
		}
		return ResolvedAuth{}, fmt.Errorf("GOOGLE_API_KEY not set")
	default:
		return ResolvedAuth{}, fmt.Errorf("unknown driver %q: cannot resolve auth", cfg.Driver)
	}
}

// maybeDecrypt transparently decrypts ENC[age:...] values if a keyring is available.
func maybeDecrypt(value string, kr *secrets.KeyRing) string {
	if kr == nil || !secrets.IsEncrypted(value) {
		return value
	}
	if dec, err := kr.DecryptValue(value); err == nil {
		return dec
	}
	return value
}
