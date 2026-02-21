package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotenv(t *testing.T) {
	content := `# Database config
DB_HOST=localhost
DB_PORT=5432

# Quoted values
SECRET="my-secret-value"
SINGLE='single-quoted'

# Spaces around =
SPACED_KEY = spaced_value
`

	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Clear any existing values.
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("SECRET")
	os.Unsetenv("SINGLE")
	os.Unsetenv("SPACED_KEY")

	if err := LoadDotenv(path); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		key, want string
	}{
		{"DB_HOST", "localhost"},
		{"DB_PORT", "5432"},
		{"SECRET", "my-secret-value"},
		{"SINGLE", "single-quoted"},
		{"SPACED_KEY", "spaced_value"},
	}

	for _, tt := range tests {
		got := os.Getenv(tt.key)
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestLoadDotenvNoOverride(t *testing.T) {
	content := `EXISTING_VAR=new-value`
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EXISTING_VAR", "original")

	if err := LoadDotenv(path); err != nil {
		t.Fatal(err)
	}

	if got := os.Getenv("EXISTING_VAR"); got != "original" {
		t.Errorf("expected existing var to be preserved, got %q", got)
	}
}

func TestLoadDotenvMissingFile(t *testing.T) {
	err := LoadDotenv("/nonexistent/.env")
	if err != nil {
		t.Errorf("missing file should be silently ignored, got: %v", err)
	}
}
