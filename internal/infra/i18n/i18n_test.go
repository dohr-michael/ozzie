package i18n

import (
	"os"
	"testing"
)

func init() {
	// Register test translations.
	Register("en", map[string]string{
		"test.hello":   "Hello",
		"test.goodbye": "Goodbye",
		"test.only_en": "English only",
	})
	Register("fr", map[string]string{
		"test.hello":   "Bonjour",
		"test.goodbye": "Au revoir",
		"test.only_fr": "Français seulement",
	})
}

func TestT(t *testing.T) {
	Lang = "en"
	if got := T("test.hello"); got != "Hello" {
		t.Errorf("T(test.hello) = %q, want Hello", got)
	}
}

func TestTFrench(t *testing.T) {
	Lang = "fr"
	defer func() { Lang = "en" }()

	if got := T("test.hello"); got != "Bonjour" {
		t.Errorf("T(test.hello) = %q, want Bonjour", got)
	}
	if got := T("test.goodbye"); got != "Au revoir" {
		t.Errorf("T(test.goodbye) = %q, want Au revoir", got)
	}
}

func TestTFallbackToEN(t *testing.T) {
	Lang = "fr"
	defer func() { Lang = "en" }()

	// Key only in EN → should fall back.
	if got := T("test.only_en"); got != "English only" {
		t.Errorf("T(test.only_en) = %q, want 'English only' (EN fallback)", got)
	}
}

func TestTFallbackToKey(t *testing.T) {
	Lang = "en"
	// Key not registered at all → returns the key itself.
	if got := T("nonexistent.key"); got != "nonexistent.key" {
		t.Errorf("T(nonexistent.key) = %q, want 'nonexistent.key'", got)
	}
}

func TestRegisterMerge(t *testing.T) {
	Register("de", map[string]string{"test.a": "A"})
	Register("de", map[string]string{"test.b": "B"})

	Lang = "de"
	defer func() { Lang = "en" }()

	if got := T("test.a"); got != "A" {
		t.Errorf("T(test.a) = %q, want A", got)
	}
	if got := T("test.b"); got != "B" {
		t.Errorf("T(test.b) = %q, want B", got)
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		env  string
		want string
	}{
		{"fr_FR.UTF-8", "fr"},
		{"en_US.UTF-8", "en"},
		{"C", "en"},
		{"POSIX", "en"},
		{"", "en"},
	}

	for _, tt := range tests {
		os.Setenv("LANG", tt.env)
		os.Unsetenv("LC_ALL")
		got := Detect()
		if got != tt.want {
			t.Errorf("Detect() with LANG=%q = %q, want %q", tt.env, got, tt.want)
		}
	}

	// LC_ALL takes priority.
	os.Setenv("LANG", "en_US.UTF-8")
	os.Setenv("LC_ALL", "fr_FR.UTF-8")
	if got := Detect(); got != "fr" {
		t.Errorf("Detect() with LC_ALL=fr = %q, want fr", got)
	}

	// Cleanup.
	os.Unsetenv("LANG")
	os.Unsetenv("LC_ALL")
}

func TestSupportedLocales(t *testing.T) {
	locales := SupportedLocales()
	if len(locales) < 2 {
		t.Errorf("SupportedLocales() = %v, want at least en and fr", locales)
	}
	hasEN, hasFR := false, false
	for _, l := range locales {
		if l == "en" {
			hasEN = true
		}
		if l == "fr" {
			hasFR = true
		}
	}
	if !hasEN || !hasFR {
		t.Errorf("SupportedLocales() = %v, missing en or fr", locales)
	}
}

func TestKeys(t *testing.T) {
	keys := Keys("en")
	if len(keys) == 0 {
		t.Error("Keys(en) returned empty")
	}
	// Should be sorted.
	for i := 1; i < len(keys); i++ {
		if keys[i] < keys[i-1] {
			t.Errorf("Keys not sorted: %q before %q", keys[i-1], keys[i])
			break
		}
	}
}

func TestExtractLang(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"fr_FR.UTF-8", "fr"},
		{"en_US.UTF-8", "en"},
		{"de_DE", "de"},
		{"ja", "ja"},
		{"C", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := extractLang(tt.input); got != tt.want {
			t.Errorf("extractLang(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
