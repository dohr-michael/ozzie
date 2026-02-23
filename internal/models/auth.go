package models

import (
	"fmt"
	"os"
	"strings"

	"github.com/dohr-michael/ozzie/internal/config"
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
func ResolveAuth(cfg config.ProviderConfig) (ResolvedAuth, error) {
	resolve := func(token string) string {
		trimmed := strings.TrimSpace(token)
		if trimmed == "" {
			return ""
		}
		if strings.HasPrefix(trimmed, "${") && strings.HasSuffix(trimmed, "}") {
			return os.Getenv(trimmed[2 : len(trimmed)-1])
		}
		return trimmed
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
		if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
			return ResolvedAuth{Kind: AuthAPIKey, Value: key}, nil
		}
		return ResolvedAuth{}, fmt.Errorf("ANTHROPIC_API_KEY not set")
	case "openai":
		if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			return ResolvedAuth{Kind: AuthAPIKey, Value: key}, nil
		}
		return ResolvedAuth{}, fmt.Errorf("OPENAI_API_KEY not set")
	case "mistral":
		if key := os.Getenv("MISTRAL_API_KEY"); key != "" {
			return ResolvedAuth{Kind: AuthAPIKey, Value: key}, nil
		}
		return ResolvedAuth{}, fmt.Errorf("MISTRAL_API_KEY not set")
	default:
		return ResolvedAuth{}, fmt.Errorf("unknown driver %q: cannot resolve auth", cfg.Driver)
	}
}
