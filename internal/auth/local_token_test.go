package auth

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"

	"github.com/dohr-michael/ozzie/internal/secrets"
)

func TestNewLocalAuth(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	tokenPath := filepath.Join(dir, ".local_token")

	auth, err := NewLocalAuth(tokenPath, identity.Recipient())
	if err != nil {
		t.Fatal(err)
	}

	// Token file should exist and be ENC[age:...] format.
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	blob := string(data)
	if !secrets.IsEncrypted(blob) {
		t.Fatalf("expected ENC[age:...] blob, got: %s", blob)
	}

	// Decrypt and verify it matches the in-memory token.
	decrypted, err := secrets.Decrypt(blob, identity)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != auth.token {
		t.Fatalf("decrypted token mismatch: got %q, want %q", decrypted, auth.token)
	}
}

func TestAuthenticateHTTP_Valid(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	auth, err := NewLocalAuth(filepath.Join(dir, ".local_token"), identity.Recipient())
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+auth.token)

	deviceID, err := auth.AuthenticateHTTP(req)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if deviceID != DeviceLocal {
		t.Fatalf("expected %q, got %q", DeviceLocal, deviceID)
	}
}

func TestAuthenticateHTTP_Invalid(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	auth, err := NewLocalAuth(filepath.Join(dir, ".local_token"), identity.Recipient())
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")

	_, err = auth.AuthenticateHTTP(req)
	if err != ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestAuthenticateHTTP_Missing(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	auth, err := NewLocalAuth(filepath.Join(dir, ".local_token"), identity.Recipient())
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", "/", nil)

	_, err = auth.AuthenticateHTTP(req)
	if err != ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
}
