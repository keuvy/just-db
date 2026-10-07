package server

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"just-db/internal/engine"
)

func TestCreateDatabaseImportRequests(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture clients use a POSIX shell")
	}
	for _, target := range []struct{ engineName, format string }{{"postgres", "sql"}, {"postgres", "custom"}, {"mysql", "sql"}} {
		engineName, format := target.engineName, target.format
		sourceName := "source.sql"
		if format == "custom" {
			sourceName = "source.dump"
		}
		for _, upload := range []bool{false, true} {
			for _, fail := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/upload=%v/fail=%v", engineName, format, upload, fail), func(t *testing.T) {
					binDir := t.TempDir()
					logPath := filepath.Join(binDir, "commands")
					// Record actual engine invocations without connecting to a server.
					client := `#!/bin/sh
if [ "$1" = "--version" ]; then exit 0; fi
printf '%s\n' "$0" "$@" >> "$JUSTDB_TEST_COMMANDS"
printf '%s\n' END >> "$JUSTDB_TEST_COMMANDS"
for arg in "$@"; do
  case "$arg" in
    'CREATE DATABASE '*)
      if [ "$JUSTDB_TEST_CREATE_FAIL" = 1 ]; then
        printf '%s\n' 'database already exists or permission denied' >&2
        exit 1
      fi
      exit 0 ;;
  esac
done
cat >/dev/null
`
					for _, name := range []string{"psql", "pg_restore", "pg_dump", "mysql", "mysqldump"} {
						if err := os.WriteFile(filepath.Join(binDir, name), []byte(client), 0o700); err != nil {
							t.Fatal(err)
						}
					}
					t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
					t.Setenv("JUSTDB_TEST_COMMANDS", logPath)
					if fail {
						t.Setenv("JUSTDB_TEST_CREATE_FAIL", "1")
					} else {
						t.Setenv("JUSTDB_TEST_CREATE_FAIL", "0")
					}
					dataDir := t.TempDir()
					srv := New(Options{DataDir: dataDir})
					name := "restore`\" copy"
					conn := apiConnection{User: "test", Database: name}
					var body bytes.Buffer
					contentType := "application/json"
					if upload {
						form := multipart.NewWriter(&body)
						connection, _ := json.Marshal(conn)
						for key, value := range map[string]string{"engine": engineName, "connection": string(connection), "format": format, "confirm": "true", "createDatabase": "true"} {
							if err := form.WriteField(key, value); err != nil {
								t.Fatal(err)
							}
						}
						file, err := form.CreateFormFile("file", sourceName)
						if err != nil {
							t.Fatal(err)
						}
						_, _ = io.WriteString(file, "SELECT 1;")
						if err := form.Close(); err != nil {
							t.Fatal(err)
						}
						contentType = form.FormDataContentType()
					} else {
						if err := os.MkdirAll(filepath.Join(dataDir, "backups"), 0o700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(filepath.Join(dataDir, "backups", sourceName), []byte("SELECT 1;"), 0o600); err != nil {
							t.Fatal(err)
						}
						if err := json.NewEncoder(&body).Encode(importRequest{Engine: engine.Name(engineName), Connection: conn, Format: format, FileName: sourceName, CreateDatabase: true, Confirm: true}); err != nil {
							t.Fatal(err)
						}
					}
					req := httptest.NewRequest(http.MethodPost, "/api/import", &body)
					req.Header.Set("Content-Type", contentType)
					rec := httptest.NewRecorder()
					srv.Handler().ServeHTTP(rec, req)
					log, err := os.ReadFile(logPath)
					if err != nil {
						t.Fatal(err)
					}
					commands := strings.Split(strings.TrimSuffix(string(log), "END\n"), "END\n")
					createSQL := "CREATE DATABASE `restore``\" copy`"
					if engineName == "postgres" {
						createSQL = "CREATE DATABASE \"restore`\"\" copy\""
						if !strings.Contains(commands[0], "--dbname\npostgres\n") {
							t.Fatalf("creation did not connect to postgres: %s", log)
						}
					} else if strings.Contains(commands[0], "--database=") {
						t.Fatalf("creation must not select the missing database: %s", log)
					}
					if !strings.Contains(commands[0], createSQL+"\n") {
						t.Fatalf("missing quoted creation SQL: %s", log)
					}
					if !fail && format == "custom" && (len(commands) < 2 || !strings.Contains(commands[1], "pg_restore")) {
						t.Fatalf("custom dump did not use pg_restore: %s", log)
					}
					if fail {
						if rec.Code == http.StatusOK || len(commands) != 1 {
							t.Fatalf("creation failure continued restore: status=%d commands=%s", rec.Code, log)
						}
					} else if rec.Code != http.StatusOK || len(commands) != 2 || !strings.Contains(commands[1], name) {
						t.Fatalf("restore must follow creation into the reviewed database: status=%d body=%s commands=%s", rec.Code, rec.Body.String(), log)
					}
				})
			}
		}
	}
}

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
