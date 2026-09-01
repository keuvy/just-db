package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"just-db/internal/appmeta"
	"just-db/internal/engine"
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
		return fmt.Errorf("usage: %s <serve|tools|version>", appmeta.Name)
	}
	switch args[0] {
	case "serve":
		return cmdServe(args[1:])
	case "tools":
		return cmdTools()
	case "version", "-version", "--version":
		fmt.Printf("%s %s\n", appmeta.Name, appmeta.Version)
		return nil
	default:
		return fmt.Errorf("unknown command %q (want serve, tools, or version)", args[0])
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
