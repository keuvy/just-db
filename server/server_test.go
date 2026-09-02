package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	srv := New(Options{Listen: "127.0.0.1:0", DataDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["mode"] != "server" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestEngines(t *testing.T) {
	srv := New(Options{DataDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/engines", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Engines []map[string]any `json:"engines"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Engines) != 2 {
		t.Fatalf("got %d engines", len(body.Engines))
	}
}

func TestImportRequiresConfirm(t *testing.T) {
	srv := New(Options{DataDir: t.TempDir()})
	body := strings.NewReader(`{"engine":"postgres","connection":{"user":"u","database":"d"},"fileName":"x.dump","confirm":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/import", body)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestUnknownEngine(t *testing.T) {
	srv := New(Options{DataDir: t.TempDir()})
	body := strings.NewReader(`{"engine":"sqlite","connection":{"user":"u","database":"d"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/test-connection", body)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestDumpsEmpty(t *testing.T) {
	srv := New(Options{DataDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/dumps", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}
