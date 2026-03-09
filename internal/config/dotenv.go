package config

import (
	"bufio"
	"os"
	"strings"
)

// parseDotenvFile reads a .env file and returns key-value pairs.
// Missing file returns nil map and no error.
func parseDotenvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = unquote(value)
		vars[key] = value
	}
	return vars, scanner.Err()
}

// LoadDotenv reads a .env file and sets environment variables that are not already defined.
// Missing file is silently ignored. Existing env vars are never overridden.
func LoadDotenv(path string) error {
	vars, err := parseDotenvFile(path)
	if err != nil {
		return err
	}
	for key, value := range vars {
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
	return nil
}

// ReloadDotenv reads a .env file and sets all environment variables unconditionally,
// overriding any existing values. Used for hot-reload after secret injection.
func ReloadDotenv(path string) error {
	vars, err := parseDotenvFile(path)
	if err != nil {
		return err
	}
	for key, value := range vars {
		os.Setenv(key, value)
	}
	return nil
}

// unquote strips matching surrounding quotes (single or double).
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
