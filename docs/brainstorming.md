# Cahier des Charges : Project X (Agentic OS)

## 1. Vision du Projet

Créer un agent IA autonome, performant et sécurisé, capable de connecter des modèles de langage (LLM) à des interfaces de communication (Messaging) et des outils d'exécution (Tools).

**Principes directeurs :**

* **Performance brute :** Runtime Rust pour une empreinte mémoire minimale (< 50Mo au repos).
* **Isolation Totale :** Exécution des extensions via WebAssembly (Sandboxing).
* **Auditabilité :** Persistance des états en Markdown pour une lecture humaine directe.
* **Extensibilité :** Système de plugins via interfaces WIT (Language-agnostic).

**Inspiration**
* OpenClaw

---

## 2. Architecture Technique (ADR)

### ADR 001 : Choix du Langage et Framework Web

* **Décision :** Rust avec le framework **Axum**.
* **Contexte :** Node.js est jugé trop lourd pour des déploiements sur serveurs contraints et manque de sécurité mémoire native pour des agents autonomes.
* **Conséquences :** Utilisation de `tokio` pour l'asynchronisme, `axum` pour l'API Gateway, et `tower-http` pour la couche middleware.

### ADR 002 : Abstraction des LLM

* **Décision :** Utilisation de la crate **Rig**.
* **Contexte :** Nécessité de supporter OpenAI, Anthropic, Gemini et Ollama de manière interchangeable.
* **Conséquences :** Création d'un wrapper interne pour pallier la complexité du multi-provider dynamique de Rig, permettant de switcher de modèle via la configuration sans recompiler.

### ADR 003 : Système de Plugins et Sandbox

* **Décision :** **Wasmtime** avec interfaces **WIT**.
* **Contexte :** Les outils (Tools) et connecteurs (Messaging) doivent être isolés pour éviter qu'un bug ou une injection ne compromette l'hôte.
* **Conséquences :** * Définition de contrats `.wit` pour la communication Host-Guest.
* Support de n'importe quel langage compilable en Wasm (Rust, Go, Python via Component Model).



### ADR 004 : Persistance et Indexation

* **Décision :** Hybride **Markdown + Tantivy/SQLite**.
* **Contexte :** L'utilisateur doit pouvoir auditer les "pensées" de l'IA via un éditeur de texte simple, mais l'IA doit pouvoir faire des recherches rapides (RAG/Mémoire).
* **Conséquences :** Les logs et la mémoire long-terme sont écrits en `.md`. Un indexeur `Tantivy` tourne en arrière-plan pour les recherches sémantiques.

### ADR 005 : Interfaces Utilisateurs

* **Décision :** **Leptos** (Web) et **Ratatui** (TUI).
* **Contexte :** Besoin d'un portail de configuration moderne (Leptos en SSR via Axum) et d'un monitoring temps-réel en terminal pour les administrateurs (TUI).

---

## 3. Spécifications des Interfaces (Contract-First)

### Messaging (Plugin-side)

L'interface doit supporter :

* `send_message(channel, content)`
* `on_message(event)`
* `react(message_id, emoji)`
* `get_history(limit)`

### Host API (Runtime-side)

Le noyau expose aux plugins :

* `kv_store(key, val)` : Persistance légère.
* `http_request(url)` : Si autorisé par les capabilities.
* `log(level, msg)` : Pipeline de logs centralisé.

---

## 4. Roadmap de Développement (Phase 1)

1. **Initialisation du Workspace Rust :** Setup du projet avec `axum` et `rig`.
2. **Runtime Wasmtime :** Implémentation du chargeur de plugins capable de lire un fichier `.wasm` et d'exécuter une fonction définie en WIT.
3. **Bridge Rig-Wasm :** Créer la logique qui transforme un `ToolCall` de Rig en appel vers le plugin Wasm correspondant.
4. **MVP Connecteur :** Création d'un plugin simple pour Telegram.
5. **Interface TUI :** Affichage basique des messages et des exécutions de plugins via Ratatui.


Exemples : 
```wit
package project-x:core;

interface host-api {
    /// Permet au plugin de stocker une info de session (ex: token, dernier offset)
    /// Le Host s'occupe de chiffrer/stocker ça dans la base locale (SQLite/Tantivy)
    kv-set: func(key: string, value: list<u8>) -> result<_, string>;
    kv-get: func(key: string) -> result<list<u8>, string>;

    /// Logging unifié (sera affiché dans ta TUI Ratatui)
    enum log-level { info, warn, error, debug }
    log: func(level: log-level, message: string);
}

interface messaging-types {
    record user {
        id: string,
        display-name: string,
    }

    record message {
        id: string,
        sender: user,
        text: string,
        metadata: option<string>, // JSON pour les spécificités (emojis, etc.)
    }
}

world messaging-plugin {
    import host-api;
    use messaging-types.{message, user};

    /// Le Host appelle ça quand il veut envoyer un message via ce plugin
    export post-message: func(channel-id: string, content: string) -> result<string, string>;

    /// Le Plugin appelle le Host (via une autre interface ou un stream) 
    /// lorsqu'il reçoit un message.
    /// Note: Dans un design pur WIT, on définit souvent des exports que le Host poll.
    export poll-events: func() -> list<message>;
}
```


```rust
use wasmtime::component::*;
use wasmtime::{Config, Engine, Store};

// On génère les traits Rust à partir du WIT
bindgen!({
    world: "messaging-plugin",
    path: "wit/provider.wit",
});

struct MyHostState {
    kv_store: Arc<SqlitePool>, // Ton stockage pour les plugins
    index: Arc<TantivyIndex>,  // Ton index pour l'audit/RAG
}

// Implémentation des Host Functions
impl host_api::Host for MyHostState {
    fn kv_set(&mut self, key: String, value: Vec<u8>) -> Result<(), String> {
        // Logique de stockage sécurisé ici
        Ok(())
    }

    fn log(&mut self, level: LogLevel, msg: String) {
        // Envoi vers Ratatui ou le fichier de log Markdown
        println!("[{:?}] Plugin log: {}", level, msg);
    }
    
    // ... kv_get, etc.
}
```