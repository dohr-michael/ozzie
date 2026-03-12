# Ozzie — Architecture

Ozzie est un Agent OS personnel. Un seul process persistant (`ozzie gateway`) orchestre tout.
Les clients se connectent via WebSocket ou connecteurs externes.

L'architecture s'articule autour de 3 organes, le cerveau étant lui-même composé de 7 sous-systèmes :

```
         ┌──────────────┐
         │  Eyes         │  Connecteurs (Discord, TUI, Web, ...)
         └──────┬───────┘  internal/eyes/
                │ events
         ┌──────▼──────────────────────────────────────────────────────────────┐
         │  Brain                                              internal/brain/ │
         │                                                                     │
         │  ┌────────────────────────────────────────────────────────────────┐ │
         │  │ Nervous System — Event Loop (EventBus)            events/     │ │
         │  │ Transport de tous les signaux                                  │ │
         │  └────────────────────────────────────────────────────────────────┘ │
         │                                                                     │
         │  ┌────────────┐  ┌──────────┐  ┌────────┐  ┌───────────┐          │
         │  │ Identity    │  │ Cognitive│  │ Memory │  │ Skills    │          │
         │  │ Personality │  │ Actor    │  │Semantic│  │ Workflows │          │
         │  │ Self-aware  │  │ Pool     │  │Layered │  │ DAG       │          │
         │  │  prompt/    │  │ actors/  │  │memory/ │  │ skills/   │          │
         │  └────────────┘  └──────────┘  └────────┘  └───────────┘          │
         │                                                                     │
         │  ┌──────────────────┐  ┌──────────────────────┐                    │
         │  │ Conscience        │  │ Introspection         │                   │
         │  │ Approval/Sandbox  │  │ Logging/Observability │                   │
         │  │ conscience/       │  │ introspection/        │                   │
         │  └──────────────────┘  └──────────────────────┘                    │
         └──────────────────────────────┬──────────────────────────────────────┘
                                        │ tool calls
         ┌──────────────────────────────▼───────┐
         │  Hands                                │  Tools (registry, native, WASM, MCP)
         └──────────────────────────────────────┘  internal/hands/
```

---

# Eyes — Connecteurs

Comment Ozzie perçoit le monde extérieur et interagit avec les utilisateurs.

## Principe

- **Zero trust** : tout connecteur doit être paired à l'agent via `approve_pairing`
- **Multi-provider** : Discord, Slack, Teams, Web, TUI, ...
- **Identity mapping** : chaque utilisateur externe est associé à une policy (admin, support, executor, readonly)
- **Async approval** : les demandes de pairing transitent par l'admin channel

## Flux

```
Utilisateur externe
    │
    ▼
Connector (Discord, TUI, ...)
    │
    ├─ Pairing request → admin approval → policy assigned
    │
    ▼
Event Bus ──► EventRunner ──► Session ──► Réponse
```

## Session mapping

- Chaque conversation externe est mappée à une session Ozzie
- Le mapping est persisté dans `~/.ozzie/connectors/`
- La policy détermine les droits : tools autorisés, skills accessibles, mode d'approbation

---

# Brain

Le centre de décision d'Ozzie, composé de 5 sous-systèmes.

## Nervous System — Event Loop

Le système nerveux : tous les signaux transitent par lui.

- Tout est un event : interaction utilisateur, trigger, thinking, streaming, tool call, tool result
- Le `EventBus` (interface, implémentation in-memory) est le bus central
- Les composants communiquent exclusivement par events, jamais par appels directs
- L'`EventRunner` orchestre le cycle ReAct : prompt → LLM → tool calls → résultat → boucle
- Event persistence : chaque event est loggé dans `~/.ozzie/logs/` (JSONL)

## Identity — Personnalité & conscience de soi (`internal/brain/prompt/`)

Ce qui fait qu'Ozzie sait **qui il est** et **ce dont il dispose** à un instant T, avant même de réfléchir.

### Personnalité

- **Persona** : le caractère d'Ozzie — pragmatique, direct, dry wit, "friend in the lab"
- Overridable via `SOUL.md` dans `OZZIE_PATH` (custom personality)
- Variantes compactes pour les petits modèles (TierSmall)

### Conscience de soi (mémoire superficielle)

Ce qu'Ozzie sait de lui-même à chaque instant — pas des souvenirs, mais une conscience de ses capacités :

- Quels **tools** sont actifs / disponibles à activer
- Quelles **skills** il maîtrise
- Quels **actors** il peut déléguer
- Quel **environnement** runtime il utilise (container/local, system tools)
- Quelle **session** est en cours (working dir, language, titre)
- Quelles **instructions custom** l'utilisateur a configurées

### Implémentation

- `Registry` : toutes les templates nommées (persona, instructions, extraction, summarization)
- `Composer` : assemble les sections avec traçabilité (`LogManifest` → slog.Debug)
- Section builders : fonctions pures, zéro dépendance interne
- Le `ContextMiddleware` utilise le Composer pour injecter 6 layers statiques + 3 layers dynamiques

