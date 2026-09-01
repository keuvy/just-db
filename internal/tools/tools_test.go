package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectKnownBinary(t *testing.T) {
	// `true` is on virtually every Unix PATH.
	tool := Detect(context.Background(), "true")
	if !tool.Found {
		t.Fatal("expected to find true")
	}
	if filepath.Base(tool.Path) != "true" {
		t.Fatalf("path %q", tool.Path)
	}
}

func TestDetectMissing(t *testing.T) {
	tool := Detect(context.Background(), "just-db-no-such-binary")
	if tool.Found {
		t.Fatalf("found missing tool at %s", tool.Path)
	}
	if tool.Name != "just-db-no-such-binary" {
		t.Fatalf("name %q", tool.Name)
	}
}

func TestLookPathExtraDir(t *testing.T) {
	dir := t.TempDir()
	name := "justdb-fake-tool"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho fake 1.0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "/nonexistent")
	// extraDirs does not include temp; LookPath still searches PATH first.
	if _, err := LookPath(name); err == nil {
		t.Fatal("expected not found on empty PATH")
	}
}
