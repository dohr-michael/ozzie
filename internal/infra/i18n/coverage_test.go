package i18n_test

import (
	"strings"
	"testing"

	"github.com/dohr-michael/ozzie/internal/infra/i18n"
	// Import component and wizard packages to trigger their init() registrations.
	_ "github.com/dohr-michael/ozzie/internal/infra/ui/components"
	_ "github.com/dohr-michael/ozzie/internal/infra/ui/setup_wizard"
)

// TestFRCoverage verifies that every EN key has a corresponding FR translation.
func TestFRCoverage(t *testing.T) {
	enKeys := i18n.Keys("en")
	if len(enKeys) == 0 {
		t.Fatal("EN catalog is empty — component/wizard init() not triggered?")
	}

	frKeys := i18n.Keys("fr")
	frSet := make(map[string]struct{}, len(frKeys))
	for _, k := range frKeys {
		frSet[k] = struct{}{}
	}

	var missing []string
	for _, k := range enKeys {
		if strings.HasPrefix(k, "test.") {
			continue
		}
		if _, ok := frSet[k]; !ok {
			missing = append(missing, k)
		}
	}

	if len(missing) > 0 {
		t.Errorf("FR catalog is missing %d keys present in EN:", len(missing))
		for _, k := range missing {
			t.Errorf("  - %s", k)
		}
	}
}

// TestENCoverage verifies that every FR key has a corresponding EN translation.
func TestENCoverage(t *testing.T) {
	frKeys := i18n.Keys("fr")
	if len(frKeys) == 0 {
		t.Fatal("FR catalog is empty — component/wizard init() not triggered?")
	}

	enKeys := i18n.Keys("en")
	enSet := make(map[string]struct{}, len(enKeys))
	for _, k := range enKeys {
		enSet[k] = struct{}{}
	}

	var extra []string
	for _, k := range frKeys {
		if strings.HasPrefix(k, "test.") {
			continue
		}
		if _, ok := enSet[k]; !ok {
			extra = append(extra, k)
		}
	}

	if len(extra) > 0 {
		t.Errorf("FR catalog has %d keys missing from EN:", len(extra))
		for _, k := range extra {
			t.Errorf("  - %s", k)
		}
	}
}