## Cognitive — Actor Pool

La partie pensante : raisonnement, prise de décision, coordination.

- Pool d'acteurs (agents LLM) avec gestion de capacité
- Quand une tâche arrive, un acteur est sélectionné selon ses tags/capabilities
- L'acteur principal gère la conversation ; les sub-agents exécutent les tâches de fond
- `AgentFactory` crée un runner frais par tour (nécessaire car Eino ADK freeze les tools)

## LLM / SLM

### Configuration

- **Provider** : Anthropic, Gemini, OpenAI, Mistral, Ollama (5 drivers)
- **Model** : Claude, GPT-4, Gemini, Mistral, modèles locaux, ...
- **Capabilities** : Thinking, Image, Code, Fast, Cheap, Tools-calling, ...
- **Tags** : Sélecteurs user-defined (self-hosted, gdpr, ...)
- **Instances** : Nombre d'instances par provider
- **Auth** : Résolution en cascade : config → env var → driver default. Ollama : pas d'auth

### Model Tiers

L'adaptation du prompt dépend du tier du modèle :

| Tier | Persona | Instructions | Memories | Skills |
|------|---------|-------------|----------|--------|
| Large / Medium | Complètes | Complètes | 5 max | Toutes |
| Small | Compactes (~40-60% shorter) | Compactes | 3 max, tronquées | 5 max |

### Context Compression

Quand la conversation dépasse 80% du context window :

- Split : messages anciens (à résumer) vs récents (25% du budget, préservés)
- Summarization LLM cumulative : intègre le résumé précédent + nouveaux messages
- Fallback : troncation si le LLM échoue
- Résumé persisté dans la session pour les reconnexions

---

# Brain > Memory — Mémoire

Comment Ozzie apprend et se souvient.

## Mémoire sémantique (`pkg/memory/`, wiring: `internal/brain/memory/bridge/`)

Stockage long-terme dans SQLite (pure Go, `modernc.org/sqlite`).

### Modèle de données

Chaque mémoire a :

- **Type** : `preference` | `fact` | `procedure` | `context`
- **Importance** : contrôle le decay — `core` (permanent) | `important` | `normal` | `ephemeral`
- **Confidence** : [0.0 - 1.0], décroît avec le temps selon l'importance
- **Tags** : labels pour la recherche
- **Source** : origine (`user`, `task:xxx`, `consolidation`)

### Decay temporel

| Importance | Grâce | Taux / semaine | Plancher |
|------------|-------|----------------|----------|
| core | ∞ | 0 | — |
| important | 30j | 0.005 | 0.3 |
| normal | 7j | 0.01 | 0.1 |
| ephemeral | 1j | 0.05 | 0.1 |

Les mémoires consultées sont renforcées (+0.05 de confidence, reset du timer).

### Recherche hybride

```
Score = 0.3 × keyword(BM25/FTS5) + 0.7 × semantic(cosine)
```

- **Keyword** : FTS5 full-text search (Porter stemmer + unicode61)
- **Semantic** : Embeddings vectoriels + brute-force cosine similarity
- Seuil minimum : 0.25

### Embedding pipeline

- Queue async single-worker, non-bloquante
- Hot-reload (changement de modèle/provider sans restart)
- Drivers : OpenAI, Gemini, Mistral, Ollama
- Staleness detection : re-index si modèle changé ou contenu modifié

### Consolidation LLM

- Détecte les mémoires similaires (cosine ≥ 0.85)
- Fusionne via LLM en une entrée consolidée
- Sources marquées `merged_into` → exclues des requêtes

### Apprentissage cross-task

- L'`Extractor` écoute les events `task.completed`
- Extrait 0-3 leçons réutilisables via LLM (patterns, conventions, gotchas)
- Dedup : skip si mémoire similaire existante (score ≥ 0.65)
- Stockées comme `procedure` pour les sessions futures

## Layered Context (`internal/brain/memory/layered/`)

Compression hiérarchique de l'historique des conversations longues.

### 3 niveaux

| Layer | Contenu | Budget tokens |
|-------|---------|---------------|
| L0 | Abstract : 1-2 phrases | ~120 |
| L1 | Summary : bullet points structurés | ~1200 |
| L2 | Transcript : conversation complète | illimité |

### Fonctionnement

```
Messages archivés
    │
    ▼
Chunking (8 messages/chunk)
    │
    ▼
Par chunk : Transcript → L1 summary → L0 abstract + keywords
    │                    (caché via SHA1 checksum)
    ▼
Index persisté : root + nodes avec L0/L1/keywords
```

### Retrieval (BM25 + recency)

Escalation progressive selon la confiance :

1. **L0** : BM25 sur abstracts + bonus recency (+0.08 max) → si score ≥ 0.64, retourner top 3 abstracts
2. **L1** : Re-score sur summaries → si marge top1-top2 ≥ 0.08, retourner summaries
3. **L2** : Charger les transcripts complets pour les top 2 candidats

