package memory

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

// openSQLiteDB opens a SQLite database with WAL mode and foreign keys.
// Uses modernc.org/sqlite (pure Go, FTS5 built-in, no CGo required).
func openSQLiteDB(path string) (*sql.DB, error) {
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	return db, nil
}
