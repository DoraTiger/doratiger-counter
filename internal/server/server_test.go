package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/DoraTiger/doratiger-counter/internal/config"
	"github.com/DoraTiger/doratiger-counter/internal/data"
	"github.com/DoraTiger/doratiger-counter/internal/handler"
)

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()

	db, err := data.OpenSQLite(filepath.Join(t.TempDir(), "counter.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	if err := data.Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	repo, err := data.NewCounterRepo(db, "test_site")
	if err != nil {
		t.Fatalf("NewCounterRepo() error = %v", err)
	}
	t.Cleanup(func() {
		if err := repo.Stop(); err != nil {
			t.Errorf("repo.Stop() error = %v", err)
		}
		if err := db.Close(); err != nil {
			t.Errorf("db.Close() error = %v", err)
		}
	})

	cfg := &config.Config{
		Counter: config.CounterConfig{
			SiteKey:        "test_site",
			AllowedOrigins: []string{"blog.example.com"},
			EnableCors:     true,
		},
	}
	return New(handler.NewCounterHandler(map[string]*data.CounterRepo{"test_site": repo}, &cfg.Counter), cfg)
}

func TestCountRouteReturnsFourFieldContract(t *testing.T) {
	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/count?page=%2Fpost%2F&uid=visitor-1", nil)
	req.Header.Set("Origin", "https://blog.example.com")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusOK, res.Body.String())
	}
	var got map[string]int64
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := map[string]int64{"site_pv": 1, "page_pv": 1, "site_uv": 1, "page_uv": 1}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("response[%q] = %d, want %d; response = %#v", key, got[key], value, got)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("response fields = %#v, want exactly %#v", got, want)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "https://blog.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want request origin", got)
	}
}

func TestCountRouteRejectsOriginEvenWithAllowedReferer(t *testing.T) {
	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/count?page=%2Fpost%2F", nil)
	req.Header.Set("Origin", "https://evil.test")
	req.Header.Set("Referer", "https://blog.example.com/post/")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusForbidden, res.Body.String())
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("rejected origin received CORS header %q", got)
	}
}

func TestCountRouteRequiresPage(t *testing.T) {
	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/count", nil)
	req.Header.Set("Origin", "https://blog.example.com")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusBadRequest, res.Body.String())
	}
}

func TestHealthDoesNotRequireOrigin(t *testing.T) {
	router := newTestRouter(t)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/health", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusOK, res.Body.String())
	}
}

func TestCountRouteSeparatesSitesByOrigin(t *testing.T) {
	db, err := data.OpenSQLite(filepath.Join(t.TempDir(), "counter.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	if err := data.MigrateForSite(db, "dtc_site"); err != nil {
		t.Fatalf("MigrateForSite() error = %v", err)
	}

	superheaoz, err := data.NewCounterRepo(db, "dtc_site")
	if err != nil {
		t.Fatalf("NewCounterRepo(superheaoz) error = %v", err)
	}
	doratiger, err := data.NewCounterRepo(db, "doratiger_site")
	if err != nil {
		t.Fatalf("NewCounterRepo(doratiger) error = %v", err)
	}
	t.Cleanup(func() {
		for _, repo := range []*data.CounterRepo{superheaoz, doratiger} {
			if err := repo.Stop(); err != nil {
				t.Errorf("repo.Stop() error = %v", err)
			}
		}
		if err := db.Close(); err != nil {
			t.Errorf("db.Close() error = %v", err)
		}
	})

	cfg := &config.Config{
		Counter: config.CounterConfig{
			Sites: map[string]string{
				"www.superheaoz.top": "dtc_site",
				"www.doratiger.top":  "doratiger_site",
			},
			EnableCors: true,
		},
	}
	router := New(handler.NewCounterHandler(map[string]*data.CounterRepo{
		"dtc_site":       superheaoz,
		"doratiger_site": doratiger,
	}, &cfg.Counter), cfg)

	for _, origin := range []string{"https://www.superheaoz.top", "https://www.doratiger.top"} {
		req := httptest.NewRequest(http.MethodGet, "/count?page=%2F&uid=visitor-1", nil)
		req.Header.Set("Origin", origin)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("status for %s = %d, want 200; body = %s", origin, res.Code, res.Body.String())
		}
		var got map[string]int64
		if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode response for %s: %v", origin, err)
		}
		want := map[string]int64{"site_pv": 1, "page_pv": 1, "site_uv": 1, "page_uv": 1}
		if len(got) != len(want) {
			t.Fatalf("response for %s = %#v, want exactly %#v", origin, got, want)
		}
		for key, value := range want {
			if got[key] != value {
				t.Fatalf("response for %s [%q] = %d, want %d; response = %#v", origin, key, got[key], value, got)
			}
		}
	}
}
