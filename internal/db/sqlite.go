// sqlite.go handles SQLite database initialization: opening the database file
// (or in-memory for tests), running schema migrations to create tables and
// indexes, and providing the raw *sql.DB connection to the Repository.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func Open(dataDir string) (*DB, error) {
	dbDir := filepath.Join(dataDir, "db")
	if err := os.MkdirAll(dbDir, 0750); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	dbPath := filepath.Join(dbDir, "authbox.db")
	return openDB(dbPath)
}

func OpenMemory() (*DB, error) {
	return openDB(":memory:")
}

func openDB(dsn string) (*DB, error) {
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}

	conn.SetMaxOpenConns(1)

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migration: %w", err)
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Conn() *sql.DB {
	return db.conn
}

func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY
		)`,
		`CREATE TABLE IF NOT EXISTS service_accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_id TEXT UNIQUE NOT NULL,
			client_secret_hash TEXT NOT NULL,
			description TEXT,
			role TEXT NOT NULL DEFAULT 'viewer',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_used_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS ssh_certs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			serial TEXT NOT NULL,
			principal TEXT NOT NULL,
			issued_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS fido2_credentials (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uid TEXT NOT NULL,
			credential_data TEXT NOT NULL,
			registered_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sync_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version INTEGER NOT NULL,
			table_name TEXT NOT NULL,
			operation TEXT NOT NULL,
			row_id INTEGER NOT NULL,
			data TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_log_version ON sync_log(version)`,
		`CREATE INDEX IF NOT EXISTS idx_fido2_uid ON fido2_credentials(uid)`,
		`CREATE INDEX IF NOT EXISTS idx_ssh_certs_username ON ssh_certs(username)`,
		`CREATE TABLE IF NOT EXISTS employee_types (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			value TEXT UNIQUE NOT NULL,
			label TEXT NOT NULL,
			emoji TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0
		)`,
		`INSERT OR IGNORE INTO employee_types (value, label, emoji, sort_order) VALUES ('employee', 'Employee', '👤', 1)`,
		`INSERT OR IGNORE INTO employee_types (value, label, emoji, sort_order) VALUES ('contractor', 'Contractor', '👷', 2)`,
		`INSERT OR IGNORE INTO employee_types (value, label, emoji, sort_order) VALUES ('service', 'Service', '🤖', 3)`,
		`INSERT OR IGNORE INTO employee_types (value, label, emoji, sort_order) VALUES ('contact', 'Contact', '🪪', 4)`,
		`CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, m := range migrations {
		if _, err := db.conn.Exec(m); err != nil {
			return fmt.Errorf("executing migration: %w", err)
		}
	}
	return nil
}
