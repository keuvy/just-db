package postgres

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"just-db/internal/engine"
	"just-db/internal/proc"
	"just-db/internal/tools"
)

type Engine struct{}

func New() *Engine { return &Engine{} }

func (e *Engine) Name() engine.Name   { return engine.Postgres }
func (e *Engine) DisplayName() string { return "PostgreSQL" }
func (e *Engine) DefaultPort() int    { return 5432 }

func (e *Engine) DetectTools(ctx context.Context) (engine.Tools, error) {
	return engine.Tools{
		Dump:    tools.Detect(ctx, "pg_dump"),
		Restore: tools.Detect(ctx, "pg_restore"),
		Client:  tools.Detect(ctx, "psql"),
	}, nil
}

func (e *Engine) TestConnection(ctx context.Context, cfg engine.Connection) error {
	cfg, tools, err := e.prepare(ctx, cfg)
	if err != nil {
		return err
	}
	if !tools.Client.Found {
		return fmt.Errorf("%w: psql", engine.ErrToolsMissing)
	}
	cfg.Database = catalogDB(cfg)
	var discarded strings.Builder
	return proc.Run(ctx, proc.RunOptions{
		Path:   tools.Client.Path,
		Args:   append(clientArgs(cfg), "--tuples-only", "--no-align", "--command", "SELECT 1"),
		Env:    clientEnv(cfg),
		Stdout: &discarded,
	})
}

