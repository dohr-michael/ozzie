package wizard

import "github.com/dohr-michael/ozzie/internal/i18n"

func init() {
	i18n.Register("en", map[string]string{
		// Wizard chrome
		"wizard.title":         "  Ozzie Setup",
		"wizard.applying":      "  Applying configuration...",
		"wizard.step_progress": "  Step %d/%d — %s",

		// Welcome
		"wizard.welcome.title":       "  Welcome to Ozzie!",
		"wizard.welcome.subtitle":    "  Let's set up your agent OS. Press enter to continue...",
		"wizard.welcome.existing_q":  "Existing config found. What would you like to do?",
		"wizard.welcome.load":        "Load existing & modify",
		"wizard.welcome.load.desc":   "Pre-fill from current config",
		"wizard.welcome.fresh":       "Start fresh",
		"wizard.welcome.fresh.desc":  "Overwrite with new config",
		"wizard.welcome.cancel":      "Cancel",
		"wizard.welcome.cancel.desc": "Exit without changes",
		"wizard.welcome.language":    "Language / Langue",

		// Provider
		"wizard.provider.choose":         "Choose LLM provider",
		"wizard.provider.alias":          "Provider alias (config key)",
		"wizard.provider.model":          "Choose a model",
		"wizard.provider.custom":         "Custom model",
		"wizard.provider.custom.desc":    "Enter a model ID manually",
		"wizard.provider.model_id":       "Enter model ID",
		"wizard.provider.api_key_for":    "API Key for %s",
		"wizard.provider.key_now":        "Enter API key now",
		"wizard.provider.key_now.desc":   "Will be encrypted with age",
		"wizard.provider.key_later":      "I'll set it later",
		"wizard.provider.key_later.desc": "Use: ozzie secret set %s",
		"wizard.provider.key_reuse":      "Reuse key from %s",
		"wizard.provider.key_reuse.desc": "Share %s",
		"wizard.provider.key_new":        "Enter a new API key",
		"wizard.provider.key_new.desc":   "Separate key, stored as %s",
		"wizard.provider.enter_key":      "Enter your %s:",
		"wizard.provider.caps":           "Capabilities (space=toggle, enter=confirm)",
		"wizard.provider.tags":           "Tags (optional, comma-separated — e.g. self-hosted, secured, primary)",
		"wizard.provider.prompt":         "System prompt (optional, custom instruction for this provider)",
		"wizard.provider.add_more":       "Add another LLM provider?",
		"wizard.provider.base_url":       "Base URL",
		"wizard.provider.base_url.ollama":  "Ollama base URL",
		"wizard.provider.base_url.openai":  "Base URL (optional, for OpenAI-compatible APIs)",

		// Driver descriptions
		"wizard.driver.anthropic.desc": "Claude models, best for coding",
		"wizard.driver.openai.desc":    "GPT models (+ OpenAI-compatible APIs)",
		"wizard.driver.gemini.desc":    "Gemini models",
		"wizard.driver.mistral.desc":   "Mistral models, EU-based",
		"wizard.driver.ollama.desc":    "Local models, no API key needed",

		// Model descriptions
		"wizard.model.sonnet4.desc":     "Best balance of speed and quality",
		"wizard.model.opus4.desc":       "Most capable",
		"wizard.model.haiku4.desc":      "Fast and affordable",
		"wizard.model.gpt4o.desc":       "Flagship multimodal model",
		"wizard.model.gpt4omini.desc":   "Fast and affordable",
		"wizard.model.o3.desc":          "Reasoning model",
		"wizard.model.gem25flash.desc":  "Fast and versatile",
		"wizard.model.gem25pro.desc":    "Most capable",
		"wizard.model.mistral_lg.desc":  "Most capable",
		"wizard.model.mistral_md.desc":  "Balanced",
		"wizard.model.mistral_sm.desc":  "Fast and affordable",
		"wizard.model.llama31.desc":     "Good general purpose",
		"wizard.model.qwen25.desc":      "Optimized for code",
		"wizard.model.deepseek.desc":    "Reasoning model",
		"wizard.model.mistral7b.desc":   "Fast and capable",

		// Capability descriptions
		"wizard.cap.thinking.desc":     "Extended thinking / chain-of-thought",
		"wizard.cap.vision.desc":       "Image / multimodal input",
		"wizard.cap.tool_use.desc":     "Function / tool calling",
		"wizard.cap.coding.desc":       "Code generation optimized",
		"wizard.cap.long_context.desc": ">100K token context",
		"wizard.cap.fast.desc":         "Low-latency inference",
		"wizard.cap.cheap.desc":        "Cost-optimized",
		"wizard.cap.writing.desc":      "Text / content generation",

		// Field labels
		"wizard.field.configured": "Configured",
		"wizard.field.driver":     "Driver",
		"wizard.field.alias":      "Alias",
		"wizard.field.model":      "Model",
		"wizard.field.base_url":   "Base URL",
		"wizard.field.caps":       "Capabilities",
		"wizard.field.tags":       "Tags",

		// Confirm
		"wizard.confirm.title":        "  Configuration Summary",
		"wizard.confirm.apply":        "Apply this configuration?",
		"wizard.confirm.provider":     "Provider",
		"wizard.confirm.default":      " (default)",
		"wizard.confirm.base_url":     "  Base URL",
		"wizard.confirm.api_key":      "  API Key",
		"wizard.confirm.caps":         "  Caps",
		"wizard.confirm.tags":         "  Tags",
		"wizard.confirm.prompt":       "  Prompt",
		"wizard.confirm.gateway":      "Gateway",
		"wizard.confirm.key.none":     "not required",
		"wizard.confirm.key.provided": "provided (will be encrypted)",
		"wizard.confirm.key.skipped":  "skipped (set later)",

		// Gateway
		"wizard.gateway.host":       "Gateway host",
		"wizard.gateway.port":       "Gateway port",
		"wizard.gateway.field.host": "Host",

		// Default provider
		"wizard.default.which": "Which provider should be the default?",

		// Finalize
		"wizard.final.ready":         "\n  Ozzie is ready.\n\n",
		"wizard.final.home":          "  Home:     %s\n",
		"wizard.final.gateway":       "  Gateway:  %s:%d\n",
		"wizard.final.default":       "  Default:  %s\n\n",
		"wizard.final.key_encrypted": " — key encrypted",
		"wizard.final.key_later":     " — set key later: ozzie secret set %s",
		"wizard.final.run":           "\n  Run: ozzie gateway\n",
	})
}
