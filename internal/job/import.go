package job

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"just-db/internal/engine"
)

func isCompressedDump(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".gz" || ext == ".zip"
}

// openImportFile validates compressed input before the engine can change the
// database. Archive names are never used as extraction paths. The temporary file
// also lets pg_restore read a seekable archive without making another copy.
func openImportFile(ctx context.Context, path, explicit string) (*os.File, string, func(), error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", nil, err
	}
	if !isCompressedDump(path) {
		if explicit == "auto" {
			explicit = ""
		}
		return file, engine.InferFormat(path, explicit), func() { _ = file.Close() }, nil
	}
	defer file.Close()

	var input io.ReadCloser
	switch strings.ToLower(filepath.Ext(path)) {
	case ".gz":
		input, err = gzip.NewReader(file)
	case ".zip":
		var info os.FileInfo
		info, err = file.Stat()
		if err == nil {
			input, err = zipDumpReader(file, info.Size())
		}
	}
	if err != nil {
		return nil, "", nil, fmt.Errorf("%w: cannot open compressed dump: %v", ErrInvalidDump, err)
	}
	defer input.Close()

	tmp, err := os.CreateTemp("", "justdb-unpacked-*")
	if err != nil {
		return nil, "", nil, err
	}
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}
	// Reading to EOF verifies gzip/ZIP checksums. Copy in chunks, with bounded
	// memory and cancellation even when a small archive expands to a large dump.
	if _, err := io.Copy(tmp, &importReader{ctx: ctx, reader: input}); err != nil {
		cleanup()
		return nil, "", nil, fmt.Errorf("%w: cannot decompress dump: %w", ErrInvalidDump, err)
	}
	var header [512]byte
	n, err := tmp.ReadAt(header[:], 0)
	if err != nil && err != io.EOF {
		cleanup()
		return nil, "", nil, err
	}
	format := engine.FormatSQL
	if bytes.HasPrefix(header[:n], []byte("PGDMP")) {
		format = engine.FormatCustom
	} else if n == 0 || bytes.ContainsRune(header[:n], 0) {
		cleanup()
		return nil, "", nil, fmt.Errorf("%w: archive must contain SQL or a PostgreSQL custom dump", ErrInvalidDump)
	}
	explicit = strings.ToLower(strings.TrimSpace(explicit))
	if explicit != "" && explicit != "auto" && explicit != format {
		cleanup()
		return nil, "", nil, fmt.Errorf("%w: compressed dump contains %s, requested %s", engine.ErrUnsupportedFormat, format, explicit)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, "", nil, err
	}
	return tmp, format, cleanup, nil
}

func zipDumpReader(file *os.File, size int64) (io.ReadCloser, error) {
	archive, err := zip.NewReader(file, size)
	if err != nil {
		return nil, err
	}
	var dump *zip.File
	for _, entry := range archive.File {
		// Ignore directories and macOS resource-fork metadata.
		if !entry.Mode().IsRegular() || strings.HasPrefix(entry.Name, "__MACOSX/") || strings.HasPrefix(filepath.Base(entry.Name), "._") {
			continue
		}
		if engine.InferFormat(entry.Name, "") == "" {
			continue
		}
		if dump != nil {
			return nil, fmt.Errorf("ZIP contains multiple dump files; choose an archive with exactly one dump")
		}
		dump = entry
	}
	if dump == nil {
		return nil, fmt.Errorf("ZIP must contain one .sql, .dump, .backup, or .pgdump file")
	}
	return dump.Open()
}

type importReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *importReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
