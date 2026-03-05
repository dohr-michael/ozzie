package wizard

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
		"wizard.provider.choose":         "Choisir le fournisseur LLM",
		"wizard.provider.alias":          "Alias du fournisseur (clé de config)",
		"wizard.provider.model":          "Choisir un modèle",
		"wizard.provider.custom":         "Modèle personnalisé",
		"wizard.provider.custom.desc":    "Saisir un ID de modèle manuellement",
		"wizard.provider.model_id":       "Saisir l'ID du modèle",
		"wizard.provider.api_key_for":    "Clé API pour %s",
		"wizard.provider.key_now":        "Saisir la clé API maintenant",
		"wizard.provider.key_now.desc":   "Sera chiffrée avec age",
		"wizard.provider.key_later":      "Je la définirai plus tard",
		"wizard.provider.key_later.desc": "Utiliser : ozzie secret set %s",
		"wizard.provider.key_reuse":      "Réutiliser la clé de %s",
		"wizard.provider.key_reuse.desc": "Partager %s",
		"wizard.provider.key_new":        "Saisir une nouvelle clé API",
		"wizard.provider.key_new.desc":   "Clé séparée, stockée sous %s",
		"wizard.provider.enter_key":      "Saisissez votre %s :",
		"wizard.provider.caps":           "Capacités (espace=basculer, entrée=confirmer)",
		"wizard.provider.tags":           "Tags (optionnel, séparés par des virgules — ex. self-hosted, secured, primary)",
		"wizard.provider.prompt":         "Prompt système (optionnel, instruction personnalisée pour ce fournisseur)",
		"wizard.provider.add_more":       "Ajouter un autre fournisseur LLM ?",
		"wizard.provider.base_url":       "URL de base",
		"wizard.provider.base_url.ollama":  "URL de base Ollama",
		"wizard.provider.base_url.openai":  "URL de base (optionnel, pour les API compatibles OpenAI)",

		// Driver descriptions
		"wizard.driver.anthropic.desc": "Modèles Claude, idéaux pour le code",
		"wizard.driver.openai.desc":    "Modèles GPT (+ API compatibles OpenAI)",
		"wizard.driver.gemini.desc":    "Modèles Gemini",
		"wizard.driver.mistral.desc":   "Modèles Mistral, basés en UE",
		"wizard.driver.ollama.desc":    "Modèles locaux, pas de clé API nécessaire",

		// Model descriptions
		"wizard.model.sonnet4.desc":     "Meilleur équilibre vitesse/qualité",
		"wizard.model.opus4.desc":       "Le plus performant",
		"wizard.model.haiku4.desc":      "Rapide et abordable",
		"wizard.model.gpt4o.desc":       "Modèle multimodal phare",
		"wizard.model.gpt4omini.desc":   "Rapide et abordable",
		"wizard.model.o3.desc":          "Modèle de raisonnement",
		"wizard.model.gem25flash.desc":  "Rapide et polyvalent",
		"wizard.model.gem25pro.desc":    "Le plus performant",
		"wizard.model.mistral_lg.desc":  "Le plus performant",
		"wizard.model.mistral_md.desc":  "Équilibré",
		"wizard.model.mistral_sm.desc":  "Rapide et abordable",
		"wizard.model.llama31.desc":     "Bon usage général",
		"wizard.model.qwen25.desc":      "Optimisé pour le code",
		"wizard.model.deepseek.desc":    "Modèle de raisonnement",
		"wizard.model.mistral7b.desc":   "Rapide et performant",

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
