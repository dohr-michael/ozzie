package agent

import "testing"

func TestResolveTier(t *testing.T) {
	tests := []struct {
		name          string
		explicitTier  string
		contextWindow int
		want          ModelTier
	}{
		{"explicit small", "small", 200000, TierSmall},
		{"explicit medium", "medium", 200000, TierMedium},
		{"explicit large", "large", 8192, TierLarge},
		{"explicit invalid falls back to window", "bogus", 4096, TierSmall},
		{"window < 16K", "", 8192, TierSmall},
		{"window == 15999", "", 15999, TierSmall},
		{"window == 16000", "", 16000, TierMedium},
		{"window == 63999", "", 63999, TierMedium},
		{"window == 64000", "", 64000, TierLarge},
		{"window == 200000", "", 200000, TierLarge},
		{"window == 0 (unknown) defaults large", "", 0, TierLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveTier(tt.explicitTier, tt.contextWindow)
			if got != tt.want {
				t.Errorf("ResolveTier(%q, %d) = %q, want %q", tt.explicitTier, tt.contextWindow, got, tt.want)
			}
		})
	}
}
