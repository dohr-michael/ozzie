package setup_wizard

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
		"wizard.provider.choose":          "Choose LLM provider",
		"wizard.provider.alias":           "Provider alias (config key)",
		"wizard.provider.model":           "Choose a model",
		"wizard.provider.custom":          "Custom model",
		"wizard.provider.custom.desc":     "Enter a model ID manually",
		"wizard.provider.model_id":        "Enter model ID",
		"wizard.provider.api_key_for":     "API Key for %s",
		"wizard.provider.key_now":         "Enter API key now",
		"wizard.provider.key_now.desc":    "Will be encrypted with age",
		"wizard.provider.key_later":       "I'll set it later",
		"wizard.provider.key_later.desc":  "Use: ozzie secret set %s",
		"wizard.provider.key_reuse":       "Reuse key from %s",
		"wizard.provider.key_reuse.desc":  "Share %s",
		"wizard.provider.key_new":         "Enter a new API key",
		"wizard.provider.key_new.desc":    "Separate key, stored as %s",
		"wizard.provider.enter_key":       "Enter your %s:",
		"wizard.provider.caps":            "Capabilities (space=toggle, enter=confirm)",
		"wizard.provider.tags":            "Tags (optional, comma-separated — e.g. self-hosted, secured, primary)",
		"wizard.provider.prompt":          "System prompt (optional, custom instruction for this provider)",
		"wizard.provider.add_more":        "Add another LLM provider?",
		"wizard.provider.base_url":        "Base URL",
		"wizard.provider.base_url.ollama": "Ollama base URL",
		"wizard.provider.base_url.openai": "Base URL (optional, for OpenAI-compatible APIs)",

		// Driver descriptions
		"wizard.driver.anthropic.desc": "Claude models, best for coding",
		"wizard.driver.openai.desc":    "GPT models (+ OpenAI-compatible APIs)",
		"wizard.driver.gemini.desc":    "Gemini models",
		"wizard.driver.mistral.desc":   "Mistral models, EU-based",
		"wizard.driver.ollama.desc":    "Local models, no API key needed",

		// Model descriptions
		"wizard.model.sonnet4.desc":    "Best balance of speed and quality",
		"wizard.model.opus4.desc":      "Most capable",
		"wizard.model.haiku4.desc":     "Fast and affordable",
		"wizard.model.gpt4o.desc":      "Flagship multimodal model",
		"wizard.model.gpt4omini.desc":  "Fast and affordable",
		"wizard.model.o3.desc":         "Reasoning model",
		"wizard.model.gem25flash.desc": "Fast and versatile",
		"wizard.model.gem25pro.desc":   "Most capable",
		"wizard.model.mistral_lg.desc": "Most capable",
		"wizard.model.mistral_md.desc": "Balanced",
		"wizard.model.mistral_sm.desc": "Fast and affordable",
		"wizard.model.llama31.desc":    "Good general purpose",
		"wizard.model.qwen25.desc":     "Optimized for code",
		"wizard.model.deepseek.desc":   "Reasoning model",
		"wizard.model.mistral7b.desc":  "Fast and capable",

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

		// Embedding step
		"wizard.embedding.keep":        "Keep current embedding config?",
		"wizard.embedding.enable":      "Enable semantic memory (embeddings)?",
		"wizard.embedding.enable.desc": "Optional — powers semantic search in conversations",
		"wizard.embedding.driver":      "Choose embedding provider",
		"wizard.embedding.model":       "Choose an embedding model",
		"wizard.embedding.custom":      "Custom model",
		"wizard.embedding.custom.desc": "Enter a model ID manually",
		"wizard.embedding.model_id":    "Enter embedding model ID",
		"wizard.embedding.base_url":       "Base URL",
		"wizard.embedding.base_url.ollama": "Ollama base URL",
		"wizard.embedding.dims":        "Embedding dimensions",
		"wizard.embedding.key_for":     "API Key for %s",
		"wizard.embedding.key_reuse":      "Reuse from LLM provider (%s)",
		"wizard.embedding.key_reuse.desc": "Share the same API key",
		"wizard.embedding.key_new":        "Enter a new API key",
		"wizard.embedding.key_new.desc":   "Separate key, stored as %s",
		"wizard.embedding.key_later":      "I'll set it later",
		"wizard.embedding.key_later.desc": "Use: ozzie secret set %s",
		"wizard.embedding.enter_key":      "Enter your %s:",

		// Embedding driver descriptions
		"wizard.driver.openai_emb.desc":   "OpenAI embedding models",
		"wizard.driver.ollama_emb.desc":   "Local embedding models",
		"wizard.driver.mistral_emb.desc":  "Mistral embedding",
		"wizard.driver.gemini_emb.desc":   "Google embedding models",

		// Embedding model descriptions
		"wizard.emb_model.oai_small.desc": "Best balance quality/cost",
		"wizard.emb_model.oai_large.desc": "Highest quality",
		"wizard.emb_model.oai_ada.desc":   "Legacy model",
		"wizard.emb_model.nomic.desc":     "Good quality, fast",
		"wizard.emb_model.mxbai.desc":     "High quality",
		"wizard.emb_model.minilm.desc":    "Fastest, smallest",
		"wizard.emb_model.mistral.desc":   "Mistral embedding",
		"wizard.emb_model.gem001.desc":    "Google embedding",
		"wizard.emb_model.gem004.desc":    "Google legacy",

		// Confirm embedding
		"wizard.confirm.embedding":    "Embedding",
		"wizard.confirm.emb_disabled": "disabled",
		"wizard.confirm.emb_reuses":   "reuses %s",

		// Final embedding
		"wizard.final.embedding":    "\n  Embedding: %s — %s (%d dims)\n",
		"wizard.final.emb_disabled": "\n  Embedding: disabled\n",

		// Layered context step
		"wizard.layered.keep":             "Keep current layered context config?",
		"wizard.layered.enable":           "Enable layered context (conversation compression)?",
		"wizard.layered.enable.desc":      "Smart compression for long conversations — summarizes older messages into archived chunks, saving 80-90% of tokens while preserving context",
		"wizard.layered.max_recent":       "Recent messages kept uncompressed",
		"wizard.layered.max_recent.desc":  "Number of recent messages always sent in full (default: 24)",
		"wizard.layered.max_archives":     "Max archived chunks per session",
		"wizard.layered.max_archives.desc": "Number of compressed summary chunks retained (default: 12)",

		// Confirm layered context
		"wizard.confirm.layered":          "Layered ctx",
		"wizard.confirm.layered_disabled": "disabled",

		// Final layered context
		"wizard.final.layered":          "\n  Layered ctx: enabled (recent=%d, archives=%d)\n",
		"wizard.final.layered_disabled": "\n  Layered ctx: disabled\n",

		// MCP server step
		"wizard.mcp.enable":                "Add an MCP server?",
		"wizard.mcp.enable.desc":           "Optional — connect external tools via Model Context Protocol",
		"wizard.mcp.keep":                  "Keep current MCP server config?",
		"wizard.mcp.name":                  "Server name (alias)",
		"wizard.mcp.name.desc":             "e.g. github, filesystem, postgres",
		"wizard.mcp.transport":             "Transport protocol",
		"wizard.mcp.transport.stdio.desc":  "Launch a local command (most common)",
		"wizard.mcp.transport.sse.desc":    "Connect via Server-Sent Events",
		"wizard.mcp.transport.http.desc":   "Connect via HTTP streaming",
		"wizard.mcp.command":               "Command to run",
		"wizard.mcp.command.desc":          "e.g. npx, uvx, docker",
		"wizard.mcp.args":                  "Command arguments",
		"wizard.mcp.args.desc":             "Space-separated, e.g. -y @modelcontextprotocol/server-github",
		"wizard.mcp.url":                   "Server endpoint URL",
		"wizard.mcp.env_ask":               "Add an environment variable?",
		"wizard.mcp.env_ask.desc":          "For auth tokens, config values, etc.",
		"wizard.mcp.env_name":              "Variable name",
		"wizard.mcp.env_value":             "Variable value (optional)",
		"wizard.mcp.env_value.desc":        "Leave empty to set later with ozzie secret set",
		"wizard.mcp.env_secret":            "Is this a secret (encrypt in .env)?",
		"wizard.mcp.probe":                 "Connect to server and discover tools?",
		"wizard.mcp.probe.desc":            "Will start the server to list available tools for trust configuration",
		"wizard.mcp.connecting":            "Connecting to %s...",
		"wizard.mcp.probe_failed":          "Connection failed: %s. Skipping tool discovery.",
		"wizard.mcp.trusted_tools":         "Select tools to trust (bypass dangerous confirmation)",
		"wizard.mcp.add_more":              "Add another MCP server?",
		"wizard.mcp.field.name":            "Name",
		"wizard.mcp.field.transport":       "Transport",
		"wizard.mcp.field.command":         "Command",
		"wizard.mcp.field.args":            "Args",
		"wizard.mcp.field.url":             "URL",
		"wizard.mcp.field.env_vars":        "Env vars",
		"wizard.mcp.field.trusted":         "Trusted",

		// Confirm MCP
		"wizard.confirm.mcp":      "MCP Server",
		"wizard.confirm.mcp_none": "none configured",

		// Final MCP
		"wizard.final.mcp":      "\n  MCP: %d server(s)\n",
		"wizard.final.mcp_none": "\n  MCP: no servers\n",

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
