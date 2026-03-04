package secrets

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"filippo.io/age"

	"github.com/dohr-michael/ozzie/internal/config"
)

const encPrefix = "ENC[age:"
const encSuffix = "]"

// KeyPath returns the legacy age key file path: $OZZIE_PATH/.age-key.
func KeyPath() string {
	return filepath.Join(config.OzziePath(), ".age-key")
}

// AgeDirPath returns the age key directory: $OZZIE_PATH/.age/.
func AgeDirPath() string {
	return filepath.Join(config.OzziePath(), ".age")
}

// CurrentKeyPath returns the active key file: $OZZIE_PATH/.age/current.key.
func CurrentKeyPath() string {
	return filepath.Join(AgeDirPath(), "current.key")
}

// KeyRing holds the current and historical age identities for decryption.
type KeyRing struct {
	Current *age.X25519Identity
	Old     []*age.X25519Identity
}

// NewKeyRing loads all keys from the .age/ directory.
// Returns nil (not error) if the directory does not exist.
func NewKeyRing() (*KeyRing, error) {
	ageDir := AgeDirPath()
	if _, err := os.Stat(ageDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(ageDir)
	if err != nil {
		return nil, fmt.Errorf("read age dir: %w", err)
	}

	kr := &KeyRing{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".key") {
			continue
		}
		path := filepath.Join(ageDir, e.Name())
		id, err := LoadIdentity(path)
		if err != nil {
			continue // skip unreadable keys
		}
		if e.Name() == "current.key" {
			kr.Current = id
		} else {
			kr.Old = append(kr.Old, id)
		}
	}
	if kr.Current == nil {
		return nil, fmt.Errorf("no current.key found in %s", ageDir)
	}
	return kr, nil
}

// CurrentRecipient returns the public key of the current identity.
func (kr *KeyRing) CurrentRecipient() *age.X25519Recipient {
	if kr == nil || kr.Current == nil {
		return nil
	}
	return kr.Current.Recipient()
}

// DecryptValue decrypts an ENC[age:...] value, trying current key first then old keys.
// If the value is not encrypted, it is returned as-is (backward compat).
func (kr *KeyRing) DecryptValue(value string) (string, error) {
	if !IsEncrypted(value) {
		return value, nil
	}
	if kr == nil || kr.Current == nil {
		return value, fmt.Errorf("no keyring available")
	}

	// Try current key first
	if dec, err := Decrypt(value, kr.Current); err == nil {
		return dec, nil
	}

	// Fallback to old keys
	for _, old := range kr.Old {
		if dec, err := Decrypt(value, old); err == nil {
			return dec, nil
		}
	}
	return "", fmt.Errorf("decrypt: no key could decrypt the value")
}

// RotateKey generates a new current key and renames the old one with a date stamp.
func RotateKey(ageDir string) (*KeyRing, error) {
	currentPath := filepath.Join(ageDir, "current.key")

	// Rename old current.key if it exists
	if _, err := os.Stat(currentPath); err == nil {
		stamp := time.Now().Format("2006-01-02")
		oldPath := filepath.Join(ageDir, stamp+".key")
		// Handle multiple rotations on same day
		for i := 1; ; i++ {
			if _, err := os.Stat(oldPath); os.IsNotExist(err) {
				break
			}
			oldPath = filepath.Join(ageDir, fmt.Sprintf("%s-%d.key", stamp, i))
		}
		if err := os.Rename(currentPath, oldPath); err != nil {
			return nil, fmt.Errorf("rename old key: %w", err)
		}
	}

	// Generate new key
	if err := GenerateIdentity(currentPath); err != nil {
		return nil, fmt.Errorf("generate new key: %w", err)
	}

	return NewKeyRing()
}

// ReEncryptAll re-encrypts all ENC[age:...] values in a .env file with the current key.
// Returns the number of re-encrypted entries.
func ReEncryptAll(dotenvPath string, kr *KeyRing) (int, error) {
	if kr == nil || kr.Current == nil {
		return 0, fmt.Errorf("no keyring available")
	}

	lines, err := readLines(dotenvPath)
	if err != nil {
		return 0, fmt.Errorf("read dotenv: %w", err)
	}

	count := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if !IsEncrypted(value) {
			continue
		}

		// Decrypt with any key
		plaintext, err := kr.DecryptValue(value)
		if err != nil {
			return count, fmt.Errorf("decrypt %s: %w", strings.TrimSpace(key), err)
		}

		// Re-encrypt with current key
		encrypted, err := Encrypt(plaintext, kr.Current.Recipient())
		if err != nil {
			return count, fmt.Errorf("re-encrypt %s: %w", strings.TrimSpace(key), err)
		}

		lines[i] = strings.TrimSpace(key) + "=" + encrypted
		count++
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(dotenvPath, []byte(content), 0o600); err != nil {
		return count, fmt.Errorf("write dotenv: %w", err)
	}
	return count, nil
}

// MigrateToAgeDir migrates from legacy .age-key to .age/current.key.
// Idempotent: does nothing if already migrated or no legacy key exists.
func MigrateToAgeDir(ozziePath string) error {
	legacyPath := filepath.Join(ozziePath, ".age-key")
	ageDir := filepath.Join(ozziePath, ".age")
	currentPath := filepath.Join(ageDir, "current.key")

	// Already migrated or no legacy key
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		return nil
	}
	if _, err := os.Stat(currentPath); err == nil {
		return nil // already migrated
	}

	// Create .age/ directory
	if err := os.MkdirAll(ageDir, 0o700); err != nil {
		return fmt.Errorf("create age dir: %w", err)
	}

	// Copy legacy key to current.key
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return fmt.Errorf("read legacy key: %w", err)
	}
	if err := os.WriteFile(currentPath, data, 0o600); err != nil {
		return fmt.Errorf("write current key: %w", err)
	}

	// Remove legacy key
	if err := os.Remove(legacyPath); err != nil {
		return fmt.Errorf("remove legacy key: %w", err)
	}
	return nil
}

// AllIdentities returns all identities (current + old) for decryption, sorted current-first.
func (kr *KeyRing) AllIdentities() []*age.X25519Identity {
	if kr == nil {
		return nil
	}
	ids := make([]*age.X25519Identity, 0, 1+len(kr.Old))
	if kr.Current != nil {
		ids = append(ids, kr.Current)
	}
	ids = append(ids, kr.Old...)
	return ids
}

// SortedKeyFiles returns key filenames sorted (for predictable listing).
func SortedKeyFiles(ageDir string) ([]string, error) {
	entries, err := os.ReadDir(ageDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".key") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
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
