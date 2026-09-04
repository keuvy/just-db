package job

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"just-db/internal/engine"
	"just-db/internal/proc"
)

func BackupsDir(dataDir string) string {
	return filepath.Join(dataDir, "backups")
}

var ErrInvalidDump = errors.New("invalid dump file")

func IsDumpName(name string) bool {
	return isDumpName(filepath.Base(name))
}

func isDumpName(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".sql", ".dump", ".backup", ".pgdump":
		return true
	default:
		return false
	}
}

// ExistingDumpPath only accepts regular dump files directly in the backups folder.
func ExistingDumpPath(dir, name string) (string, error) {
	if name != filepath.Base(name) || strings.ContainsAny(name, `/\`+"\x00") || !isDumpName(name) {
		return "", ErrInvalidDump
	}
	path := filepath.Join(dir, name)
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", ErrInvalidDump
	}
	return path, nil
}

func DeleteDump(dir, name string) error {
	path, err := ExistingDumpPath(dir, name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func CopyDump(dir, name, dest string) error {
	path, err := ExistingDumpPath(dir, name)
	if err != nil {
		return err
	}
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	srcInfo, err := src.Stat()
	if err != nil {
		return err
	}
	if destInfo, err := os.Stat(dest); err == nil && os.SameFile(srcInfo, destInfo) {
		return nil
	}
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, src)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func SafeFileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = filepath.Base(name)
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid file name")
	}
	if strings.ContainsRune(name, os.PathSeparator) {
		return "", fmt.Errorf("invalid file name")
	}
	return name, nil
}

func DefaultFileName(eng engine.Name, database, format string) string {
	ext := ".dump"
	if format == engine.FormatSQL {
		ext = ".sql"
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	db := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, database)
	if db == "" {
		db = "db"
	}
	return fmt.Sprintf("%s-%s-%s%s", eng, db, stamp, ext)
}

func ExportToFile(ctx context.Context, eng engine.Engine, cfg engine.Connection, opts engine.ExportOptions, path string) (engine.ExportResult, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return engine.ExportResult{}, err
	}
	format := engine.CoerceFormat(eng.Name(), engine.InferFormat(path, opts.Format))
	opts.Format = format
	file, err := os.Create(path)
	if err != nil {
		return engine.ExportResult{}, err
	}
	counter := &proc.CountingWriter{W: file}
	exportErr := eng.Export(ctx, cfg, opts, counter)
	closeErr := file.Close()
	if exportErr != nil {
		_ = os.Remove(path)
		return engine.ExportResult{}, exportErr
	}
	if closeErr != nil {
		return engine.ExportResult{}, closeErr
	}
	return engine.ExportResult{Path: path, Bytes: counter.Count(), Format: format}, nil
}

func ImportFromFile(ctx context.Context, eng engine.Engine, cfg engine.Connection, opts engine.ImportOptions, path string) error {
	if !opts.Confirm {
		return engine.ErrImportNotConfirmed
	}
	format := engine.CoerceFormat(eng.Name(), engine.InferFormat(path, opts.Format))
	if format == "" {
		return fmt.Errorf("%w: cannot infer format from %s", engine.ErrUnsupportedFormat, path)
	}
	opts.Format = format
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return eng.Import(ctx, cfg, opts, file)
}

func ListDumps(dir string) ([]engine.DumpFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []engine.DumpFile{}, nil
		}
		return nil, err
	}
	out := make([]engine.DumpFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isDumpName(name) {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		out = append(out, engine.DumpFile{
			Name:    name,
			Path:    filepath.Join(dir, name),
			Size:    info.Size(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

var _ io.Writer = (*proc.CountingWriter)(nil)
