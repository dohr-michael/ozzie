package agent

// ModelTier classifies LLM capabilities for prompt adaptation.
// Only TierSmall triggers compact prompt variants; TierMedium
// and TierLarge share the full prompts.
type ModelTier string

const (
	TierSmall  ModelTier = "small"  // < 16K context
	TierMedium ModelTier = "medium" // 16K-64K context
	TierLarge  ModelTier = "large"  // >= 64K context
)

// ResolveTier returns the model tier. An explicit tier string (from config)
// takes precedence; otherwise the context window size is used.
func ResolveTier(explicitTier string, contextWindow int) ModelTier {
	switch ModelTier(explicitTier) {
	case TierSmall, TierMedium, TierLarge:
		return ModelTier(explicitTier)
	}

	switch {
	case contextWindow > 0 && contextWindow < 16_000:
		return TierSmall
	case contextWindow >= 16_000 && contextWindow < 64_000:
		return TierMedium
	default:
		return TierLarge
	}
}
