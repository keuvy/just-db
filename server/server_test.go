package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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

func TestImportUploadRequiresConfirm(t *testing.T) {
	srv := New(Options{DataDir: t.TempDir()})
	body, contentType := importUploadBody(t, "x.dump", "false", "SELECT 1;\n")
	req := httptest.NewRequest(http.MethodPost, "/api/import", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "import not confirmed") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestImportUploadRejectsNonDump(t *testing.T) {
	srv := New(Options{DataDir: t.TempDir()})
	body, contentType := importUploadBody(t, "notes.txt", "true", "nope")
	req := httptest.NewRequest(http.MethodPost, "/api/import", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid dump file") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func importUploadBody(t *testing.T, filename, confirm, contents string) (*bytes.Buffer, string) {
	t.Helper()
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	if err := w.WriteField("engine", "postgres"); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteField("connection", `{"user":"u","database":"d"}`); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteField("confirm", confirm); err != nil {
		t.Fatal(err)
	}
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(contents)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf, w.FormDataContentType()
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

func TestProfilesRoundTrip(t *testing.T) {
	t.Setenv("JUSTDB_PROFILES_KEY", "")
	srv := New(Options{DataDir: t.TempDir()})
	body := strings.NewReader(`{"engine":"postgres","connection":{"host":"db","port":5432,"user":"u","password":"p","database":"app","sslMode":"disable"}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/profiles/prod", body)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status %d body %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("put response leaked password: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/profiles", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"name": "prod"`) {
		t.Fatalf("list body %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"password"`) {
		t.Fatalf("list leaked password: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/profiles/prod", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"password": "p"`) {
		t.Fatalf("get should include password: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/profiles/prod", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status %d body %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/profiles/prod", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("after delete status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestProfilesListMultiple(t *testing.T) {
	t.Setenv("JUSTDB_PROFILES_KEY", "")
	srv := New(Options{DataDir: t.TempDir()})
	for _, name := range []string{"alpha", "beta"} {
		body := strings.NewReader(`{"engine":"mysql","connection":{"host":"127.0.0.1","port":3306,"user":"root"}}`)
		req := httptest.NewRequest(http.MethodPut, "/api/profiles/"+name, body)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("put %s status %d body %s", name, rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/profiles", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"name": "alpha"`) || !strings.Contains(rec.Body.String(), `"name": "beta"`) {
		t.Fatalf("list should include both profiles: %s", rec.Body.String())
	}
}

func TestProfileInvalidName(t *testing.T) {
	t.Setenv("JUSTDB_PROFILES_KEY", "")
	srv := New(Options{DataDir: t.TempDir()})
	body := strings.NewReader(`{"engine":"postgres","connection":{"user":"u"}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/profiles/key", body)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestProfileWithoutDatabase(t *testing.T) {
	t.Setenv("JUSTDB_PROFILES_KEY", "")
	srv := New(Options{DataDir: t.TempDir()})
	body := strings.NewReader(`{"engine":"mysql","connection":{"host":"127.0.0.1","port":3306,"user":"root","password":"x","sslMode":"prefer"}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/profiles/plenario%20digital", body)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"name": "plenario digital"`) {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestProfileUnknownEngine(t *testing.T) {
	t.Setenv("JUSTDB_PROFILES_KEY", "")
	srv := New(Options{DataDir: t.TempDir()})
	body := strings.NewReader(`{"engine":"sqlite","connection":{"user":"u","database":"d"}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/profiles/prod", body)
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

func TestDumpDownloadAndDelete(t *testing.T) {
	dir := t.TempDir()
	backups := filepath.Join(dir, "backups")
	if err := os.Mkdir(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	name := "sample dump.sql"
	data := "SELECT 1;\n"
	for _, file := range []string{name, "keep.dump", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(backups, file), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	srv := New(Options{DataDir: dir})
	endpoint := "/api/dumps/" + url.PathEscape(name)
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(method, endpoint, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Disposition"), `attachment; filename="sample dump.sql"`) {
			t.Fatalf("download %s: %d, %v", method, rec.Code, rec.Header())
		}
		if method == http.MethodGet && rec.Body.String() != data {
			t.Fatalf("download contents: %q", rec.Body.String())
		}
	}
	for _, path := range []string{"notes.txt", "..%2Fsample%20dump.sql"} {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/dumps/"+path, nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("delete %s: %d, %s", path, rec.Code, rec.Body.String())
		}
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, endpoint, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d, %s", rec.Code, rec.Body.String())
	}
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		rec = httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(method, endpoint, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("after delete %s: %d", method, rec.Code)
		}
	}
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dumps", nil))
	if strings.Contains(rec.Body.String(), name) || !strings.Contains(rec.Body.String(), "keep.dump") {
		t.Fatalf("list after delete: %s", rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(backups, "notes.txt")); err != nil {
		t.Fatal(err)
	}
}
