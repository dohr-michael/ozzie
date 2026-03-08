package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dohr-michael/ozzie/pkg/names"
)

// SQLiteStore implements Store using SQLite with FTS5 for full-text search.
// Markdown files are kept in sync on disk for user transparency.
type SQLiteStore struct {
	db  *sql.DB
	dir string // base directory for .md files
	mu  sync.RWMutex
}

// NewSQLiteStore opens (or creates) the SQLite memory database and runs migrations.
func NewSQLiteStore(dir string) (*SQLiteStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create memory dir: %w", err)
	}

	dbPath := filepath.Join(dir, "memory.db")
	db, err := openSQLiteDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	store := &SQLiteStore{db: db, dir: dir}
	if err := store.createTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}
	return store, nil
}

func (s *SQLiteStore) createTables() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS memories (
			id              TEXT PRIMARY KEY,
			title           TEXT NOT NULL,
			source          TEXT NOT NULL DEFAULT '',
			type            TEXT NOT NULL CHECK(type IN ('preference','fact','procedure','context')),
			tags            TEXT NOT NULL DEFAULT '[]',
			created_at      TEXT NOT NULL,
			updated_at      TEXT NOT NULL,
			last_used_at    TEXT NOT NULL,
			confidence      REAL NOT NULL DEFAULT 0.8,
			importance      TEXT NOT NULL DEFAULT 'normal'
			                CHECK(importance IN ('core','important','normal','ephemeral')),
			embedding_model TEXT NOT NULL DEFAULT '',
			indexed_at      TEXT,
			content         TEXT NOT NULL DEFAULT '',
			merged_into     TEXT REFERENCES memories(id) ON DELETE SET NULL
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
			title, content, tags,
			content=memories, content_rowid=rowid,
			tokenize='porter unicode61'
		)`,
		// Triggers to keep FTS5 in sync
		`CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
			INSERT INTO memories_fts(rowid, title, content, tags)
			VALUES (new.rowid, new.title, new.content, new.tags);
		END`,
		`CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
			INSERT INTO memories_fts(memories_fts, rowid, title, content, tags)
			VALUES ('delete', old.rowid, old.title, old.content, old.tags);
		END`,
		`CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
			INSERT INTO memories_fts(memories_fts, rowid, title, content, tags)
			VALUES ('delete', old.rowid, old.title, old.content, old.tags);
			INSERT INTO memories_fts(rowid, title, content, tags)
			VALUES (new.rowid, new.title, new.content, new.tags);
		END`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}
	return nil
}

// Create adds a new memory entry.
func (s *SQLiteStore) Create(entry *MemoryEntry, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.ID == "" {
		entry.ID = names.GenerateID("mem", func(candidate string) bool {
			var exists bool
			_ = s.db.QueryRow("SELECT 1 FROM memories WHERE id = ?", candidate).Scan(&exists)
			return exists
		})
	}
	now := time.Now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = now
	}
	if entry.LastUsedAt.IsZero() {
		entry.LastUsedAt = now
	}
	if entry.Confidence == 0 {
		entry.Confidence = 0.8
	}
	if entry.Importance == "" {
		entry.Importance = ImportanceNormal
	}

	tagsJSON, _ := json.Marshal(entry.Tags)

	_, err := s.db.Exec(`INSERT INTO memories
		(id, title, source, type, tags, created_at, updated_at, last_used_at,
		 confidence, importance, embedding_model, indexed_at, content, merged_into)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.Title, entry.Source, string(entry.Type),
		string(tagsJSON),
		entry.CreatedAt.Format(time.RFC3339Nano),
		entry.UpdatedAt.Format(time.RFC3339Nano),
		entry.LastUsedAt.Format(time.RFC3339Nano),
		entry.Confidence, string(entry.Importance),
		entry.EmbeddingModel, formatTimePtr(entry.IndexedAt),
		content, nilIfEmpty(entry.MergedInto),
	)
	if err != nil {
		return fmt.Errorf("insert memory: %w", err)
	}

	// Sync markdown to disk
	_ = s.writeContentFile(entry.ID, content)
	return nil
}

// Get retrieves a memory entry and its content by ID.
func (s *SQLiteStore) Get(id string) (*MemoryEntry, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, content, err := s.getUnlocked(id)
	if err != nil {
		return nil, "", err
	}
	return entry, content, nil
}

func (s *SQLiteStore) getUnlocked(id string) (*MemoryEntry, string, error) {
	row := s.db.QueryRow(`SELECT id, title, source, type, tags, created_at, updated_at,
		last_used_at, confidence, importance, embedding_model, indexed_at, content, merged_into
		FROM memories WHERE id = ?`, id)

	entry := &MemoryEntry{}
	var tagsJSON, createdAt, updatedAt, lastUsedAt string
	var importance, indexedAt, mergedInto sql.NullString
	var content string

	err := row.Scan(&entry.ID, &entry.Title, &entry.Source, &entry.Type,
		&tagsJSON, &createdAt, &updatedAt, &lastUsedAt,
		&entry.Confidence, &importance, &entry.EmbeddingModel, &indexedAt,
		&content, &mergedInto)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", fmt.Errorf("memory %q not found", id)
		}
		return nil, "", fmt.Errorf("get memory: %w", err)
	}

	_ = json.Unmarshal([]byte(tagsJSON), &entry.Tags)
	entry.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	entry.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	entry.LastUsedAt, _ = time.Parse(time.RFC3339Nano, lastUsedAt)
	if importance.Valid {
		entry.Importance = ImportanceLevel(importance.String)
	}
	if indexedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, indexedAt.String)
		entry.IndexedAt = &t
	}
	if mergedInto.Valid {
		entry.MergedInto = mergedInto.String
	}

	return entry, content, nil
}

