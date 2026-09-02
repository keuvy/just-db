package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"just-db/internal/appmeta"
	"just-db/internal/engine"
	"just-db/internal/job"
	"just-db/internal/registry"
	"just-db/server"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", appmeta.Name, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s <serve|tools|test|export|import|version>", appmeta.Name)
	}
	switch args[0] {
	case "serve":
		return cmdServe(args[1:])
	case "tools":
		return cmdTools()
	case "test":
		return cmdTest(args[1:])
	case "export":
		return cmdExport(args[1:])
	case "import":
		return cmdImport(args[1:])
	case "version", "-version", "--version":
		fmt.Printf("%s %s\n", appmeta.Name, appmeta.Version)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	listen := fs.String("listen", envOr("JUSTDB_LISTEN", "0.0.0.0:8080"), "listen address")
	data := fs.String("data", envOr("JUSTDB_DATA", "./data"), "data directory for dumps and profiles")
	ui := fs.String("ui", envOr("JUSTDB_UI_DIR", "frontend/dist"), "directory of built frontend assets")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return server.New(server.Options{Listen: *listen, DataDir: *data, UIDir: *ui}).ListenAndServe()
}

func cmdTools() error {
	infos, err := registry.New().Infos(context.Background())
	if err != nil {
		return err
	}
	for _, info := range infos {
		status := "missing tools"
		if info.Ready {
			status = "ready"
		}
		fmt.Printf("%s (%s) default port %d — %s\n", info.DisplayName, info.Name, info.DefaultPort, status)
		printTool("  dump   ", info.Tools.Dump)
		printTool("  restore", info.Tools.Restore)
		printTool("  client ", info.Tools.Client)
		fmt.Println()
	}
	return nil
}

func cmdTest(args []string) error {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	c := registerConnFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	eng, err := registry.New().Get(c.engine())
	if err != nil {
		return err
	}
	if err := eng.TestConnection(context.Background(), c.connection()); err != nil {
		return err
	}
	fmt.Println("ok")
	return nil
}

func cmdExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	c := registerConnFlags(fs)
	format := fs.String("format", "", "sql or custom")
	outPath := fs.String("out", "", "output dump path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *outPath == "" {
		return fmt.Errorf("-out is required")
	}
	eng, err := registry.New().Get(c.engine())
	if err != nil {
		return err
	}
	result, err := job.ExportToFile(context.Background(), eng, c.connection(), engine.ExportOptions{Format: *format}, *outPath)
	if err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d bytes, %s)\n", result.Path, result.Bytes, result.Format)
	return nil
}

func cmdImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	c := registerConnFlags(fs)
	inPath := fs.String("in", "", "dump file")
	format := fs.String("format", "", "sql or custom (inferred from file if empty)")
	drop := fs.Bool("drop", false, "drop existing objects before restore")
	confirm := fs.Bool("confirm", false, "required to run a restore")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *inPath == "" {
		return fmt.Errorf("-in is required")
	}
	eng, err := registry.New().Get(c.engine())
	if err != nil {
		return err
	}
	if err := job.ImportFromFile(context.Background(), eng, c.connection(), engine.ImportOptions{
		Format:       *format,
		DropExisting: *drop,
		Confirm:      *confirm,
	}, *inPath); err != nil {
		return err
	}
	fmt.Println("ok")
	return nil
}

type connFlags struct {
	engineName *string
	host       *string
	port       *int
	user       *string
	password   *string
	database   *string
	sslmode    *string
}

func registerConnFlags(fs *flag.FlagSet) connFlags {
	return connFlags{
		engineName: fs.String("engine", "postgres", "postgres or mysql"),
		host:       fs.String("host", envOr("JUSTDB_HOST", "127.0.0.1"), "database host"),
		port:       fs.Int("port", atoiDefault(os.Getenv("JUSTDB_PORT"), 0), "database port (engine default if 0)"),
		user:       fs.String("user", envOr("JUSTDB_USER", ""), "database user"),
		password:   fs.String("password", "", "database password (or JUSTDB_PASSWORD / PGPASSWORD / MYSQL_PWD)"),
		database:   fs.String("database", envOr("JUSTDB_DATABASE", ""), "database name"),
		sslmode:    fs.String("sslmode", envOr("JUSTDB_SSLMODE", "prefer"), "sslmode"),
	}
}

func (c connFlags) engine() engine.Name { return engine.Name(*c.engineName) }

func (c connFlags) connection() engine.Connection {
	cfg := engine.Connection{
		Host:     *c.host,
		Port:     *c.port,
		User:     *c.user,
		Password: *c.password,
		Database: *c.database,
		SSLMode:  *c.sslmode,
	}
	if cfg.Password == "" {
		for _, key := range []string{"JUSTDB_PASSWORD", "PGPASSWORD", "MYSQL_PWD"} {
			if v := os.Getenv(key); v != "" {
				cfg.Password = v
				break
			}
		}
	}
	return cfg
}

func printTool(label string, tool engine.Tool) {
	if !tool.Found {
		fmt.Printf("%s  missing (%s)\n", label, tool.Name)
		return
	}
	fmt.Printf("%s  %s\n", label, tool.Path)
	if tool.Version != "" {
		fmt.Printf("           %s\n", tool.Version)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func atoiDefault(v string, fallback int) int {
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
