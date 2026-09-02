package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

var (
	ErrNotImplemented     = errors.New("not implemented")
	ErrUnknownEngine      = errors.New("unknown engine")
	ErrToolsMissing       = errors.New("database client tools not found")
	ErrInvalidConnection  = errors.New("invalid connection")
	ErrImportNotConfirmed = errors.New("import not confirmed")
	ErrUnsupportedFormat  = errors.New("unsupported dump format")
)

// Name is a stable engine identifier used in the API and UI.
type Name string

const (
	Postgres Name = "postgres"
	MySQL    Name = "mysql"
)

const (
	FormatSQL    = "sql"
	FormatCustom = "custom"
)

// Connection is a same-engine dump/restore target. Password must never be
// logged or placed on a process command line.
type Connection struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password,omitempty"`
	Database string `json:"database"`
	SSLMode  string `json:"sslMode,omitempty"`
}

func (c Connection) Normalized(defaultPort int) Connection {
	if c.Host == "" {
		c.Host = "127.0.0.1"
	}
	if c.Port == 0 {
		c.Port = defaultPort
	}
	if c.SSLMode == "" {
		c.SSLMode = "prefer"
	}
	return c
}

func (c Connection) Validate() error {
	if strings.TrimSpace(c.User) == "" {
		return fmt.Errorf("%w: user is required", ErrInvalidConnection)
	}
	if strings.TrimSpace(c.Database) == "" {
		return fmt.Errorf("%w: database is required", ErrInvalidConnection)
	}
	return nil
}

type ExportOptions struct {
	Format string   `json:"format"` // sql or custom (postgres)
	Tables []string `json:"tables,omitempty"`
}

type ImportOptions struct {
	Format       string `json:"format"`
	DropExisting bool   `json:"dropExisting"`
	Confirm      bool   `json:"confirm"`
}

func InferFormat(path, explicit string) string {
	if explicit != "" {
		return strings.ToLower(strings.TrimSpace(explicit))
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".sql":
		return FormatSQL
	case ".dump", ".backup", ".pgdump":
		return FormatCustom
	default:
		return ""
	}
}

func DefaultFormat(name Name) string {
	if name == MySQL {
		return FormatSQL
	}
	return FormatCustom
}

func CoerceFormat(name Name, format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if name == MySQL && (format == "" || format == FormatCustom) {
		return FormatSQL
	}
	if format == "" {
		return DefaultFormat(name)
	}
	return format
}

type Tool struct {
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	Found   bool   `json:"found"`
}

type Tools struct {
	Dump    Tool `json:"dump"`
	Restore Tool `json:"restore"`
	Client  Tool `json:"client"`
}

func (t Tools) Complete() bool {
	return t.Dump.Found && t.Restore.Found && t.Client.Found
}

// Info is a UI-facing description of an engine, plus detected client tools.
type Info struct {
	Name        Name   `json:"name"`
	DisplayName string `json:"displayName"`
	DefaultPort int    `json:"defaultPort"`
	Tools       Tools  `json:"tools"`
	Ready       bool   `json:"ready"`
}

type ExportResult struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	Format string `json:"format"`
}

type DumpFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

type Engine interface {
	Name() Name
	DisplayName() string
	DefaultPort() int
	DetectTools(ctx context.Context) (Tools, error)
	TestConnection(ctx context.Context, cfg Connection) error
	Export(ctx context.Context, cfg Connection, opts ExportOptions, out io.Writer) error
	Import(ctx context.Context, cfg Connection, opts ImportOptions, in io.Reader) error
}
