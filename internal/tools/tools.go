package tools

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"just-db/internal/engine"
)

func extraDirs() []string {
	dirs := []string{
		"/opt/homebrew/bin",
		"/usr/local/bin",
		"/opt/homebrew/opt/libpq/bin",
		"/opt/homebrew/opt/mysql-client/bin",
		"/opt/homebrew/opt/mariadb/bin",
		"/usr/local/opt/libpq/bin",
		"/usr/local/opt/mysql-client/bin",
		"/usr/pgsql-16/bin",
		"/usr/pgsql-15/bin",
		"/usr/pgsql-17/bin",
	}
	if home, err := os.UserHomeDir(); err == nil && runtime.GOOS == "darwin" {
		dirs = append(dirs, filepath.Join(home, "homebrew/bin"))
	}
	return dirs
}

// LookPath finds an executable on PATH, then in common Homebrew and Postgres dirs.
func LookPath(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	for _, dir := range extraDirs() {
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}

func Version(ctx context.Context, path string) string {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(out.String()), "\n")
	return strings.TrimSpace(line)
}

func Detect(ctx context.Context, names ...string) engine.Tool {
	if len(names) == 0 {
		return engine.Tool{}
	}
	for _, name := range names {
		path, err := LookPath(name)
		if err != nil {
			continue
		}
		return engine.Tool{
			Name:    name,
			Path:    path,
			Version: Version(ctx, path),
			Found:   true,
		}
	}
	return engine.Tool{Name: names[0], Found: false}
}
