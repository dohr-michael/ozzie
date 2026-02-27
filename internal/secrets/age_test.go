package secrets

import (
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

func TestGenerateIdentity_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".age-key")

	if err := GenerateIdentity(path); err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestGenerateIdentity_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".age-key")

	if err := GenerateIdentity(path); err != nil {
		t.Fatalf("first call: %v", err)
	}
	data1, _ := os.ReadFile(path)

	if err := GenerateIdentity(path); err != nil {
		t.Fatalf("second call: %v", err)
	}
	data2, _ := os.ReadFile(path)

	if string(data1) != string(data2) {
		t.Error("idempotency broken: file changed on second call")
	}
}

func TestLoadIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".age-key")

	if err := GenerateIdentity(path); err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}

	id, err := LoadIdentity(path)
	if err != nil {
		t.Fatalf("LoadIdentity: %v", err)
	}
	if id == nil {
		t.Fatal("identity is nil")
	}
	if id.Recipient() == nil {
		t.Fatal("recipient is nil")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	plaintext := "sk-ant-api03-secret-token-abc123"
	encrypted, err := Encrypt(plaintext, identity.Recipient())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if !IsEncrypted(encrypted) {
		t.Errorf("IsEncrypted(%q) = false, want true", encrypted)
	}
	if encrypted == plaintext {
		t.Error("encrypted text should differ from plaintext")
	}

	decrypted, err := Decrypt(encrypted, identity)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecrypt_EmptyString(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	encrypted, err := Encrypt("", identity.Recipient())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	decrypted, err := Decrypt(encrypted, identity)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if decrypted != "" {
		t.Errorf("decrypted = %q, want empty", decrypted)
	}
}

func TestIsEncrypted(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"ENC[age:abc123]", true},
		{"ENC[age:]", true},
		{"plaintext", false},
		{"ENC[age:abc123", false},
		{"age:abc123]", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsEncrypted(tt.input); got != tt.want {
			t.Errorf("IsEncrypted(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDecrypt_RejectsPlaintext(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	_, err = Decrypt("not-encrypted", identity)
	if err == nil {
		t.Error("expected error for non-encrypted input")
	}
}