// Update replaces a memory entry's metadata and content.
func (s *SQLiteStore) Update(entry *MemoryEntry, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry.UpdatedAt = time.Now()
	tagsJSON, _ := json.Marshal(entry.Tags)
	if entry.Importance == "" {
		entry.Importance = ImportanceNormal
	}

	result, err := s.db.Exec(`UPDATE memories SET
		title=?, source=?, type=?, tags=?, updated_at=?, last_used_at=?,
		confidence=?, importance=?, embedding_model=?, indexed_at=?,
		content=?, merged_into=?
		WHERE id=?`,
		entry.Title, entry.Source, string(entry.Type), string(tagsJSON),
		entry.UpdatedAt.Format(time.RFC3339Nano),
		entry.LastUsedAt.Format(time.RFC3339Nano),
		entry.Confidence, string(entry.Importance),
		entry.EmbeddingModel, formatTimePtr(entry.IndexedAt),
		content, nilIfEmpty(entry.MergedInto),
		entry.ID,
	)
	if err != nil {
		return fmt.Errorf("update memory: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("memory %q not found", entry.ID)
	}

	_ = s.writeContentFile(entry.ID, content)
	return nil
}

// Delete removes a memory entry and its content file.
func (s *SQLiteStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`DELETE FROM memories WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("memory %q not found", id)
	}

	_ = os.Remove(s.contentPath(id))
	return nil
}

// List returns all active memory entries (excluding merged ones).
func (s *SQLiteStore) List() ([]*MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, title, source, type, tags, created_at, updated_at,
		last_used_at, confidence, importance, embedding_model, indexed_at, merged_into
		FROM memories WHERE merged_into IS NULL ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	var entries []*MemoryEntry
	for rows.Next() {
		entry := &MemoryEntry{}
		var tagsJSON, createdAt, updatedAt, lastUsedAt string
		var importance, indexedAt, mergedInto sql.NullString

		if err := rows.Scan(&entry.ID, &entry.Title, &entry.Source, &entry.Type,
			&tagsJSON, &createdAt, &updatedAt, &lastUsedAt,
			&entry.Confidence, &importance, &entry.EmbeddingModel, &indexedAt, &mergedInto); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}

		_ = json.Unmarshal([]byte(tagsJSON), &entry.Tags)
		entry.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		entry.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		entry.LastUsedAt, _ = time.Parse(time.RFC3339Nano, lastUsedAt)
		if importance.Valid {
			entry.Importance = ImportanceLevel(importance.String)
		}
		if indexedAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, indexedAt.String)
			entry.IndexedAt = &t
		}
		if mergedInto.Valid {
			entry.MergedInto = mergedInto.String
		}

		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// SearchFTS performs a full-text search using FTS5.
func (s *SQLiteStore) SearchFTS(query string, limit int) ([]*MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	rows, err := s.db.Query(`SELECT m.id, m.title, m.source, m.type, m.tags,
		m.created_at, m.updated_at, m.last_used_at, m.confidence, m.importance,
		m.embedding_model, m.indexed_at, m.merged_into,
		rank
		FROM memories_fts f
		JOIN memories m ON m.rowid = f.rowid
		WHERE memories_fts MATCH ?
		AND m.merged_into IS NULL
		ORDER BY rank
		LIMIT ?`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()

	var entries []*MemoryEntry
	for rows.Next() {
		entry := &MemoryEntry{}
		var tagsJSON, createdAt, updatedAt, lastUsedAt string
		var importance, indexedAt, mergedInto sql.NullString
		var rank float64

		if err := rows.Scan(&entry.ID, &entry.Title, &entry.Source, &entry.Type,
			&tagsJSON, &createdAt, &updatedAt, &lastUsedAt,
			&entry.Confidence, &importance, &entry.EmbeddingModel, &indexedAt, &mergedInto,
			&rank); err != nil {
			return nil, fmt.Errorf("scan fts result: %w", err)
		}

		_ = json.Unmarshal([]byte(tagsJSON), &entry.Tags)
		entry.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		entry.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		entry.LastUsedAt, _ = time.Parse(time.RFC3339Nano, lastUsedAt)
		if importance.Valid {
			entry.Importance = ImportanceLevel(importance.String)
		}
		if indexedAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, indexedAt.String)
			entry.IndexedAt = &t
		}
		if mergedInto.Valid {
			entry.MergedInto = mergedInto.String
		}

		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// Touch updates LastUsedAt without changing content (lightweight reinforcement).
func (s *SQLiteStore) Touch(id string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE memories SET last_used_at = ? WHERE id = ?`,
		now.Format(time.RFC3339Nano), id)
	return err
}

// UpdateConfidence updates only the confidence field.
func (s *SQLiteStore) UpdateConfidence(id string, confidence float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE memories SET confidence = ? WHERE id = ?`, confidence, id)
	return err
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for use by vector store and migration.
func (s *SQLiteStore) DB() *sql.DB {
	return s.db
}

// --- helpers ---

func (s *SQLiteStore) contentPath(id string) string {
	return filepath.Join(s.dir, "entries", id+".md")
}

func (s *SQLiteStore) writeContentFile(id, content string) error {
	dir := filepath.Join(s.dir, "entries")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.contentPath(id), []byte(content), 0o644)
}

func formatTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339Nano)
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

var _ Store = (*SQLiteStore)(nil)
