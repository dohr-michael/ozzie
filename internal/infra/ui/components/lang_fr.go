package components

import "github.com/dohr-michael/ozzie/internal/infra/i18n"

func init() {
	i18n.Register("fr", map[string]string{
		// Shared hints
		"hint.quit":        "ctrl+c=quitter",
		"hint.back":        "esc=retour",
		"hint.submit_back": "entrée=soumettre • esc=retour",

		// Confirm
		"confirm.yes": "Oui",
		"confirm.no":  "Non",

		// Input
		"input.placeholder.chat":  "Tapez un message...",
		"input.placeholder.value": "Entrez une valeur...",
		"input.error.required":    "Ce champ est obligatoire",
		"input.error.invalid":     "Format invalide",
		"input.waiting":           "  En attente de réponse...",

		// Hints
		"hint.text":    "entrée=soumettre • esc=annuler",
		"hint.select":  "↑↓=naviguer • entrée=sélectionner • esc=annuler",
		"hint.multi":   "↑↓=naviguer • espace=basculer • entrée=soumettre • esc=annuler",
		"hint.confirm": "y/n ou ↑↓ + entrée • esc=annuler",
		"hint.scroll":  "↑↓=défiler",

		// Chat
		"chat.thinking":        "Réflexion en cours...",
		"chat.welcome.tagline": " — Votre système d'exploitation IA personnel.",
		"chat.tips.quit":       "  Astuce : Ctrl+C pour quitter",
		"chat.tool.no_output":  "(Pas de sortie)",
		"chat.tool.more_lines": "... (%d lignes supplémentaires)",
		"chat.tool.awaiting":   " (en attente de confirmation)",
		"chat.tool.denied":     " (refusé)",

		// Header
		"header.tokens":    " tokens",
		"header.streaming": "● streaming",

		// Roles
		"role.system": "Système : ",
	})
}
