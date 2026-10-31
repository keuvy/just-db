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
	ErrInvalidExport      = errors.New("invalid export options")
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
	return nil
}

func (c Connection) RequireDatabase() error {
	if strings.TrimSpace(c.Database) == "" {
		return fmt.Errorf("%w: pick a database", ErrInvalidConnection)
	}
	return nil
}

func LineNames(raw string) []string {
	out := make([]string, 0)
	for _, line := range strings.Split(raw, "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

// Export content choices. Empty means ContentAll.
const (
	ContentAll    = "all"
	ContentSchema = "schema"
	ContentData   = "data"
)

type ExportOptions struct {
	Format string   `json:"format"` // sql or custom (postgres)
	Tables []string `json:"tables,omitempty"`
	// Content limits the dump to table structure or table rows.
	Content string `json:"content,omitempty"`
	// PerTable chooses structure and rows for each table. When set, it
	// replaces Tables and Content.
	PerTable []TableContent `json:"perTable,omitempty"`
}

// TableContent chooses the structure and rows of one table.
type TableContent struct {
	Name   string `json:"name"`
	Schema bool   `json:"schema"`
	Data   bool   `json:"data"`
}

// Selection is the validated table choice of an export.
type Selection struct {
	// All exports every table with Schema and Data. Otherwise only Tables
	// are exported, each with its own parts.
	All          bool
	Schema, Data bool
	Tables       []TableContent
}

// Selection validates the options and returns what the dump must contain.
func (o ExportOptions) Selection() (Selection, error) {
	if len(o.PerTable) > 0 {
		sel := Selection{}
		seen := map[string]bool{}
		for _, table := range o.PerTable {
			table.Name = strings.TrimSpace(table.Name)
			if table.Name == "" {
				return Selection{}, fmt.Errorf("%w: table name is empty", ErrInvalidExport)
			}
			if seen[table.Name] {
				return Selection{}, fmt.Errorf("%w: table %q is listed twice", ErrInvalidExport, table.Name)
			}
			seen[table.Name] = true
			if !table.Schema && !table.Data {
				continue
			}
			sel.Schema = sel.Schema || table.Schema
			sel.Data = sel.Data || table.Data
			sel.Tables = append(sel.Tables, table)
		}
		if len(sel.Tables) == 0 {
			return Selection{}, fmt.Errorf("%w: choose structure or data for at least one table", ErrInvalidExport)
		}
		return sel, nil
	}
	var schema, data bool
	switch strings.ToLower(strings.TrimSpace(o.Content)) {
	case "", ContentAll:
		schema, data = true, true
	case ContentSchema:
		schema = true
	case ContentData:
		data = true
	default:
		return Selection{}, fmt.Errorf("%w: content must be all, schema, or data", ErrInvalidExport)
	}
	sel := Selection{All: true, Schema: schema, Data: data}
	for _, name := range o.Tables {
		if name = strings.TrimSpace(name); name != "" {
			sel.All = false
			sel.Tables = append(sel.Tables, TableContent{Name: name, Schema: schema, Data: data})
		}
	}
	return sel, nil
}

// Groups splits the chosen tables into those dumped with structure and rows,
// structure only, and rows only.
func (s Selection) Groups() (both, schemaOnly, dataOnly []string) {
	for _, table := range s.Tables {
		switch {
		case table.Schema && table.Data:
			both = append(both, table.Name)
		case table.Schema:
			schemaOnly = append(schemaOnly, table.Name)
		default:
			dataOnly = append(dataOnly, table.Name)
		}
	}
	return both, schemaOnly, dataOnly
}

// Names lists the chosen tables; it is empty when every table is exported.
func (s Selection) Names() []string {
	names := make([]string, 0, len(s.Tables))
	for _, table := range s.Tables {
		names = append(names, table.Name)
	}
	return names
}

// Table is a table or view an export can be limited to. Name is in the form
// the engine's dump tool accepts.
type Table struct {
	Name string `json:"name"`
	View bool   `json:"view,omitempty"`
}

type ImportOptions struct {
	Format             string `json:"format"`
	CreateDatabase     bool   `json:"createDatabase"`
	DropExisting       bool   `json:"dropExisting"`
	DisableForeignKeys bool   `json:"disableForeignKeys"`
	Confirm            bool   `json:"confirm"`
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
	ListDatabases(ctx context.Context, cfg Connection) ([]string, error)
	ListTables(ctx context.Context, cfg Connection) ([]Table, error)
	Export(ctx context.Context, cfg Connection, opts ExportOptions, out io.Writer) error
	Import(ctx context.Context, cfg Connection, opts ImportOptions, in io.Reader) error
}
