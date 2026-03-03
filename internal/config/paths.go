package config

import (
	"os"
	"path/filepath"
)

// OzziePath returns the root directory for Ozzie data.
// It checks $OZZIE_PATH first, then $OZZIE_HOME as alias, otherwise defaults to ~/.ozzie.
func OzziePath() string {
	if v := os.Getenv("OZZIE_PATH"); v != "" {
		return v
	}
	if v := os.Getenv("OZZIE_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".ozzie")
	}
	return filepath.Join(home, ".ozzie")
}

// ConfigPath returns the path to the Ozzie config file.
func ConfigPath() string {
	return filepath.Join(OzziePath(), "config.jsonc")
}

// DotenvPath returns the path to the Ozzie .env file.
func DotenvPath() string {
	return filepath.Join(OzziePath(), ".env")
}
