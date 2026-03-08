//go:build cgo

package memory

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

func init() {
	sqlite_vec.Auto()
}

// openSQLiteDB opens a SQLite database with WAL mode, foreign keys, and sqlite-vec loaded.
func openSQLiteDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=ON")
	if err != nil {
		return nil, err
	}
	// Verify sqlite-vec extension loaded
	var vecVersion string
	if err := db.QueryRow("SELECT vec_version()").Scan(&vecVersion); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
