package server

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompressedImportRequests(t *testing.T) {
	for _, ext := range []string{".gz", ".zip"} {
		t.Run(ext, func(t *testing.T) {
			var compressed bytes.Buffer
			var writer io.Closer
			var contents io.Writer
			if ext == ".gz" {
				archive := gzip.NewWriter(&compressed)
				writer = archive
				contents = archive
			} else {
				archive := zip.NewWriter(&compressed)
				writer = archive
				var err error
				contents, err = archive.Create("nested/backup.sql")
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := io.WriteString(contents, "SELECT 1;\n"); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			dataDir := t.TempDir()
			tmpDir := t.TempDir()
			t.Setenv("TMPDIR", tmpDir)
			srv := New(Options{DataDir: dataDir})
			name := "backup" + ext
			for _, upload := range []bool{true, false} {
				var body bytes.Buffer
				contentType := "application/json"
				// An empty database stops the real engine before tool discovery or
				// connections, after archive validation and decompression succeed.
				if upload {
					form := multipart.NewWriter(&body)
					for key, value := range map[string]string{"engine": "postgres", "connection": "{}", "format": "auto", "confirm": "true"} {
						if err := form.WriteField(key, value); err != nil {
							t.Fatal(err)
						}
					}
					file, err := form.CreateFormFile("file", name)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := file.Write(compressed.Bytes()); err != nil {
						t.Fatal(err)
					}
					if err := form.Close(); err != nil {
						t.Fatal(err)
					}
					contentType = form.FormDataContentType()
				} else {
					if err := os.MkdirAll(filepath.Join(dataDir, "backups"), 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dataDir, "backups", name), compressed.Bytes(), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := json.NewEncoder(&body).Encode(map[string]any{"engine": "postgres", "fileName": name, "format": "auto", "confirm": true}); err != nil {
						t.Fatal(err)
					}
				}
				req := httptest.NewRequest(http.MethodPost, "/api/import", &body)
				req.Header.Set("Content-Type", contentType)
				rec := httptest.NewRecorder()
				srv.Handler().ServeHTTP(rec, req)
				if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "pick a database") {
					t.Fatalf("upload=%v status=%d body=%s", upload, rec.Code, rec.Body.String())
				}
				if entries, err := os.ReadDir(tmpDir); err != nil || len(entries) != 0 {
					t.Fatalf("temporary files left: %v, %v", entries, err)
				}
			}
		})
	}
}
