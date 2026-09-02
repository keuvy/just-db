package job

import (
	"os"
	"path/filepath"
	"testing"
)

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
