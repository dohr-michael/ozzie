package commands

import (
	"fmt"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/secrets"
)

// loadConfigWithKeyRing loads the config with optional age-key decryption.
// Returns the config, the keyring (may be nil), and any error.
func loadConfigWithKeyRing(configPath string) (*config.Config, *secrets.KeyRing, error) {
	kr, _ := secrets.NewKeyRing()
	var opts []config.LoadOption
	if kr != nil {
		opts = append(opts, config.WithDecrypt(kr.DecryptValue))
	}
	cfg, err := config.Load(configPath, opts...)
	if err != nil {
		return nil, kr, fmt.Errorf("load config: %w", err)
	}
	return cfg, kr, nil
}
