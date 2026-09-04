package data

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCounterRepoStopPersistsPVAndIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "counter.db")
	db, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	repo, err := NewCounterRepo(db, "test_site")
	if err != nil {
		t.Fatalf("NewCounterRepo() error = %v", err)
	}
	if _, err := repo.Increment(context.Background(), "/post/", "visitor-1"); err != nil {
		t.Fatalf("Increment() error = %v", err)
	}

	if err := repo.Stop(); err != nil {
		t.Fatalf("first Stop() error = %v", err)
	}
	if err := repo.Stop(); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}

	var sitePV int64
	if err := db.QueryRow(`SELECT value FROM site_stats WHERE key = ?`, "test_site").Scan(&sitePV); err != nil {
		t.Fatalf("read persisted site PV: %v", err)
	}
	if sitePV != 1 {
		t.Fatalf("persisted site PV = %d, want 1", sitePV)
	}

	var pagePV int64
	if err := db.QueryRow(`SELECT page_count FROM page_stats WHERE page_key = ?`, "/post/").Scan(&pagePV); err != nil {
		t.Fatalf("read persisted page PV: %v", err)
	}
	if pagePV != 1 {
		t.Fatalf("persisted page PV = %d, want 1", pagePV)
	}
}

func TestLegacyDatabaseUpgradePreservesPVAndUVAcrossRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "counter.db")
	db, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}

	const legacySchema = `
		CREATE TABLE page_stats (page_key TEXT PRIMARY KEY, page_count INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE site_stats (key TEXT PRIMARY KEY, value INTEGER NOT NULL DEFAULT 0);
		INSERT INTO page_stats (page_key, page_count) VALUES ('/post/', 3);
		INSERT INTO site_stats (key, value) VALUES ('test_site', 40);`
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatalf("create legacy database: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() legacy database error = %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() second call error = %v", err)
	}

	var schemaVersion int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&schemaVersion); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if schemaVersion != 1 {
		t.Fatalf("schema version = %d, want 1", schemaVersion)
	}

	repo, err := NewCounterRepo(db, "test_site")
	if err != nil {
		t.Fatalf("NewCounterRepo() error = %v", err)
	}
	first, err := repo.Increment(context.Background(), "/post/", "visitor-1")
	if err != nil {
		t.Fatalf("first Increment() error = %v", err)
	}
	if first.SitePV != 41 || first.PagePV != 4 || first.SiteUV != 1 || first.PageUV != 1 {
		t.Fatalf("first result = %#v, want PV 41/4 and UV 1/1", first)
	}
	if err := repo.Stop(); err != nil {
		t.Fatalf("first Stop() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}

	db, err = OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("reopen SQLite error = %v", err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() reopened database error = %v", err)
	}
	repo, err = NewCounterRepo(db, "test_site")
	if err != nil {
		t.Fatalf("NewCounterRepo() after reopen error = %v", err)
	}
	second, err := repo.Increment(context.Background(), "/post/", "visitor-1")
	if err != nil {
		t.Fatalf("second Increment() error = %v", err)
	}
	if second.SitePV != 42 || second.PagePV != 5 || second.SiteUV != 1 || second.PageUV != 1 {
		t.Fatalf("second result = %#v, want PV 42/5 and stable UV 1/1", second)
	}
	if err := repo.Stop(); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}

	var rawVisitorCount int
	if err := db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM site_visitors WHERE visitor_hash = 'visitor-1') +
			(SELECT COUNT(*) FROM page_visitors WHERE visitor_hash = 'visitor-1')`).Scan(&rawVisitorCount); err != nil {
		t.Fatalf("check raw visitor storage: %v", err)
	}
	if rawVisitorCount != 0 {
		t.Fatalf("raw visitor ID was persisted %d times", rawVisitorCount)
	}
}

func TestNewCounterRepoRejectsIncompatibleCurrentSchema(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "counter.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE site_stats (key TEXT PRIMARY KEY, value INTEGER NOT NULL DEFAULT 0);
		PRAGMA user_version = 1;`); err != nil {
		t.Fatalf("create incompatible database: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	if repo, err := NewCounterRepo(db, "test_site"); err == nil {
		_ = repo.Stop()
		t.Fatal("NewCounterRepo() accepted a current-version database with missing tables")
	}
}
