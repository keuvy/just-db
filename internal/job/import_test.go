package job

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"just-db/internal/engine"
)

type importEngine struct {
	engine.Engine
	name     engine.Name
	called   bool
	contents string
	opts     engine.ImportOptions
	path     string
	err      error
}

func (e *importEngine) Name() engine.Name { return e.name }

func (e *importEngine) Import(_ context.Context, _ engine.Connection, opts engine.ImportOptions, in io.Reader) error {
	e.called = true
	e.opts = opts
	if f, ok := in.(*os.File); ok {
		e.path = f.Name()
	}
	data, err := io.ReadAll(in)
	e.contents = string(data)
	if err != nil {
		return err
	}
	return e.err
}

func gzipBytes(t *testing.T, contents string) []byte {
	t.Helper()
	var out bytes.Buffer
	w := gzip.NewWriter(&out)
	if _, err := io.WriteString(w, contents); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func zipBytes(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for name, contents := range entries {
		// Store makes checksum corruption fixtures independent of deflate details.
		entry, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, contents); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestImportCompressedDump(t *testing.T) {
	const sql = "CREATE TABLE sample (id int);\nINSERT INTO sample VALUES (1);\n"
	const custom = "PGDMP\x01\x00sample"
	for _, tc := range []struct {
		name   string
		data   []byte
		want   string
		format string
		engine engine.Name
	}{
		{"backup.sql.gz", gzipBytes(t, sql), sql, engine.FormatSQL, engine.Postgres},
		{"backup.GZ", gzipBytes(t, sql), sql, engine.FormatSQL, engine.MySQL},
		{"backup.dump.gz", gzipBytes(t, custom), custom, engine.FormatCustom, engine.Postgres},
		{"backup.ZIP", zipBytes(t, map[string]string{"nested/backup.SQL": sql, "readme.txt": "backup", "__MACOSX/._backup.sql": "metadata"}), sql, engine.FormatSQL, engine.Postgres},
		{"mysql.zip", zipBytes(t, map[string]string{"backup.sql": sql}), sql, engine.FormatSQL, engine.MySQL},
		{"custom.zip", zipBytes(t, map[string]string{"backup.pgdump": custom}), custom, engine.FormatCustom, engine.Postgres},
		{"plain.sql", []byte(sql), sql, engine.FormatSQL, engine.Postgres},
		{"plain.dump", []byte(custom), custom, engine.FormatCustom, engine.Postgres},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.name)
			if err := os.WriteFile(path, tc.data, 0o600); err != nil {
				t.Fatal(err)
			}
			for _, format := range []string{"", "auto", tc.format} {
				eng := &importEngine{name: tc.engine}
				opts := engine.ImportOptions{Format: format, Confirm: true, DropExisting: true, DisableForeignKeys: true}
				if err := ImportFromFile(context.Background(), eng, engine.Connection{}, opts, path); err != nil {
					t.Fatal(err)
				}
				opts.Format = tc.format
				if !eng.called || eng.contents != tc.want || eng.opts != opts {
					t.Fatalf("restore got contents=%q opts=%+v", eng.contents, eng.opts)
				}
				if isCompressedDump(path) {
					if _, err := os.Stat(eng.path); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("temporary dump not removed: %v", err)
					}
				}
			}
			if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, tc.data) {
				t.Fatal("source changed")
			}
		})
	}
}

func TestInvalidCompressedDumpDoesNotStartRestore(t *testing.T) {
	badGzip := gzipBytes(t, "SELECT 1;")
	badGzip[len(badGzip)-8] ^= 0xff
	badZip := zipBytes(t, map[string]string{"backup.sql": "SELECT 1;"})
	badZip[bytes.Index(badZip, []byte("SELECT 1;"))] = 'X'
	for _, tc := range []struct {
		name string
		data []byte
		want string
	}{
		{"broken.gz", []byte("not gzip"), "cannot open"},
		{"broken.zip", []byte("not zip"), "cannot open"},
		{"checksum.gz", badGzip, "cannot decompress"},
		{"checksum.zip", badZip, "cannot decompress"},
		{"empty.gz", gzipBytes(t, ""), "must contain SQL"},
		{"binary.gz", gzipBytes(t, "\x00binary"), "must contain SQL"},
		{"empty.zip", zipBytes(t, map[string]string{}), "must contain one"},
		{"notes.zip", zipBytes(t, map[string]string{"readme.txt": "hi"}), "must contain one"},
		{"multiple.zip", zipBytes(t, map[string]string{"a.sql": "SELECT 1;", "b.dump": "PGDMP"}), "multiple dump"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			path := filepath.Join(t.TempDir(), tc.name)
			if err := os.WriteFile(path, tc.data, 0o600); err != nil {
				t.Fatal(err)
			}
			eng := &importEngine{name: engine.Postgres}
			err := ImportFromFile(context.Background(), eng, engine.Connection{}, engine.ImportOptions{Confirm: true, DropExisting: true}, path)
			if !errors.Is(err, ErrInvalidDump) || !strings.Contains(err.Error(), tc.want) || eng.called {
				t.Fatalf("error=%v called=%v", err, eng.called)
			}
			if entries, err := os.ReadDir(tmp); err != nil || len(entries) != 0 {
				t.Fatalf("temporary files left: %v, %v", entries, err)
			}
		})
	}
}

func TestCompressedImportGuardsAndCleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.gz")
	if err := os.WriteFile(path, gzipBytes(t, "PGDMP\x00test"), 0o600); err != nil {
		t.Fatal(err)
	}
	eng := &importEngine{name: engine.MySQL}
	if err := ImportFromFile(context.Background(), eng, engine.Connection{}, engine.ImportOptions{}, "missing.gz"); !errors.Is(err, engine.ErrImportNotConfirmed) {
		t.Fatal(err)
	}
	if err := ImportFromFile(context.Background(), eng, engine.Connection{}, engine.ImportOptions{Confirm: true}, path); !errors.Is(err, engine.ErrUnsupportedFormat) || eng.called {
		t.Fatalf("MySQL custom: %v", err)
	}
	eng.name = engine.Postgres
	if err := ImportFromFile(context.Background(), eng, engine.Connection{}, engine.ImportOptions{Confirm: true, Format: "sql"}, path); !errors.Is(err, engine.ErrUnsupportedFormat) || eng.called {
		t.Fatalf("format mismatch: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ImportFromFile(ctx, eng, engine.Connection{}, engine.ImportOptions{Confirm: true}, path); !errors.Is(err, context.Canceled) || eng.called {
		t.Fatalf("cancelled: %v", err)
	}
	eng.err = errors.New("restore failed")
	if err := ImportFromFile(context.Background(), eng, engine.Connection{}, engine.ImportOptions{Confirm: true}, path); !errors.Is(err, eng.err) {
		t.Fatal(err)
	}
	if _, err := os.Stat(eng.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file survived failed restore: %v", err)
	}
}
