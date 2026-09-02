package appdir

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBackupsCreatesDir(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG data dir is Linux-only")
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	dir, err := Backups()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(dir)) != "just-db" {
		t.Fatalf("unexpected path %s", dir)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("backups dir: %v", err)
	}
}
