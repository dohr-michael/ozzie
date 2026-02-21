# Cahier des Charges v2 : Ozzie

> **Ozzie** — **O**ntological **Z**ero-trust **Z**one for **I**ndependent **E**xecution
>
> *"Ozzie would have loved to dive right in and give himself a decent clean, but even though they still hadn't seen a single living creature on this world, he just couldn't quite bring himself to trust the water. Too many late student nights with a pizza, a couple of six-packs, some grass, and a bad sci-fi DVD. God only knew what lurked along the bottom of the river, maybe nothing, but he certainly wasn't going to wind up with alien eggs hatching out of his ass, thank you."*
> — Peter F. Hamilton, *Pandora's Star*
>
> Nommé d'après Ozzie Fernandez Isaacs, co-inventeur des wormholes (gateways entre mondes),
> créateur de la Sentient Intelligence, architecte du Gaiafield, et explorateur des Silfen Paths
> dans la saga du Commonwealth.

### Ozzie-isms — Citations de la saga

| Citation | Source | Thème |
|---|---|---|
| *"He just couldn't quite bring himself to trust the water."* | Pandora's Star | **Zero-trust** — même décontracté, on ne fait pas confiance à l'inconnu |
| *"Machines served life, it could not be otherwise."* | Pandora's Star | **Agent au service de l'humain** — la machine sert, jamais l'inverse |
| *"We just need some fresh resources, is all."* | Judas Unchained (Ozzie) | **Résilience** — chaque problème est un problème de ressources |
| *"Chain of command was always a nominal concept — you trusted people to do their job right."* | Pandora's Star | **Autonomie des agents** — décentralisation, capabilities |
| *"Oh, thank Ozzie!"* / *"For Ozzie's sake!"* | The Dreaming Void (~3500 AD) | Le personnage a atteint le **statut divin** dans le Commonwealth |

> **Tagline projet :** *Connect everything. Trust nothing.*

---

## 1. Vision du Projet

Créer un agent IA autonome, performant et sécurisé, capable de connecter des modèles de langage (LLM) à des interfaces de communication (Messaging) et des outils d'exécution (Tools).

**Principes directeurs :**

* **Performance :** Runtime Go pour un bon ratio performance/vélocité de développement.
* **Isolation Totale :** Exécution des extensions via WebAssembly (Sandboxing Extism/wazero).
* **Auditabilité :** Persistance des états en Markdown pour une lecture humaine directe.
* **Extensibilité :** Système de plugins multi-langage via Extism PDK.
* **Sécurité :** Deny-by-default, capability-based, isolation des secrets, model affinity enforcement via tags.

**Inspiration :**
* OpenClaw (architecture event-driven, session isolation, multi-input vectors)
* Ozzie Isaacs / Commonwealth Saga (Peter F. Hamilton)
* Différenciation : isolation Wasm (vs pas de sandbox par défaut), model routing par tags/affinity (vs provider unique), skills déclaratifs (vs code arbitraire)

**Parallèles avec le personnage :**

