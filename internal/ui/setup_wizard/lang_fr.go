package setup_wizard

import "github.com/dohr-michael/ozzie/internal/i18n"

func init() {
	i18n.Register("fr", map[string]string{
		// Wizard chrome
		"wizard.title":         "  Configuration d'Ozzie",
		"wizard.applying":      "  Application de la configuration...",
		"wizard.step_progress": "  Étape %d/%d — %s",

		// Welcome
		"wizard.welcome.title":       "  Bienvenue sur Ozzie !",
		"wizard.welcome.subtitle":    "  Configurons votre agent OS. Appuyez sur entrée pour continuer...",
		"wizard.welcome.existing_q":  "Configuration existante trouvée. Que souhaitez-vous faire ?",
		"wizard.welcome.load":        "Charger et modifier",
		"wizard.welcome.load.desc":   "Pré-remplir depuis la config actuelle",
		"wizard.welcome.fresh":       "Repartir de zéro",
		"wizard.welcome.fresh.desc":  "Écraser avec une nouvelle config",
		"wizard.welcome.cancel":      "Annuler",
		"wizard.welcome.cancel.desc": "Quitter sans changement",
		"wizard.welcome.language":    "Language / Langue",

		// Provider
		"wizard.provider.choose":               "Choisir le fournisseur LLM",
		"wizard.provider.alias":                "Alias du fournisseur (clé de config)",
		"wizard.provider.model":                "Choisir un modèle",
		"wizard.provider.custom":               "Modèle personnalisé",
		"wizard.provider.custom.desc":          "Saisir un ID de modèle manuellement",
		"wizard.provider.model_id":             "Saisir l'ID du modèle",
		"wizard.provider.api_key_for":          "Clé API pour %s",
		"wizard.provider.key_now":              "Saisir la clé API maintenant",
		"wizard.provider.key_now.desc":         "Sera chiffrée avec age",
		"wizard.provider.key_later":            "Je la définirai plus tard",
		"wizard.provider.key_later.desc":       "Utiliser : ozzie secret set %s",
		"wizard.provider.key_reuse":            "Réutiliser la clé de %s",
		"wizard.provider.key_reuse.desc":       "Partager %s",
		"wizard.provider.key_new":              "Saisir une nouvelle clé API",
		"wizard.provider.key_new.desc":         "Clé séparée, stockée sous %s",
		"wizard.provider.enter_key":            "Saisissez votre %s :",
		"wizard.provider.caps":                 "Capacités (espace=basculer, entrée=confirmer)",
		"wizard.provider.tags":                 "Tags (optionnel, séparés par des virgules — ex. self-hosted, secured, primary)",
		"wizard.provider.prompt":               "Prompt système (optionnel, instruction personnalisée pour ce fournisseur)",
		"wizard.provider.add_more":             "Ajouter un autre fournisseur LLM ?",
		"wizard.provider.base_url":             "URL de base",
		"wizard.provider.base_url.ollama":      "URL de base Ollama",
		"wizard.provider.base_url.openai-like": "URL de base pour les API compatibles OpenAI",

		// Driver descriptions
		"wizard.driver.anthropic.desc":   "Modèles Claude, idéaux pour le code",
		"wizard.driver.openai.desc":      "Modèles GPT",
		"wizard.driver.openai-like.desc": "API compatibles OpenAI",
		"wizard.driver.gemini.desc":      "Modèles Gemini",
		"wizard.driver.mistral.desc":     "Modèles Mistral, basés en UE",
		"wizard.driver.ollama.desc":      "Modèles locaux, pas de clé API nécessaire",

		// Model descriptions
		"wizard.model.sonnet4.desc":    "Meilleur équilibre vitesse/qualité",
		"wizard.model.opus4.desc":      "Le plus performant",
		"wizard.model.haiku4.desc":     "Rapide et abordable",
		"wizard.model.gpt4o.desc":      "Modèle multimodal phare",
		"wizard.model.gpt4omini.desc":  "Rapide et abordable",
		"wizard.model.o3.desc":         "Modèle de raisonnement",
		"wizard.model.gem25flash.desc": "Rapide et polyvalent",
		"wizard.model.gem25pro.desc":   "Le plus performant",
		"wizard.model.mistral_lg.desc": "Le plus performant",
		"wizard.model.mistral_md.desc": "Équilibré",
		"wizard.model.mistral_sm.desc": "Rapide et abordable",
		"wizard.model.llama31.desc":    "Bon usage général",
		"wizard.model.qwen25.desc":     "Optimisé pour le code",
		"wizard.model.deepseek.desc":   "Modèle de raisonnement",
		"wizard.model.mistral7b.desc":  "Rapide et performant",

		// Capability descriptions
		"wizard.cap.thinking.desc":     "Réflexion étendue / chaîne de pensée",
		"wizard.cap.vision.desc":       "Image / entrée multimodale",
		"wizard.cap.tool_use.desc":     "Appel de fonctions / outils",
		"wizard.cap.coding.desc":       "Génération de code optimisée",
		"wizard.cap.long_context.desc": "Contexte >100K tokens",
		"wizard.cap.fast.desc":         "Inférence à faible latence",
		"wizard.cap.cheap.desc":        "Optimisé en coût",
		"wizard.cap.writing.desc":      "Génération de texte / contenu",

		// Field labels
		"wizard.field.configured": "Configurés",
		"wizard.field.driver":     "Driver",
		"wizard.field.alias":      "Alias",
		"wizard.field.model":      "Modèle",
		"wizard.field.base_url":   "URL de base",
		"wizard.field.caps":       "Capacités",
		"wizard.field.tags":       "Tags",

		// Confirm
		"wizard.confirm.title":        "  Résumé de la configuration",
		"wizard.confirm.apply":        "Appliquer cette configuration ?",
		"wizard.confirm.provider":     "Fournisseur",
		"wizard.confirm.default":      " (par défaut)",
		"wizard.confirm.base_url":     "  URL de base",
		"wizard.confirm.api_key":      "  Clé API",
		"wizard.confirm.caps":         "  Capacités",
		"wizard.confirm.tags":         "  Tags",
		"wizard.confirm.prompt":       "  Prompt",
		"wizard.confirm.gateway":      "Gateway",
		"wizard.confirm.key.none":     "non requise",
		"wizard.confirm.key.provided": "fournie (sera chiffrée)",
		"wizard.confirm.key.skipped":  "ignorée (à définir plus tard)",

		// Embedding step
		"wizard.embedding.keep":            "Conserver la config d'embedding actuelle ?",
		"wizard.embedding.enable":          "Activer la mémoire sémantique (embeddings) ?",
		"wizard.embedding.enable.desc":     "Optionnel — active la recherche sémantique dans les conversations",
		"wizard.embedding.driver":          "Choisir le fournisseur d'embeddings",
		"wizard.embedding.model":           "Choisir un modèle d'embedding",
		"wizard.embedding.custom":          "Modèle personnalisé",
		"wizard.embedding.custom.desc":     "Saisir un ID de modèle manuellement",
		"wizard.embedding.model_id":        "Saisir l'ID du modèle d'embedding",
		"wizard.embedding.base_url":        "URL de base",
		"wizard.embedding.base_url.ollama": "URL de base Ollama",
		"wizard.embedding.dims":            "Dimensions de l'embedding",
		"wizard.embedding.key_for":         "Clé API pour %s",
		"wizard.embedding.key_reuse":       "Réutiliser du fournisseur LLM (%s)",
		"wizard.embedding.key_reuse.desc":  "Partager la même clé API",
		"wizard.embedding.key_new":         "Saisir une nouvelle clé API",
		"wizard.embedding.key_new.desc":    "Clé séparée, stockée sous %s",
		"wizard.embedding.key_later":       "Je la définirai plus tard",
		"wizard.embedding.key_later.desc":  "Utiliser : ozzie secret set %s",
		"wizard.embedding.enter_key":       "Saisissez votre %s :",

		// Embedding driver descriptions
		"wizard.driver.openai_emb.desc":  "Modèles d'embedding OpenAI",
		"wizard.driver.ollama_emb.desc":  "Modèles d'embedding locaux",
		"wizard.driver.mistral_emb.desc": "Embedding Mistral",
		"wizard.driver.gemini_emb.desc":  "Modèles d'embedding Google",

		// Embedding model descriptions
		"wizard.emb_model.oai_small.desc": "Meilleur équilibre qualité/coût",
		"wizard.emb_model.oai_large.desc": "Meilleure qualité",
		"wizard.emb_model.oai_ada.desc":   "Modèle historique",
		"wizard.emb_model.nomic.desc":     "Bonne qualité, rapide",
		"wizard.emb_model.mxbai.desc":     "Haute qualité",
		"wizard.emb_model.minilm.desc":    "Le plus rapide, le plus petit",
		"wizard.emb_model.mistral.desc":   "Embedding Mistral",
		"wizard.emb_model.gem001.desc":    "Embedding Google",
		"wizard.emb_model.gem004.desc":    "Google historique",

		// Confirm embedding
		"wizard.confirm.embedding":    "Embedding",
		"wizard.confirm.emb_disabled": "désactivé",
		"wizard.confirm.emb_reuses":   "réutilise %s",

		// Final embedding
		"wizard.final.embedding":    "\n  Embedding : %s — %s (%d dims)\n",
		"wizard.final.emb_disabled": "\n  Embedding : désactivé\n",

		// Layered context step
		"wizard.layered.keep":              "Conserver la config de layered context actuelle ?",
		"wizard.layered.enable":            "Activer le layered context (compression de conversations) ?",
		"wizard.layered.enable.desc":       "Compression intelligente pour les longues conversations — résume les anciens messages en chunks archivés, économisant 80-90% des tokens tout en préservant le contexte",
		"wizard.layered.max_recent":        "Messages récents conservés sans compression",
		"wizard.layered.max_recent.desc":   "Nombre de messages récents toujours envoyés en entier (défaut : 24)",
		"wizard.layered.max_archives":      "Chunks archivés max par session",
		"wizard.layered.max_archives.desc": "Nombre de chunks de résumé compressés conservés (défaut : 12)",

		// Confirm layered context
		"wizard.confirm.layered":          "Layered ctx",
		"wizard.confirm.layered_disabled": "désactivé",

		// Final layered context
		"wizard.final.layered":          "\n  Layered ctx : activé (récents=%d, archives=%d)\n",
		"wizard.final.layered_disabled": "\n  Layered ctx : désactivé\n",

		// MCP server step
		"wizard.mcp.enable":               "Ajouter un serveur MCP ?",
		"wizard.mcp.enable.desc":          "Optionnel — connecter des outils externes via Model Context Protocol",
		"wizard.mcp.keep":                 "Conserver la config des serveurs MCP ?",
		"wizard.mcp.name":                 "Nom du serveur (alias)",
		"wizard.mcp.name.desc":            "ex. github, filesystem, postgres",
		"wizard.mcp.transport":            "Protocole de transport",
		"wizard.mcp.transport.stdio.desc": "Lancer une commande locale (le plus courant)",
		"wizard.mcp.transport.sse.desc":   "Connexion via Server-Sent Events",
		"wizard.mcp.transport.http.desc":  "Connexion via HTTP streaming",
		"wizard.mcp.command":              "Commande à exécuter",
		"wizard.mcp.command.desc":         "ex. npx, uvx, docker",
		"wizard.mcp.args":                 "Arguments de la commande",
		"wizard.mcp.args.desc":            "Séparés par espaces, ex. -y @modelcontextprotocol/server-github",
		"wizard.mcp.url":                  "URL du serveur",
		"wizard.mcp.env_ask":              "Ajouter une variable d'environnement ?",
		"wizard.mcp.env_ask.desc":         "Pour les tokens d'auth, valeurs de config, etc.",
		"wizard.mcp.env_name":             "Nom de la variable",
		"wizard.mcp.env_value":            "Valeur de la variable (optionnel)",
		"wizard.mcp.env_value.desc":       "Laisser vide pour définir plus tard avec ozzie secret set",
		"wizard.mcp.env_secret":           "Est-ce un secret (chiffrer dans .env) ?",
		"wizard.mcp.probe":                "Se connecter au serveur et découvrir les tools ?",
		"wizard.mcp.probe.desc":           "Va démarrer le serveur pour lister les tools disponibles pour la config de confiance",
		"wizard.mcp.connecting":           "Connexion à %s...",
		"wizard.mcp.probe_failed":         "Connexion échouée : %s. Découverte des tools ignorée.",
		"wizard.mcp.trusted_tools":        "Sélectionner les tools de confiance (contournent la confirmation dangerous)",
		"wizard.mcp.add_more":             "Ajouter un autre serveur MCP ?",
		"wizard.mcp.field.name":           "Nom",
		"wizard.mcp.field.transport":      "Transport",
		"wizard.mcp.field.command":        "Commande",
		"wizard.mcp.field.args":           "Args",
		"wizard.mcp.field.url":            "URL",
		"wizard.mcp.field.env_vars":       "Vars env",
		"wizard.mcp.field.trusted":        "Confiance",

		// Confirm MCP
		"wizard.confirm.mcp":      "Serveur MCP",
		"wizard.confirm.mcp_none": "aucun configuré",

		// Final MCP
		"wizard.final.mcp":      "\n  MCP : %d serveur(s)\n",
		"wizard.final.mcp_none": "\n  MCP : aucun serveur\n",

		// Gateway
		"wizard.gateway.host":       "Hôte du gateway",
		"wizard.gateway.port":       "Port du gateway",
		"wizard.gateway.field.host": "Hôte",

		// Default provider
		"wizard.default.which": "Quel fournisseur utiliser par défaut ?",

		// Finalize
		"wizard.final.ready":         "\n  Ozzie est prêt.\n\n",
		"wizard.final.home":          "  Dossier :  %s\n",
		"wizard.final.gateway":       "  Gateway :  %s:%d\n",
		"wizard.final.default":       "  Défaut :   %s\n\n",
		"wizard.final.key_encrypted": " — clé chiffrée",
		"wizard.final.key_later":     " — définir la clé plus tard : ozzie secret set %s",
		"wizard.final.run":           "\n  Lancer : ozzie gateway\n",
	})
}