func (e *Engine) ListDatabases(ctx context.Context, cfg engine.Connection) ([]string, error) {
	cfg, detected, err := e.prepare(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if !detected.Client.Found {
		return nil, fmt.Errorf("%w: psql", engine.ErrToolsMissing)
	}
	cfg.Database = catalogDB(cfg)
	var out strings.Builder
	if err := proc.Run(ctx, proc.RunOptions{
		Path:   detected.Client.Path,
		Args:   append(clientArgs(cfg), "--tuples-only", "--no-align", "--command", "SELECT datname FROM pg_database WHERE datallowconn ORDER BY 1"),
		Env:    clientEnv(cfg),
		Stdout: &out,
	}); err != nil {
		return nil, err
	}
	return engine.LineNames(out.String()), nil
}

func (e *Engine) Export(ctx context.Context, cfg engine.Connection, opts engine.ExportOptions, out io.Writer) error {
	if err := cfg.RequireDatabase(); err != nil {
		return err
	}
	cfg, detected, err := e.prepare(ctx, cfg)
	if err != nil {
		return err
	}
	if !detected.Dump.Found {
		return fmt.Errorf("%w: pg_dump", engine.ErrToolsMissing)
	}
	format, err := normalizeFormat(opts.Format, engine.FormatCustom)
	if err != nil {
		return err
	}
	args, err := dumpArgs(cfg, opts.Tables, format)
	if err != nil {
		return err
	}
	return proc.Run(ctx, proc.RunOptions{
		Path:   detected.Dump.Path,
		Args:   args,
		Env:    clientEnv(cfg),
		Stdout: out,
	})
}

func (e *Engine) Import(ctx context.Context, cfg engine.Connection, opts engine.ImportOptions, in io.Reader) error {
	if !opts.Confirm {
		return engine.ErrImportNotConfirmed
	}
	if err := cfg.RequireDatabase(); err != nil {
		return err
	}
	cfg, detected, err := e.prepare(ctx, cfg)
	if err != nil {
		return err
	}
	format, err := normalizeFormat(opts.Format, "")
	if err != nil {
		return err
	}
	if format == "" {
		return fmt.Errorf("%w: set format to sql or custom", engine.ErrUnsupportedFormat)
	}
	switch format {
	case engine.FormatSQL:
		if !detected.Client.Found {
			return fmt.Errorf("%w: psql", engine.ErrToolsMissing)
		}
		if opts.DropExisting {
			if err := e.resetPublicSchema(ctx, cfg, detected.Client.Path); err != nil {
				return err
			}
		}
		return proc.Run(ctx, proc.RunOptions{
			Path:  detected.Client.Path,
			Args:  append(clientArgs(cfg), "--set", "ON_ERROR_STOP=1"),
			Env:   clientEnv(cfg),
			Stdin: in,
		})
	case engine.FormatCustom:
		if !detected.Restore.Found {
			return fmt.Errorf("%w: pg_restore", engine.ErrToolsMissing)
		}
		dumpPath, cleanup, err := fileForRestore(in)
		if err != nil {
			return err
		}
		defer cleanup()
		return proc.Run(ctx, proc.RunOptions{
			Path: detected.Restore.Path,
			Args: restoreArgs(cfg, opts.DropExisting, dumpPath),
			Env:  clientEnv(cfg),
		})
	default:
		return fmt.Errorf("%w: %s", engine.ErrUnsupportedFormat, format)
	}
}

func (e *Engine) prepare(ctx context.Context, cfg engine.Connection) (engine.Connection, engine.Tools, error) {
	cfg = cfg.Normalized(e.DefaultPort())
	if err := cfg.Validate(); err != nil {
		return cfg, engine.Tools{}, err
	}
	detected, err := e.DetectTools(ctx)
	if err != nil {
		return cfg, engine.Tools{}, err
	}
	return cfg, detected, nil
}

func (e *Engine) resetPublicSchema(ctx context.Context, cfg engine.Connection, psql string) error {
	sql := strings.Join([]string{
		"DROP SCHEMA IF EXISTS public CASCADE;",
		"CREATE SCHEMA public;",
		"GRANT ALL ON SCHEMA public TO public;",
		"GRANT ALL ON SCHEMA public TO CURRENT_USER;",
	}, " ")
	return proc.Run(ctx, proc.RunOptions{
		Path: psql,
		Args: append(clientArgs(cfg), "--set", "ON_ERROR_STOP=1", "--command", sql),
		Env:  clientEnv(cfg),
	})
}

func catalogDB(cfg engine.Connection) string {
	if db := strings.TrimSpace(cfg.Database); db != "" {
		return db
	}
	return "postgres"
}

func clientArgs(cfg engine.Connection) []string {
	return []string{
		"--host", cfg.Host,
		"--port", strconv.Itoa(cfg.Port),
		"--username", cfg.User,
		"--dbname", cfg.Database,
		"--no-password",
	}
}

func dumpArgs(cfg engine.Connection, tables []string, format string) ([]string, error) {
	args := append(clientArgs(cfg), "--no-owner", "--verbose")
	switch format {
	case engine.FormatCustom:
		args = append(args, "--format=custom")
	case engine.FormatSQL:
		args = append(args, "--format=plain", "--clean", "--if-exists")
	default:
		return nil, fmt.Errorf("%w: %s", engine.ErrUnsupportedFormat, format)
	}
	for _, table := range tables {
		table = strings.TrimSpace(table)
		if table == "" {
			continue
		}
		args = append(args, "--table", table)
	}
	return args, nil
}

func restoreArgs(cfg engine.Connection, dropExisting bool, dumpPath string) []string {
	args := append(clientArgs(cfg),
		"--no-owner",
		"--verbose",
		"--exit-on-error",
		"--format=custom",
	)
	if dropExisting {
		args = append(args, "--clean", "--if-exists")
	}
	args = append(args, dumpPath)
	return args
}

func fileForRestore(in io.Reader) (string, func(), error) {
	if file, ok := in.(*os.File); ok {
		info, err := file.Stat()
		if err == nil && info.Mode().IsRegular() {
			return file.Name(), func() {}, nil
		}
	}
	tmp, err := os.CreateTemp("", "justdb-*.dump")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.Remove(tmp.Name()) }
	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		cleanup()
		return "", nil, err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return tmp.Name(), cleanup, nil
}

func clientEnv(cfg engine.Connection) map[string]string {
	env := map[string]string{
		"PGSSLMODE":         cfg.SSLMode,
		"PGCONNECT_TIMEOUT": "10",
		"PGCLIENTENCODING":  "UTF8",
		"PGOPTIONS":         "-c statement_timeout=0",
	}
	if cfg.Password != "" {
		env["PGPASSWORD"] = cfg.Password
	}
	return env
}

func normalizeFormat(format, fallback string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = fallback
	}
	switch format {
	case "", engine.FormatSQL, engine.FormatCustom:
		return format, nil
	default:
		return "", fmt.Errorf("%w: %s", engine.ErrUnsupportedFormat, format)
	}
}

// ContainsPassword reports whether any argument looks like a plaintext password.
// Used in tests to lock the "never put secrets on argv" rule.
func ContainsPassword(args []string, password string) bool {
	if password == "" {
		return false
	}
	for _, arg := range args {
		if arg == password {
			return true
		}
	}
	return false
}