| Ozzie Isaacs (Commonwealth) | Ozzie (le projet) |
|---|---|
| Co-inventeur des **wormholes** (gateways entre mondes) | Un **Gateway** qui connecte LLM, tools et messaging |
| Créateur de la **Sentient Intelligence** (IA construite sur ses thought patterns) | Orchestre des **agents IA autonomes** |
| Architecte du **Gaiafield** (réseau d'émotions/expériences partagées entre êtres sentients) | **Event Bus** partagé entre tous les composants, **WS Hub** connectant les clients |
| Explorateur des **Silfen Paths** (chemins entre mondes, **désactivent l'électronique**) | **Plugins Wasm** isolés — pas d'accès au host, survie par resourcefulness |
| Ne fait pas confiance à l'eau d'un monde inconnu | Architecture **Zero-Trust**, sandbox Wasm, deny-by-default |
| **Friendship Pendant** pour accéder aux Silfen | **Capabilities** pour accéder aux ressources du host |
| Quitte le Commonwealth pour construire **The Spike** — un "galactic dream" | Vision long-terme : une plateforme **extensible à n'importe quel domaine** |
| Devenu une **figure quasi-divine** (~3500 AD) — "Thank Ozzie!" | Ambition : devenir **l'outil de référence** pour les agents IA self-hosted |

---

## 2. Architecture Technique (ADR)

### ADR 001 : Choix du Langage et Framework Web

* **Décision :** Go avec le routeur **go-chi**.
* **Contexte :** Node.js est jugé trop lourd pour des déploiements sur serveurs contraints. Rust offre les meilleures performances mais au prix d'une vélocité de développement réduite (temps de compilation, complexité du borrow checker). Go offre un compromis idéal : binaire unique, faible empreinte mémoire, goroutines natives, écosystème mature et itération rapide.
* **Conséquences :** Utilisation de `go-chi` pour le routage HTTP, middlewares standards `net/http`. Un seul binaire statique à déployer.

### ADR 002 : Abstraction des LLM

* **Décision :** Utilisation du framework **Eino** (CloudWeGo/ByteDance).
* **Contexte :** Nécessité de supporter OpenAI, Anthropic, Gemini, Ollama, DeepSeek et vLLM (compatible OpenAI) de manière interchangeable, avec streaming first-class et tool calling.
* **Justification vs alternatives :**
  * Eino (~9.7k stars) : type safety forte via generics, streaming natif dans tout le pipeline, patterns agent intégrés (ReAct, Deep Agent, multi-agent, plan-execute), support MCP natif.
  * LangChainGo (~8.7k stars) : plus de contributeurs (187 vs 28) mais typage plus faible (`map[string]any`), orchestration streaming moins sophistiquée.
* **Risques identifiés :**
  * Dépendance forte sur l'équipe ByteDance (28 contributeurs).
  * Documentation parfois traduite du chinois.
  * Si ByteDance désinvestit l'open-source, migration vers LangChainGo ou abstraction maison.
* **Conséquences :** Switch de provider via configuration. Support natif du streaming et du tool calling pour le bridge vers les plugins Wasm.

### ADR 003 : Système de Plugins et Sandbox

* **Décision :** **Extism** (Go SDK, basé sur wazero) pour les plugins custom + **MCP** (via `eino-ext/tool/mcp`) pour l'écosystème existant.
* **Contexte :** Les outils (Tools) et connecteurs (Messaging) doivent être isolés pour éviter qu'un bug ou une injection ne compromette l'hôte. Le Component Model WIT n'est pas disponible en Go (wazero refuse de l'implémenter avant standardisation W3C). L'écosystème MCP est trop vaste pour être ignoré.
* **Justification vs alternatives :**
  * **Extism** (~5.4k stars) : construit sur wazero (pur Go, pas de CGo), gère automatiquement le passage de strings/mémoire/sérialisation, PDKs en 7+ langages, host functions injectables, battle-tested.
  * **wazero brut** : contrôle total mais demande d'implémenter manuellement la gestion mémoire, la sérialisation, et les conventions host/guest.
  * **knqyf263/go-plugin** (~720 stars) : interfaces via Protobuf, plus petit écosystème.
  * **wasmtime-go** : CGo obligatoire, Component Model incomplet, casse la cross-compilation Go.
* **Conséquences :**
  * Plugins custom (Telegram, tools maison) → Extism/Wasm (isolation totale).
  * Serveurs MCP existants (GitHub, filesystem, etc.) → MCP Client Go natif via Eino (stdio/SSE transport).
  * Les deux types exposent la même interface `tool.InvokableTool` d'Eino.
  * PDKs disponibles pour les auteurs de plugins : Go, Rust, C, Zig, JS, AssemblyScript, Haskell.

### ADR 004 : Persistance et Indexation

* **Décision :** **SQLite** (via `modernc.org/sqlite`, pur Go) avec **FTS5** pour l'indexation full-text.
* **Contexte :** L'utilisateur doit pouvoir auditer les "pensées" de l'IA via un éditeur de texte simple, mais l'IA doit pouvoir faire des recherches rapides (RAG/Mémoire).
* **Justification vs alternatives :**
  * **SQLite + FTS5** (via `modernc.org/sqlite`) : pur Go, zéro dépendance, index et données dans la même base, BM25 intégré. Idéal pour le MVP.
  * **Bleve** (~11k stars) : moteur de recherche Go pur, supporte la recherche vectorielle (k-NN). Option d'évolution si besoin de RAG sémantique.
  * **Tantivy** : performances supérieures mais nécessiterait du CGo/FFI, cassant le "pur Go".
* **Conséquences :**
  * Les logs et la mémoire long-terme sont écrits en `.md` (audit humain, versionnable Git).
  * SQLite FTS5 indexe le contenu Markdown pour les recherches rapides.
  * Migration possible vers Bleve en Phase 2 si besoin de recherche vectorielle/sémantique.

### ADR 005 : Architecture Gateway (Daemon)

* **Décision :** L'agent tourne comme un process **Gateway** persistant. Les clients (TUI, CLI, Web) se connectent via **WebSocket** (chi).
* **Contexte :** L'agent doit tourner en permanence (polling plugins comm, crons, heartbeats). Les interfaces utilisateur sont des clients éphémères qui se connectent/déconnectent. Inspiré du Gateway OpenClaw mais en Go avec isolation Wasm.
* **Conséquences :**
  * Le Gateway est le **single source of truth** : event store, sessions, config, état des plugins.
  * Protocole WS à 3 frames : `req` (requête), `res` (réponse), `event` (push bidirectionnel).
  * Bind `127.0.0.1` (loopback) par défaut pour la sécurité.
  * Plusieurs clients simultanés supportés (TUI + CLI + Web).
  * L'Event Bus existant (inspiré de `ctm-ai-agent/pkg/events/`) est bridgé vers le WS Hub.

### ADR 005b : Interfaces Utilisateurs

* **Décision :** **CLI client** (one-shot) et **Bubbletea TUI** (interactive) en Phase 1. **React SPA** en Phase 2+.
* **Contexte :** Les clients ne font pas partie du Gateway — ce sont des process séparés qui se connectent via WS.
* **Conséquences :**
  * Phase 1 : CLI (`ozzie ask`) + TUI via Bubbletea + Bubbles + Lipgloss, connectés au Gateway via WS.
  * Phase 2+ : React SPA servie par chi (même endpoint WS).
  * Le portail Web est volontairement reporté pour se concentrer sur le core.

### ADR 006 : Architecture Event-Driven

* **Décision :** Event Bus in-memory (Go channels) avec Event Store SQLite. Basé sur l'implémentation éprouvée de `ctm-ai-agent/pkg/events/`.
* **Contexte :** Chaque action (message reçu, décision LLM, tool call, résultat) doit être traçable, auditable et rejouable. Les fichiers Markdown et l'index FTS5 sont des projections (read-models), pas la source de vérité.
* **Base de code existante réutilisée :**
  * `bus.go` : Bus + RingBuffer + dispatch goroutine — réutilisé tel quel.
  * `types.go` : Event struct, ID generation, ResumeToken — réutilisé, event types domain-specific à adapter.
  * `payloads.go` : Typed payloads avec generics (`ExtractPayload[T]`) — réutilisé.
  * `prompt.go` : PromptConfig, builder pattern — réutilisé.
  * `callbacks/events.go` : Eino callbacks → events adapter — réutilisé.
  * `agent/eventrunner.go` : Orchestration event-driven + interrupt/resume — adapté pour multi-skill et model routing.
  * `agent/middlewares.go` : Confirmation middleware — réutilisé.
* **Conséquences :**
  * La source de vérité = le stream d'events immuables dans SQLite.
  * Les projections (Markdown, FTS5, TUI state) sont des consumers de l'Event Bus.
  * Le WS Hub est un consumer/producer de l'Event Bus (bridge vers les clients distants).
  * Replay possible : reconstruction de l'état complet en rejouant les events.
  * Découplage : TUI, CLI, portail Web, logs Markdown sont des consumers indépendants.

### ADR 007 : Model Registry, Tag/Affinity et Sélecteurs

* **Décision :** Un registre de modèles LLM/SLM centralisé avec **tags libres** (affinity), sélection par nom ou par **sélecteur à la Kubernetes**, et support de **multiples types d'authentification**.
* **Contexte :** Certains sub-agents traitent des données sensibles (compliance, données internes) et doivent utiliser des modèles on-premise (vLLM). D'autres tâches simples (traduction, digest) peuvent utiliser des SLM rapides et peu coûteux. Plutôt qu'un modèle binaire cloud/on-premise codé en dur, un système de tags libres permet à l'utilisateur de modéliser ses propres niveaux de sécurité, coût, vitesse, juridiction, etc.
* **Conséquences :**
  * Les providers déclarent des **tags arbitraires** (security tier, cost, speed, capability, jurisdiction...).
  * Chaque skill peut sélectionner son modèle soit **par nom** (explicite), soit via un **`model_selector`** avec contraintes required/preferred.
  * Le core est **agnostique** de la sémantique des tags — il se contente de matcher.
  * L'enforcement de sécurité est entièrement piloté par la configuration utilisateur, pas par du code.

---

## 3. Architecture Core

### 3.1 Gateway — Control Plane

Le Gateway est le process central et persistant du système. Inspiré du Gateway OpenClaw, adapté avec isolation Wasm et Go.

```
┌──────────────────────────────────────────────────────────────────┐
│                     GATEWAY (ozzie gateway)                    │
│                     127.0.0.1:18420                               │
│                                                                   │
│  ┌──────────┐  ┌──────────┐  ┌───────────┐  ┌────────────────┐  │
│  │ Deep     │  │ Event    │  │ Scheduler │  │ Extism Runtime │  │
│  │ Agent    │  │ Bus      │  │ Heartbeat │  │ + MCP Client   │  │
│  │ (Eino)   │  │ + Store  │  │ Cron      │  │                │  │
│  └────┬─────┘  └────┬─────┘  └─────┬─────┘  └──────┬─────────┘  │
│       └──────────────┼──────────────┘               │            │
│                      │                               │            │
│  ┌───────────────────┴───────────────────────────────┘            │
│  │                                                                │
│  │  ┌──────────────────────────────────────────────────────────┐  │
│  │  │                     chi Router                            │  │
│  │  │                                                           │  │
│  │  │  GET  /api/ws          → WS Hub (event streaming)        │  │
│  │  │  GET  /api/health      → Gateway health + status         │  │
│  │  │  POST /api/message     → REST one-shot (CLI ask)         │  │
│  │  │  GET  /api/sessions    → Liste des sessions              │  │
│  │  │  GET  /api/events      → Historique (ring buffer)        │  │
│  │  └──────────────────────────────────────────────────────────┘  │
│  │                                                                │
│  │  ┌──────────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │  │ Session Manager  │  │ Secret Vault │  │ SQLite          │  │
│  │  │                  │  │ (chiffré)    │  │ Event Store     │  │
│  │  └──────────────────┘  └──────────────┘  └─────────────────┘  │
│  └────────────────────────────────────────────────────────────────┘
└────────────────────────────┬─────────────────────────────────────┘
                             │ WebSocket :18420/api/ws
                ┌────────────┼────────────┐
                │            │            │
           ┌────┴────┐  ┌───┴─────┐  ┌───┴─────┐
           │   TUI   │  │   CLI   │  │  React  │
           │ client  │  │  client │  │   SPA   │
           │(bubbletea)│ │(one-shot)│ │(Phase 2)│
           └─────────┘  └─────────┘  └─────────┘
```

**Responsabilités du Gateway :**
* Single source of truth : event store, sessions, config, état des plugins.
* Maintient les connexions aux plugins communication (Telegram, Discord...).
* Exécute les crons et heartbeats même sans client connecté.
* Expose l'API WS + REST pour les clients.

**Commandes CLI :**

```bash
# Gateway
ozzie gateway                    # foreground
ozzie gateway --detach           # background (daemon)
ozzie gateway --port 18420       # port custom

# Clients
ozzie tui                        # TUI interactive (connecte via WS)
ozzie ask "Résume mes messages"  # One-shot (REST ou WS éphémère)
ozzie status                     # Health check du gateway

# Admin
ozzie sessions list              # Sessions actives
ozzie events tail                # Tail des events en live
```

### 3.2 Protocole WebSocket

Protocole inspiré d'OpenClaw, 3 types de frames :

```jsonc
// Client → Gateway : requête (request-response)
{ "type": "req", "id": "uuid", "method": "message.send", "params": { "content": "hello" } }

// Gateway → Client : réponse
{ "type": "res", "id": "uuid", "ok": true, "payload": { ... } }

// Bidirectionnel : event (push)
// Gateway → Client : stream LLM, tool calls, prompts...
{ "type": "event", "event": "assistant.stream", "payload": { "phase": "delta", "content": "Bonjour" } }

// Client → Gateway : prompt response, user message...
{ "type": "event", "event": "user.message", "payload": { "content": "hello" } }
```

Le WS Hub est un composant du Gateway qui :
* Subscribe à l'Event Bus interne → forward les events en JSON aux clients WS connectés.
* Reçoit les messages/events des clients WS → publie sur l'Event Bus interne.
* Gère la reconnexion (le client TUI retry si le gateway redémarre).
* Supporte plusieurs clients simultanés.

### 3.3 Event-Driven Backbone

```
[Plugin Comm] → IncomingMessage Event
                      ↓
                 [Event Bus]  ←──── [Scheduler: heartbeat, cron, event-triggered]
                      ↓               ↓
                [Agent Core]      [WS Hub] ←→ Clients (TUI, CLI, Web)
                      ↓
                LLM Decision → ToolCallRequested Event
                      ↓                              ↓
                [Event Store]              [Plugin Router] → Extism / MCP
                      ↓                              ↓
                [Projections]              ToolCallResult Event
                - Markdown logs                      ↓
                - SQLite FTS5              [Agent Core] → synthèse → OutgoingMessage Event
                                                                          ↓
                                                                   [Plugin Comm]
```

**Types d'events :**

| Event | Description |
|---|---|
| `incoming.message` | Message reçu d'un plugin communication |
| `outgoing.message` | Message envoyé via un plugin communication |
| `agent.thinking` | L'agent prépare sa réponse (stream LLM) |
| `agent.delegation` | L'agent délègue à un sub-agent/skill |
| `tool.call.requested` | Appel de tool demandé par le LLM |
| `tool.call.result` | Résultat d'un appel de tool |
| `tool.call.confirmation` | Confirmation requise pour un dangerous tool |
| `schedule.trigger` | Un cron/timer a déclenché une action |
| `session.created` | Nouvelle session créée |
| `session.closed` | Session terminée |

### 3.2 System Prompt — Composition par layers

Le system prompt est assemblé dynamiquement à chaque turn :

```
┌─────────────────────────────────┐
│  Base Persona                   │  ← Identité, ton, règles globales
├─────────────────────────────────┤
│  Memory Context                 │  ← Injecté depuis SQLite/FTS5 (RAG)
├─────────────────────────────────┤
│  Active Plugins Manifest        │  ← "Tu as accès à Telegram, DuckDuckGo..."
├─────────────────────────────────┤
│  Delegation Table               │  ← Générée depuis les skills actifs
├─────────────────────────────────┤
│  Conversation Context           │  ← Messages récents, état courant
└─────────────────────────────────┘
```

* Chaque plugin communication injecte un fragment décrivant ses capacités.
* Chaque tool injecte son `tool_spec` (nom, description, paramètres).
* La Delegation Table est générée automatiquement depuis les fichiers `skills/*.jsonc`.
* L'injection de skills est **sélective** : seuls les skills pertinents au contexte courant sont injectés (pas de prompt ballooning).

### 3.3 Deep Agent Pattern (Eino)

Architecture basée sur le pattern Deep Agent d'Eino :

```
┌──────────────────────────────────────────────────────────┐
│            Main Agent (Deep Agent - Eino)                │
│  - Rôle : Routeur & généraliste                          │
│  - Modèle : configurable (default du registre)           │
│  - Tools directs : monitoring, read-only                 │
│  - Delegation Table : routes vers sub-agents             │
│                                                           │
│  SubAgents (= Skills, chargés depuis JSONC) :            │
│  ├── researcher        [model: "sonnet" (par nom)]       │
│  ├── compliance-check  [selector: security>=4]           │
│  ├── quick-translate   [model: "haiku" (par nom)]        │
│  ├── daily-digest      [selector: security>=2, cost=low] │
│  └── ...                                                  │
└──────────────────────────────────────────────────────────┘
```

* Single Agent + Tool Routing en Phase 1 (suffisant).
* Le multi-agent (supervisor + sub-agents spécialisés) est natif dans Eino et activable sans refactoring.
* Les sub-agents sont créés dynamiquement au démarrage depuis les fichiers `skills/*.jsonc`.

### 3.4 Session Isolation

* **1 session = 1 contexte isolé** par sender (DM) ou par group chat.
* Chaque session a son propre historique de conversation, état, et mémoire.
* Pas de fuite de contexte entre sessions.
* Sessions persistées en SQLite (event store) + fichiers Markdown (projection audit).

---

## 4. Scheduling Interne

```
┌──────────────────────────────────────────────┐
│                Scheduler                      │
├──────────┬──────────────┬────────────────────┤
│ Heartbeat│  Cron        │  Event-triggered   │
│          │              │                    │
│ - poll   │ - tâches     │ - réaction à un    │
│   plugins│   planifiées │   event spécifique │
│ - health │ - rappels    │ - chaînes causales │
│   checks │ - digests    │                    │
└──────────┴──────────────┴────────────────────┘
```

* **Heartbeat** : tick régulier, configurable par plugin. Telegram poll toutes les 2s, RSS toutes les 5min. L'agent core vérifie la santé des plugins.
* **Cron** : tâches planifiées — "envoie un digest chaque matin à 9h", "nettoie le cache mémoire chaque nuit". Défini dans les skills via le champ `triggers.cron`.
* **Event-triggered** : réactions en chaîne — un `incoming.message` déclenche l'agent, dont le `tool.call.result` peut déclencher un autre tool.

---

## 5. Les 3 Niveaux de Plugins

### Niveau 1 — Communication (les "sens")

L'agent entend et parle à travers ces plugins.

* **Rôle** : pont bidirectionnel entre l'agent et le monde extérieur.
* **Exemples** : Telegram, Discord, Slack, Matrix, Email, Webhook HTTP.
* **Provider** : Extism/Wasm uniquement (isolation totale requise pour les I/O externes).
* **Contrat** : `poll_events() → []Message` + `send_message(channel, content)`.
* **Particularités** :
  * Heartbeat configurable par plugin.
  * Plusieurs plugins communication actifs simultanément (écoute Telegram ET Discord).
  * Chaque plugin injecte un fragment dans le system prompt.

### Niveau 2 — Tools (les "mains")

L'agent agit sur le monde à travers ces plugins.

* **Rôle** : capacités atomiques que le LLM peut invoquer.
* **Exemples** : DuckDuckGo search, fetch URL, lire un fichier, exécuter du code, appeler une API.
* **Provider** : **Extism/Wasm** (tools custom, isolation max) ou **MCP** (écosystème existant).
* **Contrat** : `call(input: JSON) → JSON` — une fonction pure, un input, un output.
* **Particularités** :
  * Chaque tool s'auto-décrit via un `tool_spec` (nom, description, schéma JSON des params).
  * Le `tool_spec` est injecté dans le system prompt pour que le LLM sache quand/comment l'utiliser.
  * Les dangerous tools déclenchent un middleware de confirmation (`compose.Interrupt`).

### Niveau 3 — Skills (le "savoir-faire")

L'agent combine tools + prompts pour accomplir des tâches complexes.

* **Rôle** : workflows pré-composés, recettes réutilisables, définis en JSONC.
* **Implémentation** : chaque skill = un **SubAgent Eino** (`ChatModelAgent`), chargé dynamiquement.
* **Contrat JSONC** :

```jsonc
{
  "name": "researcher",
  "description": "Deep research - multi-step web research and synthesis",

  // Sélection du modèle — SOIT par nom :
  "model": "sonnet",
  // SOIT par sélecteur (mutuellement exclusif avec "model") :
  // "model_selector": {
  //   "required": [{ "key": "security", "op": "Gte", "value": 2 }],
  //   "preferred": [{ "key": "speed", "op": "Eq", "value": "fast" }]
  // },
  // Si absent : hérite du modèle "default" du registre.

  // Prompt spécialisé
  "instruction": "You are a research specialist...",

  // Tools requis (doivent être installés — Extism ou MCP)
  "tools": ["web_search", "fetch_url"],

  // Triggers : quand ce skill est activé
  "triggers": {
    "delegation": true,       // le main agent peut déléguer
    "keywords": ["research", "find out", "investigate"],
    "cron": null              // pas de scheduling
  },

  // Confirmation pour actions sensibles
  "dangerous_tools": [],

  // Limites
  "max_iterations": 15
}
```

* **Exemples de skills** :

| Skill | Modèle | Pourquoi |
|---|---|---|
| `researcher` | `"model": "sonnet"` | Besoin de synthèse complexe |
| `daily-digest` | `"model_selector": { required: [security >= 2], preferred: [cost == "low"] }` | Résumé simple, optimise coût |
| `compliance-checker` | `"model_selector": { required: [security >= 4, capability ∈ compliance] }` | Données confidentielles, modèle spécialisé |
| `quick-translate` | `"model": "haiku"` | Tâche simple, latence minimale |

### Vue d'ensemble

```
╔═══════════════════════════════════════════════════════════════════════╗
║                    GATEWAY (ozzie gateway)                         ║
║                    127.0.0.1:18420                                    ║
║                                                                       ║
║  ┌──────────────┐  ┌────────────┐  ┌───────────────────────────────┐  ║
║  │ Deep Agent   │  │ Event Bus  │  │ Scheduler                     │  ║
║  │ (Eino)       │  │ + Store    │  │ - Heartbeat                   │  ║
║  │              │  │            │  │ - Cron (skill triggers)        │  ║
║  │ Main Agent   │  │ Subscribe  │  │ - Event-triggered             │  ║
║  │ + SubAgents  │  │ Publish    │  │                               │  ║
║  └──────┬───────┘  └─────┬──────┘  └──────────┬────────────────────┘  ║
║         │                │                     │                       ║
║  ┌──────┴────────────────┴─────────────────────┴────────────────────┐  ║
║  │                     PLUGIN ROUTER                                 │  ║
║  ├──────────────────┬──────────────────┬────────────────────────────┤  ║
║  │  Lvl 1: COMMS    │  Lvl 2: TOOLS    │  Lvl 3: SKILLS            │  ║
║  │  (Extism/Wasm)   │(Extism + MCP)    │  (JSONC → SubAgents)      │  ║
║  │                  │                  │                            │  ║
║  │  telegram.wasm   │  search.wasm     │  researcher.jsonc          │  ║
║  │  discord.wasm    │  github (MCP)    │  daily-digest.jsonc        │  ║
║  │  slack.wasm      │  filesystem (MCP)│  compliance.jsonc          │  ║
║  └──────────────────┴──────────────────┴────────────────────────────┘  ║
║                                                                        ║
║  ┌──────────┐  ┌──────────┐  ┌──────────────┐  ┌───────────────────┐  ║
║  │ chi      │  │ SQLite   │  │ Secret Vault │  │ Session Manager   │  ║
║  │ + WS Hub │  │ +FTS5+MD │  │ (chiffré)    │  │ (1 ctx/sender)    │  ║
║  └────┬─────┘  └──────────┘  └──────────────┘  └───────────────────┘  ║
╚═══════╪══════════════════════════════════════════════════════════════╝
        │ WebSocket
        ├──────────────┐
        │              │
   ┌────┴────┐    ┌────┴────┐    ┌──────────┐
   │   TUI   │    │   CLI   │    │  React   │
   │ client  │    │  client │    │   SPA    │
   │(bubbletea)│  │(one-shot)│   │(Phase 2) │
   └─────────┘    └─────────┘    └──────────┘
```

---

## 6. Modèle de Sécurité

### 6.1 Principe : Deny-by-default, Capability-based

```
┌──────────────────────────────────────────────────────┐
│                    TRUST BOUNDARY                     │
│                                                       │
│  ┌─────────────────────────────────────────────────┐  │
│  │              AGENT CORE (Go natif)              │  │
│  │  - Event Bus        - Scheduler                 │  │
│  │  - Prompt Composer   - Secret Vault (chiffré)   │  │
│  │  - Session Manager   - Plugin Registry          │  │
│  └──────────────┬──────────────────────────────────┘  │
│                 │                                      │
│        Extism Host Functions (API contrôlée)          │
│        ┌────────┼────────────────────────┐            │
│        │ kv_get │ kv_set │ log │ http* │ ...         │
│        └────────┼────────────────────────┘            │
│                 │                                      │
│  ═══════════════╪══════ WASM SANDBOX ═════════════    │
│                 │                                      │
│  ┌──────────────┴──────────────────────────────────┐  │
│  │              PLUGIN (Wasm Guest)                 │  │
│  │  - Pas d'accès filesystem hôte                  │  │
│  │  - Pas d'accès réseau direct                    │  │
│  │  - Pas d'accès aux secrets bruts                │  │
│  │  - Pas d'accès aux autres plugins               │  │
│  │  - Mémoire linéaire isolée                      │  │
│  └─────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────┘
```

### 6.2 Capabilities par niveau

Chaque plugin déclare ses capabilities dans son manifest JSONC :

```jsonc
// Exemple : manifest plugin Telegram (Level 1 - Communication)
{
  "name": "telegram-connector",
  "level": "communication",
  "capabilities": {
    "http": {
      "allowed_hosts": ["api.telegram.org"],
      "methods": ["GET", "POST"]
    },
    "kv": true,
    "log": true,
    "filesystem": false,
    "secrets": ["TELEGRAM_BOT_TOKEN"]
  }
}
```

```jsonc
// Exemple : manifest tool DuckDuckGo (Level 2 - Tool)
{
  "name": "duckduckgo-search",
  "level": "tool",
  "provider": "extism",
  "capabilities": {
    "http": {
      "allowed_hosts": ["duckduckgo.com", "api.duckduckgo.com"],
      "methods": ["GET"]
    },
    "kv": false,
    "log": true,
    "filesystem": false,
    "secrets": []
  },
  "tool_spec": {
    "name": "web_search",
    "description": "Search the web using DuckDuckGo",
    "parameters": {
      "query": { "type": "string", "required": true },
      "max_results": { "type": "integer", "default": 5 }
    }
  }
}
```

### 6.3 Gestion des secrets

Les secrets ne sont **jamais** exposés directement aux plugins :

1. Le plugin déclare les secrets dont il a besoin dans son manifest (`"secrets": ["TELEGRAM_BOT_TOKEN"]`).
2. Le host vérifie que le manifest autorise ce secret.
3. Le secret est injecté via une host function dédiée, jamais en clair dans la config.
4. Si un plugin demande un secret non déclaré → refus + log d'incident.

### 6.4 Model Registry — Tags, Sélecteurs et Auth

#### Tags / Affinity

Chaque provider LLM déclare des **tags libres**. L'utilisateur modélise ses propres dimensions (sécurité, coût, vitesse, juridiction, etc.) :

```jsonc
{
  "models": {
    "default": "sonnet",

    "providers": {
      "sonnet": {
        "driver": "anthropic",
        "model": "claude-sonnet-4-20250514",
        "auth": { "type": "api_key", "env": "ANTHROPIC_API_KEY" },
        "max_tokens": 4096,
        "tags": {
          "security": 2,
          "cost": "medium",
          "speed": "fast",
          "capability": "general"
        }
      },
      "haiku": {
        "driver": "anthropic",
        "model": "claude-haiku-4-5-20251001",
        "auth": { "type": "api_key", "env": "ANTHROPIC_API_KEY" },
        "max_tokens": 2048,
        "tags": {
          "security": 2,
          "cost": "low",
          "speed": "very-fast",
          "capability": "general"
        }
      },
      "gemini-pro": {
        "driver": "gemini",
        "model": "gemini-2.5-pro",
        "auth": { "type": "api_key", "env": "GOOGLE_API_KEY" },
        "tags": {
          "security": 1,
          "cost": "low",
          "speed": "fast",
          "capability": "general"
        }
      },
      "gpt4": {
        "driver": "openai",
        "model": "gpt-4o",
        "auth": { "type": "api_key", "env": "OPENAI_API_KEY" },
        "tags": {
          "security": 0,
          "cost": "high",
          "speed": "fast",
          "capability": "general"
        }
      },
      "infomaniak-llm": {
        "driver": "openai",
        "model": "infomaniak-large",
        "base_url": "https://api.infomaniak.com/v1",
        "auth": { "type": "api_key", "env": "INFOMANIAK_API_KEY" },
        "tags": {
          "security": 3,
          "cost": "medium",
          "speed": "medium",
          "capability": "general",
          "jurisdiction": "swiss"
        }
      },
      "local-llama": {
        "driver": "ollama",
        "model": "llama3.1:70b",
        "base_url": "http://localhost:11434",
        "auth": { "type": "none" },
        "tags": {
          "security": 4,
          "cost": "free",
          "speed": "slow",
          "capability": "general"
        }
      },
      "compliance-ft": {
        "driver": "openai",
        "model": "compliance-finetuned-v3",
        "base_url": "https://vllm.internal.corp:8000/v1",
        "auth": { "type": "api_key", "env": "VLLM_API_KEY" },
        "tags": {
          "security": 4,
          "cost": "free",
          "speed": "medium",
          "capability": "compliance"
        }
      }
    }
  }
}
```

Les tags sont **entièrement libres** — le core ne connaît pas leur sémantique, il se contente de matcher. L'utilisateur est responsable de sa propre modélisation des niveaux de sécurité, coût, etc.

#### Sélection du modèle par skill

Deux modes mutuellement exclusifs :

**Mode 1 — Par nom (explicite) :**

```jsonc
{
  "name": "quick-translate",
  "model": "haiku"
}
```

**Mode 2 — Par sélecteur (à la Kubernetes) :**

```jsonc
{
  "name": "compliance-checker",
  "model_selector": {
    // MUST match — le skill refuse de démarrer si aucun provider ne match
    "required": [
      { "key": "security", "op": "Gte", "value": 3 },
      { "key": "capability", "op": "In", "values": ["compliance", "general"] }
    ],
    // SHOULD match — critères de tri parmi les candidats éligibles
    "preferred": [
      { "key": "capability", "op": "Eq", "value": "compliance" },
      { "key": "cost", "op": "Eq", "value": "free" }
    ]
  }
}
```

**Opérateurs supportés :**

| Opérateur | Description | Applicable à |
|---|---|---|
| `Eq` | Égalité exacte | string, number |
| `NotEq` | Différent de | string, number |
| `In` | Valeur parmi une liste | string |
| `NotIn` | Valeur absente de la liste | string |
| `Gte` | Supérieur ou égal | number |
| `Lte` | Inférieur ou égal | number |
| `Exists` | Le tag existe | — |

**Algorithme de résolution :**

1. Filtrer les providers qui matchent **tous** les `required`.
2. Si 0 candidats → **erreur au démarrage**, le skill ne se charge pas.
3. Parmi les candidats, scorer par nombre de `preferred` matchés.
4. En cas d'égalité → prendre le premier déclaré (ordre du config).

**Exemples de résolution :**

```
Skill "daily-digest" :
  required: security >= 2
  preferred: cost == "low", speed == "very-fast"
  → Candidats: sonnet(2), haiku(2), infomaniak(3), local-llama(4), compliance-ft(4)
  → Scores preferred: haiku = 2/2, sonnet = 0/2, infomaniak = 0/2, ...
  → Résolu: haiku ✓

Skill "compliance-checker" :
  required: security >= 4, capability ∈ [compliance, general]
  preferred: capability == "compliance"
  → Candidats: local-llama(4,general), compliance-ft(4,compliance)
  → Scores preferred: compliance-ft = 1/1, local-llama = 0/1
  → Résolu: compliance-ft ✓
```

Si aucun `model` ni `model_selector` n'est spécifié → le skill hérite du modèle `default` du registre.

#### Types d'authentification

Inspiré d'OpenClaw, avec support de multiples types d'auth :

```jsonc
// API Key classique (via variable d'environnement)
"auth": {
  "type": "api_key",
  "env": "ANTHROPIC_API_KEY"
}

// API Key avec header custom
"auth": {
  "type": "api_key",
  "env": "CUSTOM_API_KEY",
  "header": "X-Custom-Auth"      // défaut: "Authorization: Bearer ..."
}

// Token Claude Code (session token)
"auth": {
  "type": "claude_code_token"
}

// OAuth2 Client Credentials
"auth": {
  "type": "oauth2",
  "client_id_env": "OAUTH_CLIENT_ID",
  "client_secret_env": "OAUTH_CLIENT_SECRET",
  "token_url": "https://auth.corp.internal/oauth/token",
  "scopes": ["llm:invoke"]
}

// Aucune auth (Ollama local, etc.)
"auth": {
  "type": "none"
}
```

Tous les secrets restent dans les variables d'environnement ou le secret vault — jamais en clair dans la config.

### 6.5 Différenciation vs OpenClaw

| Faille OpenClaw | Réponse Ozzie |
|---|---|
| Pas de sandbox par défaut (workspace accède à des chemins absolus) | Wasm isolation (Extism) — le plugin ne voit rien de l'hôte |
| Dual supply chain risk (skills + instructions externes dans le même runtime) | 3 niveaux de plugins séparés avec capabilities distinctes |
| Credentials persistants accessibles par les agents en runtime | Pas d'accès direct aux secrets — le host fait le proxy |
| 26% des skills contiennent au moins une vulnérabilité (Cisco) | Skills = JSONC déclaratif (prompt + config), pas du code arbitraire |
| Runtime TypeScript = injection de code native | Wasm = pas d'accès mémoire hôte, pas d'import système |

---

## 7. Spécifications des Interfaces (Contract-First)

### Messaging — Plugin Communication (Level 1)

L'interface doit supporter :

* `poll_events() → []Message`
* `send_message(channel_id, content) → result`
* `react(message_id, emoji) → result`
* `get_history(channel_id, limit) → []Message`

### Host API (Runtime-side)

Le noyau expose aux plugins via Extism Host Functions :

* `kv_set(key, val)` / `kv_get(key)` : Persistance légère (SQLite).
* `http_request(url, method, body)` : Si autorisé par les capabilities (domaines whitelistés).
* `log(level, msg)` : Pipeline de logs centralisé (affiché dans la TUI).
* `get_secret(name)` : Injection de secret si autorisé par le manifest.

### Tool Spec (Level 2)

Chaque tool (Extism ou MCP) expose un `tool_spec` unifié :

```jsonc
{
  "name": "web_search",
  "description": "Search the web using DuckDuckGo",
  "parameters": {
    "query": { "type": "string", "required": true },
    "max_results": { "type": "integer", "default": 5 }
  }
}
```

Ce spec est injecté dans le system prompt et utilisé par Eino pour le tool calling.

### Tool MCP (Level 2 - MCP Provider)

```jsonc
{
  "name": "github",
  "level": "tool",
  "provider": "mcp",
  "mcp": {
    "transport": "stdio",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-github"],
    "env": { "GITHUB_TOKEN": "${{ .Env.GITHUB_TOKEN }}" }
  }
}
```

Les tools MCP sont chargés via `eino-ext/components/tool/mcp` et exposent la même interface `tool.InvokableTool` que les tools Extism.

### Convention de sérialisation

Les échanges host ↔ plugin Extism passent par des bytes. Format retenu : **JSON** pour le MVP (simplicité, debugging facile), avec migration possible vers **Protobuf** ou **MessagePack** si les performances l'exigent.

### Configuration

Les fichiers de configuration (agent, plugins, manifests, skills) utilisent du **JSON avec commentaires** via `marcozac/go-jsonc`. Ce choix permet :
* Des fichiers de config commentés et documentés directement dans le fichier.
* Zéro dépendance externe — `go-jsonc` strip les commentaires puis délègue à `encoding/json` standard.
* Compatibilité totale avec l'outillage JSON existant (validation, schémas).

---

## 8. Analyse Pro/Cons du Stack

### Go (vs Rust initial)

| | Pro | Con |
|---|---|---|
| **Vélocité** | Itération 3-5x plus rapide, compilation instantanée | Moins performant que Rust sur du compute pur |
| **Écosystème** | Mature pour le backend, goroutines natives | Écosystème AI/Agent plus jeune qu'en Python |
| **Déploiement** | Binaire unique, cross-compilation triviale | GC peut introduire des pauses (négligeable ici) |
| **Contributions** | Courbe d'apprentissage faible, plus de contributeurs potentiels | — |

### Extism + MCP (vs Wasmtime+WIT)

| | Pro | Con |
|---|---|---|
| **Intégration** | Pur Go, pas de CGo, API haut niveau | Pas de WIT / Component Model |
| **Multi-langage** | PDKs en 7+ langages | Convention Extism propre (pas un standard W3C) |
| **Sandbox** | Deny-by-default, capabilities granulaires | Overhead sérialisation bytes à chaque appel |
| **Écosystème MCP** | Accès à des centaines de tools MCP existants | MCP moins isolé que Wasm (process natif) |

### Eino (vs Rig)

| | Pro | Con |
|---|---|---|
| **Features** | Streaming first-class, Deep Agent, MCP natif | Dépendance ByteDance (28 contributeurs) |
| **Type safety** | Generics Go, pas de `map[string]any` | Documentation parfois traduite du chinois |
| **Providers** | OpenAI, Anthropic, Gemini, Ollama, DeepSeek, vLLM | Certains providers orientés cloud chinois |
| **Agent patterns** | Deep Agent, ReAct, multi-agent, middleware confirmation | — |

---

## 9. Risques et Mitigations

| Risque | Impact | Mitigation |
|---|---|---|
| ByteDance abandonne Eino open-source | Élevé | Couche d'abstraction interne ; migration vers LangChainGo |
| Extism ne supporte jamais WIT | Moyen | Suffisant pour le use-case actuel ; surveiller Arcjet Gravity |
| SQLite FTS5 insuffisant pour du RAG sémantique | Moyen | Migration vers Bleve (recherche vectorielle) en Phase 2 |
| Projet solo, surface technique large | Élevé | Focus Phase 1 strict : core + TUI uniquement, pas de portail Web |
| Faille dans un serveur MCP tiers | Moyen | Transport stdio isolé + capabilities restreintes + audit events |
| Data leak via mauvais model routing | Élevé | Enforcement par `model_selector.required` au démarrage du skill ; tags security sur les providers |

---

## 10. Roadmap de Développement

### Priorisation — Chemin critique

Le premier objectif est un **flow end-to-end minimal** : `ozzie gateway` tourne → `ozzie ask "hello"` → le LLM répond.

```
                    ┌─────────────┐
                    │ 1. Workspace │
                    │    Go setup  │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              ↓            ↓            ↓
       ┌────────────┐ ┌─────────┐ ┌──────────────┐
       │ 2. Model   │ │ 3. Event│ │ 4. Gateway   │
       │  Registry  │ │  Bus    │ │  chi + WS Hub│
       │  (simple)  │ │(from CTM)│ │              │
       └─────┬──────┘ └────┬────┘ └──────┬───────┘
             │              │             │
             └──────┬───────┴─────────────┘
                    ↓
             ┌────────────┐
             │ 5. Bridge  │
             │ Eino-Tools │
             │ (minimal)  │
             └──────┬─────┘
                    ↓
             ┌────────────┐
             │ 6. CLI     │        ← PREMIER FLOW E2E
             │ ask client │
             └──────┬─────┘
                    │
       ┌────────────┼────────────┐
       ↓            ↓            ↓
  ┌─────────┐ ┌──────────┐ ┌──────────┐
  │7. Extism│ │ 8. MCP   │ │ 9. TUI   │
  │ Runtime │ │ Client   │ │ client   │
  └────┬────┘ └────┬─────┘ └──────────┘
       │           │
       └─────┬─────┘
             ↓
      ┌─────────────┐
      │10. Skill    │
      │  Loader     │
      └──────┬──────┘
             ↓
      ┌─────────────┐
      │11. Telegram │
      │  Plugin     │
      └──────┬──────┘
             ↓
      ┌─────────────┐
      │12. Scheduler│
      │  Cron       │
      └─────────────┘
```

### Phase 1a — Premier flow E2E (P0)

Objectif : `ozzie gateway` → `ozzie ask "hello"` → réponse LLM.

1. **Workspace Go** : Setup module, dépendances (`go-chi`, `eino`, `gorilla/websocket`).
2. **Model Registry** (simple) : Au moins 1 provider fonctionnel (Anthropic). Tags/sélecteurs dès le début mais un seul provider suffit.
3. **Event Bus** : Fork/adaptation depuis `ctm-ai-agent/pkg/events/`. Bus + RingBuffer + typed payloads + callbacks Eino.
4. **Gateway + chi + WS Hub** : Process `ozzie gateway` qui tourne, chi router, endpoint WS, protocole req/res/event.
5. **Bridge Eino** (minimal) : Agent Eino basique connecté à l'Event Bus via EventRunner (adapté du CTM).
6. **CLI client** (`ozzie ask`) : Envoie un message via WS ou REST, attend la réponse, quitte.

### Phase 1b — Plugins et TUI (P1)

Objectif : tools fonctionnels + interface interactive.

7. **Runtime Extism** : Chargeur de plugins Wasm, capabilities, host functions.
8. **Intégration MCP** : Client MCP via `eino-ext/tool/mcp`.
9. **TUI client** (Bubbletea) : Se connecte au gateway via WS, streaming, prompts, tool calls.

### Phase 1c — Skills et Communication (P2)

Objectif : agent autonome avec skills et messaging.

10. **Skill Loader** : Chargement `skills/*.jsonc` → SubAgents Eino avec model routing par sélecteurs.
11. **MVP Connecteur Telegram** : Premier plugin communication Wasm.
12. **Scheduling** : Heartbeat polling + cron pour les skills schedulés.

### Phase 2 — Enrichissement

* Portail Web React SPA servi par chi (même endpoint WS).
* Migration indexation vers Bleve si besoin RAG sémantique.
* Plugins communication additionnels (Discord, Slack, Matrix, etc.).
* Workflows multi-étapes (pattern résumable, inspiré du job-creation du CTM).
* Système de capabilities avancé (permissions granulaires dynamiques).
* Device pairing / authentification des clients (inspiré OpenClaw).

---

## 11. Stack Technique Résumé

| Composant | Choix | Justification |
|---|---|---|
| Langage | **Go** | Performance/vélocité, binaire unique, écosystème mature |
| Gateway / Router | **go-chi** | Léger, idiomatique, WS + REST + middlewares |
| WebSocket | **gorilla/websocket** | Standard de facto en Go, protocole req/res/event |
| LLM Framework | **Eino** | Multi-provider, type-safe, streaming, Deep Agent, MCP natif |
| Plugins/Sandbox | **Extism** (sur wazero) | Isolation Wasm, multi-langage, pur Go |
| Tools écosystème | **MCP** (via `eino-ext/tool/mcp`) | Accès à l'écosystème MCP existant |
| Event Bus | Fork de `ctm-ai-agent/pkg/events/` | Éprouvé, Go channels, typed payloads, ring buffer |
| Persistance | **SQLite** (`modernc.org/sqlite`) + FTS5 | Pur Go, zéro dépendance, event store + audit Markdown |
| TUI client | **Bubbletea** + Bubbles + Lipgloss | Connecté au Gateway via WS |
| Configuration | **go-jsonc** (`marcozac/go-jsonc`) | JSON avec commentaires, zéro dépendance, API `encoding/json` |
| Portail Web | **React SPA** (Phase 2+) | Connecté au Gateway via même endpoint WS |

---

## 12. Architecture de Code

### Structure du repo

**Mono-repo** pour la Phase 1. Un seul repo contient le gateway, les clients, les plugins built-in et les skills. Extraction possible des plugins dans des repos séparés quand il y aura des contributeurs.

```
ozzie/
├── cmd/
│   ├── ozzie/                      # Binaire unique
│   │   └── main.go                 # Root command
│   └── commands/
│       ├── gateway.go              # `ozzie gateway`
│       ├── ask.go                  # `ozzie ask "..."`
│       ├── tui.go                  # `ozzie tui`
│       ├── status.go              # `ozzie status`
│       └── sessions.go            # `ozzie sessions list`
│
├── internal/                       # Packages privés (pas d'import externe)
│   ├── gateway/                    # Orchestration du gateway
│   │   ├── server.go              # chi setup, routes, lifecycle
│   │   └── ws/
│   │       ├── hub.go             # WS hub, connexions, broadcast
│   │       └── protocol.go        # req/res/event frames
│   │
│   ├── agent/                      # Agent core
│   │   ├── agent.go               # Deep Agent setup, skill loading
│   │   ├── eventrunner.go         # Event-driven orchestration (from CTM)
│   │   ├── middlewares.go         # Confirmation, logging (from CTM)
│   │   └── prompt.go             # System prompt composer (layers)
│   │
│   ├── events/                     # Event bus (forked from CTM)
│   │   ├── bus.go                 # Bus + RingBuffer + dispatch
│   │   ├── types.go              # Event types, ID gen, ResumeToken
│   │   ├── payloads.go           # Typed payloads + generics helpers
│   │   └── prompt.go             # Prompt config, builder
│   │
│   ├── models/                     # Model registry
│   │   ├── registry.go           # Provider loading, tag matching
│   │   ├── selector.go           # K8s-style selector resolution
│   │   └── auth.go               # Auth types (api_key, oauth2, etc.)
│   │
│   ├── plugins/                    # Plugin runtime
│   │   ├── extism/
│   │   │   ├── runtime.go        # Extism loader, host functions
│   │   │   ├── capabilities.go   # Capability enforcement
│   │   │   └── bridge.go         # Extism → tool.InvokableTool adapter
│   │   ├── mcp/
│   │   │   └── client.go         # MCP client wrapper (eino-ext)
│   │   └── registry.go           # Unified tool registry (Extism + MCP)
│   │
│   ├── skills/                     # Skill loader
│   │   ├── loader.go             # JSONC → SubAgent factory
│   │   └── types.go              # Skill config structs
│   │
│   ├── sessions/                   # Session management
│   │   ├── manager.go            # Create, get, list sessions
│   │   └── store.go              # SQLite persistence
│   │
│   ├── scheduler/                  # Scheduling
│   │   ├── scheduler.go          # Heartbeat, cron, event triggers
│   │   └── cron.go               # Cron expression parsing
│   │
│   ├── storage/                    # Persistance
│   │   ├── sqlite.go             # SQLite setup, migrations
│   │   ├── eventstore.go         # Event store (write)
│   │   ├── projections.go        # Markdown + FTS5 projections
│   │   └── secrets.go            # Secret vault (chiffré)
│   │
│   ├── callbacks/                  # Eino callbacks → Event Bus
│   │   └── events.go             # (from CTM)
│   │
│   └── config/                     # Configuration
│       ├── config.go             # Root config struct
│       ├── loader.go             # JSONC loading, env template
│       └── validate.go           # Validation au démarrage
│
├── clients/                        # Clients (process séparés)
│   ├── tui/                        # Bubbletea TUI
│   │   ├── app.go
│   │   └── components/
│   │       ├── chat.go
│   │       ├── header.go
│   │       └── input.go
│   └── ws/                         # Client WS partagé (TUI + CLI)
│       └── client.go
│
├── plugins/                        # Source des plugins Wasm
│   ├── telegram/                   # Plugin Telegram (Go → Wasm)
│   └── duckduckgo/                 # Plugin search
│
├── skills/                         # Définitions JSONC des skills built-in
│   ├── researcher.jsonc
│   ├── daily-digest.jsonc
│   └── quick-translate.jsonc
│
├── configs/                        # Exemples de config
│   └── config.example.jsonc
│
├── docs/
│   ├── brainstorming.md
│   └── brainstorming-v2.md
│
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### Principes de structure

| Principe | Règle |
|---|---|
| **`internal/`** | Tout ce qui est spécifique au gateway est privé. Pas d'API publique Go (c'est un produit, pas un SDK). |
| **`clients/`** | Séparé d'`internal/`. Le TUI ne dépend QUE du protocole WS, jamais d'`internal/`. Force le découplage. |
| **`plugins/`** | Chaque plugin = module Go indépendant compilé en Wasm. Pourra devenir un repo séparé. |
| **`skills/`** | JSONC versionnés avec le projet. L'utilisateur peut ajouter les siens dans `~/.ozzie/skills/`. |
| **`configs/`** | Exemples uniquement. La config active vit dans `~/.ozzie/config.jsonc`. |

### Répertoire utilisateur

```
~/.ozzie/
├── config.jsonc                    # Configuration active
├── data/
│   ├── ozzie.db                    # SQLite (event store, sessions, KV)
│   └── logs/                       # Projections Markdown
│       └── 2026-02-20.md
├── plugins/                        # Plugins Wasm installés par l'utilisateur
├── skills/                         # Skills JSONC de l'utilisateur
└── secrets/                        # Secret vault (chiffré)
```

---

## 13. Coding Conventions

### Go — Règles générales

| Convention | Règle |
|---|---|
| **Formattage** | `gofumpt` (gofmt strict, grouping imports). |
| **Linting** | `golangci-lint` avec : `errcheck`, `govet`, `staticcheck`, `unused`, `gosimple`, `ineffassign`, `misspell`, `revive`. |
| **Errors** | Toujours wrappés avec contexte : `fmt.Errorf("load skill %q: %w", name, err)`. Jamais de `panic` sauf init. |
| **Interfaces** | Petites, définies côté consommateur (pas chez le producteur). |
| **Nommage packages** | Un mot (`events`, `models`, pas `event_bus`). Pas de stuttering (`events.Event`, pas `events.EventStruct`). |
| **Constructeurs** | `NewXxx(cfg XxxConfig)` avec struct de config plutôt que 5+ params. |
| **Concurrence** | Goroutines toujours avec `context.Context` + done channel. Pas de goroutine leak. |
| **Tests** | Table-driven. `_test.go` dans le même package. Mocks via interfaces, pas de framework mock. |
| **Logging** | `slog` (stdlib). Structured logging : `slog.With("key", val)`. |
| **Context** | Propagé partout. Premier paramètre de toute fonction I/O. |

### Conventions spécifiques Ozzie

| Convention | Règle |
|---|---|
| **Config files** | JSONC (`go-jsonc`). Templates env avec `${{ .Env.VAR }}`. |
| **Events** | Tout event a un type constant, un typed payload avec helper `Extract`. Pattern du CTM. |
| **Plugins Wasm** | Chaque plugin = module Go séparé avec son propre `go.mod`, compilé en `.wasm`. |
| **Skills** | Pur JSONC, pas de code. Si c'est trop complexe pour du JSONC → c'est un plugin. |
| **Commits** | Conventional Commits : `feat:`, `fix:`, `refactor:`, `docs:`, `chore:`. |
| **Branches** | `main` (stable), `feat/*`, `fix/*`. PR obligatoires. |

---

## 14. Milestone : CTM Integration

**Objectif** : Valider l'extensibilité d'Ozzie en portant les tools et skills de `ctm-ai-agent` comme extensions.

### Composants à porter

| Composant CTM | Cible dans Ozzie | Type |
|---|---|---|
| `pkg/tools/controlm/list_jobs.go` | `plugins/controlm/` (Wasm) ou tool natif | Level 2 — Tool |
| `pkg/tools/controlm/run_job.go` | Idem | Level 2 — Tool (dangerous) |
| `pkg/tools/controlm/hold_job.go` | Idem | Level 2 — Tool (dangerous) |
| `pkg/tools/controlm/release_job.go` | Idem | Level 2 — Tool (dangerous) |
| `pkg/tools/controlm/rerun_job.go` | Idem | Level 2 — Tool (dangerous) |
| `pkg/tools/controlm/get_job_log.go` | Idem | Level 2 — Tool |
| `pkg/tools/controlm/get_alerts.go` | Idem | Level 2 — Tool |
| `pkg/tools/controlm/acknowledge_alert.go` | Idem | Level 2 — Tool (dangerous) |
| Agent "write-operation" | `skills/ctm-write-ops.jsonc` | Level 3 — Skill |
| Agent "analysing" | `skills/ctm-analysis.jsonc` | Level 3 — Skill |
| Agent "alerting" | `skills/ctm-alerting.jsonc` | Level 3 — Skill |
| Workflow "job-creation" | `skills/ctm-job-creation.jsonc` | Level 3 — Skill (workflow) |
| **TUI** | **Non repris** — nouvelle TUI dans `clients/tui/` | — |

### Ce que ça valide

* Un domaine métier complet (Control-M) tourne comme **extension** d'Ozzie.
* Les tools dangerous fonctionnent avec le middleware de confirmation.
* Les skills JSONC couvrent des cas réels (routing, analyse, alerting, workflow multi-étapes).
* La séparation gateway / tools / skills est viable en pratique.
* Si le CTM tourne, **n'importe quel domaine** peut être pluggé de la même façon.

### Position dans la roadmap

Ce milestone se place **après Phase 1c** (skills + Telegram fonctionnels) et **avant Phase 2** (portail Web) :

```
Phase 1a (P0) → Phase 1b (P1) → Phase 1c (P2) → Milestone CTM → Phase 2
```
