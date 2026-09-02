package mysql

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"just-db/internal/engine"
	"just-db/internal/proc"
	"just-db/internal/tools"
)

type Engine struct{}

func New() *Engine { return &Engine{} }

func (e *Engine) Name() engine.Name   { return engine.MySQL }
func (e *Engine) DisplayName() string { return "MySQL / MariaDB" }
func (e *Engine) DefaultPort() int    { return 3306 }

func (e *Engine) DetectTools(ctx context.Context) (engine.Tools, error) {
	client := tools.Detect(ctx, "mysql", "mariadb")
	return engine.Tools{
		Dump:    tools.Detect(ctx, "mysqldump", "mariadb-dump"),
		Restore: client,
		Client:  client,
	}, nil
}

func (e *Engine) TestConnection(ctx context.Context, cfg engine.Connection) error {
	cfg, detected, defaults, cleanup, err := e.prepare(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	if !detected.Client.Found {
		return fmt.Errorf("%w: mysql/mariadb", engine.ErrToolsMissing)
	}
	var discarded strings.Builder
	return proc.Run(ctx, proc.RunOptions{
		Path:   detected.Client.Path,
		Args:   append(clientArgs(defaults, cfg, detected.Client), "--batch", "--skip-column-names", "--execute", "SELECT 1"),
		Stdout: &discarded,
	})
}

func (e *Engine) Export(ctx context.Context, cfg engine.Connection, opts engine.ExportOptions, out io.Writer) error {
	cfg, detected, defaults, cleanup, err := e.prepare(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	if !detected.Dump.Found {
		return fmt.Errorf("%w: mysqldump/mariadb-dump", engine.ErrToolsMissing)
	}
	if _, err := mysqlFormat(opts.Format); err != nil {
		return err
	}
	return proc.Run(ctx, proc.RunOptions{
		Path:   detected.Dump.Path,
		Args:   dumpArgs(defaults, cfg, detected.Dump, opts.Tables),
		Stdout: out,
	})
}

func (e *Engine) Import(ctx context.Context, cfg engine.Connection, opts engine.ImportOptions, in io.Reader) error {
	if !opts.Confirm {
		return engine.ErrImportNotConfirmed
	}
	cfg, detected, defaults, cleanup, err := e.prepare(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	if !detected.Client.Found {
		return fmt.Errorf("%w: mysql/mariadb", engine.ErrToolsMissing)
	}
	if _, err := mysqlFormat(opts.Format); err != nil {
		return err
	}
	if opts.DropExisting {
		if err := e.dropTables(ctx, cfg, detected.Client, defaults); err != nil {
			return err
		}
	}
	return proc.Run(ctx, proc.RunOptions{
		Path:  detected.Client.Path,
		Args:  clientArgs(defaults, cfg, detected.Client),
		Stdin: in,
	})
}

func (e *Engine) prepare(ctx context.Context, cfg engine.Connection) (engine.Connection, engine.Tools, string, func(), error) {
	cfg = cfg.Normalized(e.DefaultPort())
	if err := cfg.Validate(); err != nil {
		return cfg, engine.Tools{}, "", func() {}, err
	}
	detected, err := e.DetectTools(ctx)
	if err != nil {
		return cfg, engine.Tools{}, "", func() {}, err
	}
	path, cleanup, err := writeDefaultsFile(cfg)
	if err != nil {
		return cfg, engine.Tools{}, "", func() {}, err
	}
	return cfg, detected, path, cleanup, nil
}

func (e *Engine) dropTables(ctx context.Context, cfg engine.Connection, client engine.Tool, defaults string) error {
	var out strings.Builder
	if err := proc.Run(ctx, proc.RunOptions{
		Path:   client.Path,
		Args:   append(clientArgs(defaults, cfg, client), "--batch", "--skip-column-names", "--execute", "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'"),
		Stdout: &out,
	}); err != nil {
		return err
	}
	tables := strings.Fields(out.String())
	if len(tables) == 0 {
		return nil
	}
	var sql strings.Builder
	sql.WriteString("SET FOREIGN_KEY_CHECKS=0;")
	for _, table := range tables {
		fmt.Fprintf(&sql, " DROP TABLE IF EXISTS %s;", quoteIdent(table))
	}
	sql.WriteString(" SET FOREIGN_KEY_CHECKS=1;")
	return proc.Run(ctx, proc.RunOptions{
		Path: client.Path,
		Args: append(clientArgs(defaults, cfg, client), "--execute", sql.String()),
	})
}

func writeDefaultsFile(cfg engine.Connection) (string, func(), error) {
	file, err := os.CreateTemp("", "justdb-my-*.cnf")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.Remove(file.Name()) }
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, err
	}
	var b strings.Builder
	b.WriteString("[client]\n")
	fmt.Fprintf(&b, "user=%s\n", cnfQuote(cfg.User))
	fmt.Fprintf(&b, "host=%s\n", cnfQuote(cfg.Host))
	fmt.Fprintf(&b, "port=%d\n", cfg.Port)
	if cfg.Password != "" {
		fmt.Fprintf(&b, "password=%s\n", cnfQuote(cfg.Password))
	}
	if _, err := file.WriteString(b.String()); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return file.Name(), cleanup, nil
}

func clientArgs(defaults string, cfg engine.Connection, tool engine.Tool) []string {
	args := []string{
		"--defaults-file=" + defaults,
		"--protocol=TCP",
		"--connect-timeout=10",
	}
	args = append(args, sslArgs(tool, cfg.SSLMode)...)
	if !isMaria(tool) {
		args = append(args, "--get-server-public-key")
	}
	if cfg.Database != "" {
		args = append(args, "--database="+cfg.Database)
	}
	return args
}

func dumpArgs(defaults string, cfg engine.Connection, tool engine.Tool, tables []string) []string {
	args := []string{
		"--defaults-file=" + defaults,
		"--protocol=TCP",
		"--single-transaction",
		"--triggers",
		"--hex-blob",
	}
	args = append(args, sslArgs(tool, cfg.SSLMode)...)
	if !isMaria(tool) {
		args = append(args, "--set-gtid-purged=OFF", "--column-statistics=0", "--no-tablespaces", "--get-server-public-key")
	}
	args = append(args, cfg.Database)
	for _, table := range tables {
		table = strings.TrimSpace(table)
		if table != "" {
			args = append(args, table)
		}
	}
	return args
}

func sslArgs(tool engine.Tool, mode string) []string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if isMaria(tool) {
		switch mode {
		case "disable":
			return []string{"--skip-ssl"}
		case "require":
			return []string{"--ssl"}
		default:
			return nil
		}
	}
	switch mode {
	case "disable":
		return []string{"--ssl-mode=DISABLED"}
	case "require":
		return []string{"--ssl-mode=REQUIRED"}
	default:
		return []string{"--ssl-mode=PREFERRED"}
	}
}

func isMaria(tool engine.Tool) bool {
	blob := strings.ToLower(tool.Name + " " + tool.Path)
	return strings.Contains(blob, "maria")
}

func mysqlFormat(format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "", engine.FormatSQL, engine.FormatCustom:
		// Custom is Postgres-only; MySQL dumps are always SQL.
		return engine.FormatSQL, nil
	default:
		return "", fmt.Errorf("%w: %s", engine.ErrUnsupportedFormat, format)
	}
}

func cnfQuote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func ContainsPassword(args []string, password string) bool {
	if password == "" {
		return false
	}
	for _, arg := range args {
		if arg == password || strings.Contains(arg, password) {
			return true
		}
	}
	return false
}
