package engine

import (
	"context"
	"errors"
	"io"
)

var (
	ErrNotImplemented = errors.New("not implemented")
	ErrUnknownEngine  = errors.New("unknown engine")
	ErrToolsMissing   = errors.New("database client tools not found")
)

// Name is a stable engine identifier used in the API and UI.
type Name string

const (
	Postgres Name = "postgres"
	MySQL    Name = "mysql"
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

type ExportOptions struct {
	Format string   `json:"format"` // sql or custom (postgres)
	Tables []string `json:"tables,omitempty"`
}

type ImportOptions struct {
	Format       string `json:"format"`
	DropExisting bool   `json:"dropExisting"`
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

type Engine interface {
	Name() Name
	DisplayName() string
	DefaultPort() int
	DetectTools(ctx context.Context) (Tools, error)
	TestConnection(ctx context.Context, cfg Connection) error
	Export(ctx context.Context, cfg Connection, opts ExportOptions, out io.Writer) error
	Import(ctx context.Context, cfg Connection, opts ImportOptions, in io.Reader) error
}
