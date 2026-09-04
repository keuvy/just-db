package job

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestIsDumpName(t *testing.T) {
	for _, name := range []string{"app.sql", "app.dump", "app.backup", "app.pgdump", "APP.SQL"} {
		if !IsDumpName(name) {
			t.Fatalf("%s should be a dump name", name)
		}
	}
	for _, name := range []string{"notes.txt", "app", "app.dump.exe"} {
		if IsDumpName(name) {
			t.Fatalf("%s should not be a dump name", name)
		}
	}
}

func TestSafeFileName(t *testing.T) {
	name, err := SafeFileName("../../etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	if name != "passwd" {
		t.Fatalf("got %q", name)
	}
	if _, err := SafeFileName(".."); err == nil {
		t.Fatal("expected error")
	}
}

func TestDumpFileActions(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.sql")
	data := []byte("SELECT 1;\n")
	for _, path := range []string{filepath.Join(dir, "sample.sql"), filepath.Join(dir, "keep.dump"), filepath.Join(dir, "notes.txt"), outside} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "folder.sql"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.sql")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../outside.sql", outside, `..\outside.sql`, "notes.txt", "folder.sql", "link.sql"} {
		if err := DeleteDump(dir, name); !errors.Is(err, ErrInvalidDump) {
			t.Fatalf("delete %q: got %v, want invalid dump", name, err)
		}
	}
	dumps, err := ListDumps(dir)
	if err != nil || len(dumps) != 2 {
		t.Fatalf("list: %+v, %v", dumps, err)
	}
	copyPath := filepath.Join(t.TempDir(), "copy.sql")
	if err := CopyDump(dir, "sample.sql", copyPath); err != nil {
		t.Fatal(err)
	}
	if err := CopyDump(dir, "sample.sql", filepath.Join(dir, "sample.sql")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{copyPath, filepath.Join(dir, "sample.sql")} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != string(data) {
			t.Fatalf("copy changed contents at %s: %q, %v", path, got, err)
		}
	}
	if err := DeleteDump(dir, "sample.sql"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteDump(dir, "sample.sql"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("delete missing dump: %v", err)
	}
	for _, path := range []string{filepath.Join(dir, "keep.dump"), filepath.Join(dir, "notes.txt"), outside, copyPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("unrelated file removed: %s: %v", path, err)
		}
	}
}

func TestListDumps(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.dump"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dumps, err := ListDumps(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(dumps) != 1 || dumps[0].Name != "a.dump" {
		t.Fatalf("%+v", dumps)
	}
}
