package job

import (
	"context"
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
	format := engine.InferFormat(path, opts.Format)
	if format == "" {
		format = engine.FormatCustom
	}
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
	format := engine.InferFormat(path, opts.Format)
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
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".sql" && ext != ".dump" && ext != ".backup" && ext != ".pgdump" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
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
