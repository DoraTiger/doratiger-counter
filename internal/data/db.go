package data

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DB() *sql.DB { return s.db }

func OpenSQLite(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}

func Migrate(db *sql.DB) error {
	const currentSchemaVersion = 1
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback()

	var version int
	if err := tx.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > currentSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", version, currentSchemaVersion)
	}
	if version == currentSchemaVersion {
		return tx.Commit()
	}

	const schema = `
		CREATE TABLE IF NOT EXISTS page_stats (
			page_key TEXT PRIMARY KEY,
			page_count INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS site_stats (
			key TEXT PRIMARY KEY,
			value INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS site_visitors (
			site_key TEXT NOT NULL,
			visitor_hash TEXT NOT NULL,
			PRIMARY KEY (site_key, visitor_hash)
		);
		CREATE TABLE IF NOT EXISTS page_visitors (
			page_key TEXT NOT NULL,
			visitor_hash TEXT NOT NULL,
			PRIMARY KEY (page_key, visitor_hash)
		);`
	if _, err := tx.ExecContext(context.Background(), schema); err != nil {
		return fmt.Errorf("apply schema version 1: %w", err)
	}
	if _, err := tx.Exec(`PRAGMA user_version = 1`); err != nil {
		return fmt.Errorf("record schema version 1: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}
