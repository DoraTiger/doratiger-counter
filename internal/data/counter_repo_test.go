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
	if err := MigrateForSite(db, "test_site"); err != nil {
		t.Fatalf("MigrateForSite() legacy database error = %v", err)
	}
	if err := MigrateForSite(db, "test_site"); err != nil {
		t.Fatalf("MigrateForSite() second call error = %v", err)
	}

	var schemaVersion int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&schemaVersion); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if schemaVersion != 2 {
		t.Fatalf("schema version = %d, want 2", schemaVersion)
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
	if err := MigrateForSite(db, "test_site"); err != nil {
		t.Fatalf("MigrateForSite() reopened database error = %v", err)
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
	if err := Migrate(db); err == nil {
		t.Fatal("Migrate() accepted a version 1 database with missing required tables")
	}
}

func TestMigrateForSitePartitionsLegacyPageData(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "counter.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	const schemaV1 = `
		CREATE TABLE page_stats (page_key TEXT PRIMARY KEY, page_count INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE site_stats (key TEXT PRIMARY KEY, value INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE site_visitors (site_key TEXT NOT NULL, visitor_hash TEXT NOT NULL, PRIMARY KEY (site_key, visitor_hash));
		CREATE TABLE page_visitors (page_key TEXT NOT NULL, visitor_hash TEXT NOT NULL, PRIMARY KEY (page_key, visitor_hash));
		INSERT INTO page_stats (page_key, page_count) VALUES ('/', 5039);
		INSERT INTO page_visitors (page_key, visitor_hash) VALUES ('/', 'legacy-hash');
		PRAGMA user_version = 1;`
	if _, err := db.Exec(schemaV1); err != nil {
		t.Fatalf("create version 1 database: %v", err)
	}

	if err := MigrateForSite(db, "dtc_site"); err != nil {
		t.Fatalf("MigrateForSite() error = %v", err)
	}

	var pagePV int64
	if err := db.QueryRow(`SELECT page_count FROM page_stats WHERE site_key = ? AND page_key = ?`, "dtc_site", "/").Scan(&pagePV); err != nil {
		t.Fatalf("read migrated superheaoz page PV: %v", err)
	}
	if pagePV != 5039 {
		t.Fatalf("migrated page PV = %d, want 5039", pagePV)
	}

	var doratigerRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM page_stats WHERE site_key = ?`, "doratiger_site").Scan(&doratigerRows); err != nil {
		t.Fatalf("read doratiger page rows: %v", err)
	}
	if doratigerRows != 0 {
		t.Fatalf("doratiger inherited %d legacy page rows", doratigerRows)
	}

	var visitorRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM page_visitors WHERE site_key = ? AND page_key = ? AND visitor_hash = ?`, "dtc_site", "/", "legacy-hash").Scan(&visitorRows); err != nil {
		t.Fatalf("read migrated page visitor: %v", err)
	}
	if visitorRows != 1 {
		t.Fatalf("migrated page visitor rows = %d, want 1", visitorRows)
	}
}

func TestCounterReposKeepIdenticalPagePathsSeparateAcrossSites(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "counter.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()
	if err := MigrateForSite(db, "dtc_site"); err != nil {
		t.Fatalf("MigrateForSite() error = %v", err)
	}

	superheaoz, err := NewCounterRepo(db, "dtc_site")
	if err != nil {
		t.Fatalf("NewCounterRepo(superheaoz) error = %v", err)
	}
	doratiger, err := NewCounterRepo(db, "doratiger_site")
	if err != nil {
		t.Fatalf("NewCounterRepo(doratiger) error = %v", err)
	}

	superResult, err := superheaoz.Increment(context.Background(), "/", "visitor-1")
	if err != nil {
		t.Fatalf("superheaoz Increment() error = %v", err)
	}
	doratigerResult, err := doratiger.Increment(context.Background(), "/", "visitor-1")
	if err != nil {
		t.Fatalf("doratiger Increment() error = %v", err)
	}
	if superResult.SitePV != 1 || superResult.PagePV != 1 || superResult.SiteUV != 1 || superResult.PageUV != 1 {
		t.Fatalf("superheaoz result = %#v, want independent 1/1/1/1", superResult)
	}
	if doratigerResult.SitePV != 1 || doratigerResult.PagePV != 1 || doratigerResult.SiteUV != 1 || doratigerResult.PageUV != 1 {
		t.Fatalf("doratiger result = %#v, want independent 1/1/1/1", doratigerResult)
	}
	if err := superheaoz.Stop(); err != nil {
		t.Fatalf("superheaoz Stop() error = %v", err)
	}
	if err := doratiger.Stop(); err != nil {
		t.Fatalf("doratiger Stop() error = %v", err)
	}

	rows, err := db.Query(`SELECT site_key, page_count FROM page_stats WHERE page_key = ? ORDER BY site_key`, "/")
	if err != nil {
		t.Fatalf("query persisted page stats: %v", err)
	}
	defer rows.Close()
	var got []struct {
		siteKey string
		count   int64
	}
	for rows.Next() {
		var row struct {
			siteKey string
			count   int64
		}
		if err := rows.Scan(&row.siteKey, &row.count); err != nil {
			t.Fatalf("scan persisted page stat: %v", err)
		}
		got = append(got, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate persisted page stats: %v", err)
	}
	if len(got) != 2 || got[0].siteKey != "doratiger_site" || got[0].count != 1 || got[1].siteKey != "dtc_site" || got[1].count != 1 {
		t.Fatalf("persisted page stats = %#v, want two independent site rows", got)
	}
}
