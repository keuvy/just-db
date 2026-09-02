package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"just-db/internal/engine"
)

func TestHealthUnauthenticatedWhenAuthEnabled(t *testing.T) {
	srv := New(Options{DataDir: t.TempDir(), AuthUser: "admin", AuthPassword: "secret"})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health should be public, got %d", rec.Code)
	}
}

func TestAPIRequiresAuth(t *testing.T) {
	srv := New(Options{DataDir: t.TempDir(), AuthUser: "admin", AuthPassword: "secret"})
	req := httptest.NewRequest(http.MethodGet, "/api/engines", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/engines", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authorized got %d body %s", rec.Code, rec.Body.String())
	}
}

func TestWrongPassword(t *testing.T) {
	srv := New(Options{DataDir: t.TempDir(), AuthUser: "admin", AuthPassword: "secret"})
	req := httptest.NewRequest(http.MethodGet, "/api/engines", nil)
	req.SetBasicAuth("admin", "nope")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestDumpDownload(t *testing.T) {
	dir := t.TempDir()
	backups := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backups, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backups, "app.sql"), []byte("-- dump\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := New(Options{DataDir: dir})
	req := httptest.NewRequest(http.MethodGet, "/api/dumps/app.sql", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "-- dump\n" {
		t.Fatalf("body %q", rec.Body.String())
	}
}

func TestDefaults(t *testing.T) {
	srv := New(Options{
		DataDir:       t.TempDir(),
		DefaultEngine: engine.MySQL,
		DefaultConn: engine.Connection{
			Host:     "dbhost",
			Port:     3306,
			User:     "u",
			Password: "p",
			Database: "app",
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/defaults", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"engine": "mysql"`) || !strings.Contains(body, "dbhost") {
		t.Fatalf("body %s", body)
	}
}
