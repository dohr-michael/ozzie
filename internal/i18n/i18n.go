// Package i18n provides a lightweight translation system.
// Each context (components, wizard) registers its own translations via Register().
package i18n

import (
	"os"
	"sort"
	"strings"
	"sync"
)

// Lang is the active language code (e.g. "en", "fr").
var Lang = "en"

var (
	mu       sync.RWMutex
	catalogs = map[string]map[string]string{}
)

// Register merges entries into the catalog for the given lang.
// Called by each context (components, wizard) at init time.
func Register(lang string, entries map[string]string) {
	mu.Lock()
	defer mu.Unlock()
	if catalogs[lang] == nil {
		catalogs[lang] = make(map[string]string)
	}
	for k, v := range entries {
		catalogs[lang][k] = v
	}
}

// T returns the translation for key in the current Lang.
// Fallback: EN catalog → key string itself.
func T(key string) string {
	mu.RLock()
	defer mu.RUnlock()

	if c := catalogs[Lang]; c != nil {
		if v, ok := c[key]; ok {
			return v
		}
	}
	if Lang != "en" {
		if c := catalogs["en"]; c != nil {
			if v, ok := c[key]; ok {
				return v
			}
		}
	}
	return key
}

// Keys returns all registered keys for the given lang, sorted.
func Keys(lang string) []string {
	mu.RLock()
	defer mu.RUnlock()

	c := catalogs[lang]
	if c == nil {
		return nil
	}
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Detect resolves locale from LANG/LC_ALL env vars.
// Returns the two-letter language code (e.g. "fr", "en").
func Detect() string {
	for _, env := range []string{"LC_ALL", "LANG"} {
		val := os.Getenv(env)
		if val == "" || val == "C" || val == "POSIX" {
			continue
		}
		lang := extractLang(val)
		if lang != "" && isSupported(lang) {
			return lang
		}
	}
	return "en"
}

// SupportedLocales returns available locale codes.
func SupportedLocales() []string {
	mu.RLock()
	defer mu.RUnlock()

	locales := make([]string, 0, len(catalogs))
	for lang := range catalogs {
		locales = append(locales, lang)
	}
	sort.Strings(locales)
	return locales
}

// extractLang extracts the two-letter language code from a locale string
// like "fr_FR.UTF-8" → "fr".
func extractLang(locale string) string {
	// Strip encoding (e.g. ".UTF-8")
	if i := strings.IndexByte(locale, '.'); i > 0 {
		locale = locale[:i]
	}
	// Strip country (e.g. "_FR")
	if i := strings.IndexByte(locale, '_'); i > 0 {
		locale = locale[:i]
	}
	if len(locale) >= 2 {
		return strings.ToLower(locale[:2])
	}
	return ""
}

// isSupported checks if a language has any registered entries.
func isSupported(lang string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return len(catalogs[lang]) > 0
}