Budget : 45% du context window (min 400 tokens).

---

# Brain > Skills — Compétences

Les capacités apprises d'Ozzie — workflows structurés et progressive disclosure.

## Principle

- Les skills sont des définitions déclaratives (fichiers `SKILL.md`) avec instructions, tools requis, et optionnellement un workflow DAG
- **Progressive disclosure** : les skills sont listés dans le prompt mais pas chargés. L'agent utilise `activate_skill(name)` pour charger les instructions complètes quand pertinent
- Séparation connaissance (skill body) vs exécution (workflow DAG)

## Structure d'un skill

```
skills/
  my-skill/
    SKILL.md          # instructions + metadata (YAML frontmatter)
    scripts/           # optional: scripts référencés
    resources/         # optional: assets
```

**Frontmatter** : name, description, tools (required), triggers (cron, on_event), workflow (steps DAG), acceptance_criteria

## Workflow DAG

- Chaque step est exécuté par un sub-agent éphémère avec ses propres instructions et tools
- Steps sans dépendances → exécution parallèle
- Variables injectées via `run_workflow(skill_name, vars)`
- Acceptance criteria optionnels : vérification LLM post-step

## Triggers

| Type | Description |
|------|-------------|
| `cron` | Expression cron standard |
| `interval` | Intervalle fixe (min 5s) |
| `on_event` | Déclenché par un event bus (avec filtre optionnel) |

Les schedules skill-based sont gérés par le scheduler et ne peuvent pas être supprimés via `unschedule_task`.

---

# Hands — Tools

Comment Ozzie agit sur le monde. Trois niveaux de confiance.

## Safe

Exécution directe, pas de demande d'approbation.

| Tool | Catégorie | Description |
|------|-----------|-------------|
| web_search | Web | Recherche web (DuckDuckGo/Google/Bing) — conditionnellement enregistré |
| store_memory | Memory | Stocke une mémoire long-terme (preference/fact/procedure/context) |
| query_memories | Memory | Recherche hybride keyword+vector dans les mémoires |
| forget_memory | Memory | Supprime une entrée mémoire |
| submit_task | Tasks | Soumet une tâche async en background |
| check_task | Tasks | Vérifie le statut/progrès d'une tâche |
| cancel_task | Tasks | Annule une tâche running/pending |
| plan_task | Tasks | Crée un plan DAG de sous-tâches avec dépendances |
| list_tasks | Tasks | Liste les tâches (filtrable par status/session) |
| schedule_task | Schedule | Crée une tâche récurrente (cron/interval/event) |
| unschedule_task | Schedule | Supprime un schedule dynamique |
| list_schedules | Schedule | Liste tous les schedules |
| trigger_schedule | Schedule | Déclenche manuellement un schedule existant |
| activate_skill | Skills | Charge les instructions d'un skill + active ses tools |
| run_workflow | Skills | Exécute le DAG workflow d'un skill |
| activate_tools | Control | Active des tools on-demand (MCP principalement) |
| update_session | Control | Met à jour les metadata de session |
| approve_pairing | Control | Approuve un pairing utilisateur depuis une plateforme externe |
| ls | Filesystem | Liste les fichiers/dossiers d'un dossier |
| read_file | Filesystem | Lit un fichier |
| glob | Filesystem | Recherche de fichiers par pattern glob |
| grep | Filesystem | Recherche dans le contenu des fichiers |

## Dangerous — require user approval (allow once / always for session / deny)

Le `DangerousToolWrapper` intercepte l'appel et prompt l'utilisateur.
Les approbations "always" sont persistées dans `Session.ApprovedTools`.
Les sub-tasks héritent des approbations de la session parente.
Pre-approval possible via `submit_task` et `schedule_task`.

| Tool | Catégorie | Description |
|------|-----------|-------------|
| run_command | Execution | Exécution shell avec sudo optionnel, timeout configurable |
| git | Execution | Opérations git (status, diff, log, add, commit, branch, checkout) |
| web_fetch | Web | Fetch une page web et extrait le texte |
| MCP tools | External | Unsafe par défaut, configurable `trusted_tools` par serveur MCP |
| Plugins | External | Unsafe par défaut (to review) |

## Sandboxed — si dans le working directory → OK, sinon → dangerous

Hybrid : pas d'approbation dans le sandbox (WorkDir + paths autorisés),
approbation requise hors sandbox, hard block en mode autonome.

| Tool | Catégorie | Description |
|------|-----------|-------------|
| write_file | Filesystem | Remplace un fichier |
| edit_file | Filesystem | Edite un fichier texte |
| str_replace_editor | Filesystem | Éditeur riche (view, create, str_replace, insert, undo_edit) |

## Core vs On-demand

- **Core** (toujours actifs) : tous les native tools sauf `run_workflow`
- **On-demand** (via `activate_tools`) : tools MCP, `run_workflow`
- L'`AgentFactory` recrée un runner par tour pour intégrer les tools fraîchement activés
