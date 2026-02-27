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

// quoteValue wraps the value in double quotes if it contains spaces, quotes, or special chars.
func quoteValue(v string) string {
	if strings.ContainsAny(v, " \t\"'\\#$") {
		escaped := strings.ReplaceAll(v, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return v
}
