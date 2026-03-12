package secrets

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// SetEntry writes or updates a KEY=VALUE line in a .env file.
// It preserves comments, ordering, and blank lines.
// If the key already exists, its value is replaced in-place.
// If the key is new, it is appended at the end.
func SetEntry(path, key, value string) error {
	lines, err := readLines(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read dotenv: %w", err)
	}

	// Escape value: wrap in double quotes if it contains special chars.
	quotedValue := quoteValue(value)
	newLine := key + "=" + quotedValue

	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		k, _, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == key {
			lines[i] = newLine
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, newLine)
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o600)
}

// readLines reads all lines from a file. Returns empty slice if file doesn't exist.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// ListAllKeys returns all key names from a .env file (skips comments and blank lines).
func ListAllKeys(path string) ([]string, error) {
	lines, err := readLines(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read dotenv: %w", err)
	}

	var keys []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		k, _, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		keys = append(keys, strings.TrimSpace(k))
	}
	return keys, nil
}

// DeleteEntry removes a key from the .env file. Returns nil if key not found.
func DeleteEntry(path, key string) error {
	lines, err := readLines(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read dotenv: %w", err)
	}

	var newLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			newLines = append(newLines, line)
			continue
		}
		k, _, ok := strings.Cut(trimmed, "=")
		if ok && strings.TrimSpace(k) == key {
			continue // skip this line
		}
		newLines = append(newLines, line)
	}

	content := strings.Join(newLines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o600)
}

// GetValue returns the raw value for a key from a .env file.
// Returns empty string and false if not found.
func GetValue(path, key string) (string, bool) {
	lines, err := readLines(path)
	if err != nil {
		return "", false
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		k, v, ok := strings.Cut(trimmed, "=")
		if ok && strings.TrimSpace(k) == key {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

// quoteValue wraps the value in double quotes if it contains spaces, quotes, or special chars.
func quoteValue(v string) string {
	if strings.ContainsAny(v, " \t\"'\\#$") {
		escaped := strings.ReplaceAll(v, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return v
}
