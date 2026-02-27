package secrets

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"

	"github.com/dohr-michael/ozzie/internal/config"
)

const encPrefix = "ENC[age:"
const encSuffix = "]"

// KeyPath returns the default age key file path: $OZZIE_PATH/.age-key.
func KeyPath() string {
	return filepath.Join(config.OzziePath(), ".age-key")
}

// GenerateIdentity creates an X25519 key pair and writes it to path with 0o600.
// It is idempotent: if the file already exists, it does nothing.
func GenerateIdentity(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return fmt.Errorf("generate age identity: %w", err)
	}

	content := fmt.Sprintf("# created by ozzie\n# public key: %s\n%s\n",
		identity.Recipient().String(), identity.String())

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create key directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write age key: %w", err)
	}
	return nil
}

// LoadIdentity reads an age private key from the given file.
func LoadIdentity(path string) (*age.X25519Identity, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open age key: %w", err)
	}
	defer f.Close()

	identities, err := age.ParseIdentities(f)
	if err != nil {
		return nil, fmt.Errorf("parse age identities: %w", err)
	}
	if len(identities) == 0 {
		return nil, fmt.Errorf("no identities found in %s", path)
	}

	id, ok := identities[0].(*age.X25519Identity)
	if !ok {
		return nil, fmt.Errorf("unexpected identity type in %s", path)
	}
	return id, nil
}

// Encrypt encrypts plaintext with the given recipient and returns an ENC[age:...] blob.
func Encrypt(plaintext string, recipient *age.X25519Recipient) (string, error) {
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return "", fmt.Errorf("age encrypt init: %w", err)
	}
	if _, err := io.WriteString(w, plaintext); err != nil {
		return "", fmt.Errorf("age encrypt write: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("age encrypt close: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return encPrefix + encoded + encSuffix, nil
}

// Decrypt decrypts an ENC[age:...] blob back to plaintext.
func Decrypt(blob string, identity *age.X25519Identity) (string, error) {
	if !IsEncrypted(blob) {
		return "", fmt.Errorf("not an encrypted blob")
	}

	encoded := blob[len(encPrefix) : len(blob)-len(encSuffix)]
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	r, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return "", fmt.Errorf("age decrypt: %w", err)
	}

	plainBytes, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read decrypted: %w", err)
	}
	return string(plainBytes), nil
}

// IsEncrypted returns true if the string is an ENC[age:...] blob.
func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, encPrefix) && strings.HasSuffix(s, encSuffix)
}
