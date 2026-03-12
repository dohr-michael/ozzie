package ws

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/infra/secrets"
)

// DiscoverLocalToken reads and decrypts the local token from $OZZIE_PATH/.local_token.
// Returns empty string if not found or decryption fails (no error — caller decides policy).
func DiscoverLocalToken() string {
	tokenPath := filepath.Join(config.OzziePath(), ".local_token")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return ""
	}
	blob := strings.TrimSpace(string(data))
	if !secrets.IsEncrypted(blob) {
		return ""
	}

	identity, err := secrets.LoadIdentity(secrets.CurrentKeyPath())
	if err != nil {
		return ""
	}

	token, err := secrets.Decrypt(blob, identity)
	if err != nil {
		return ""
	}
	return token
}
