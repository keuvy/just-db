package mysql

import (
	"errors"
	"strings"
	"testing"

	"just-db/internal/engine"
)

func TestDumpArgsNeverIncludePassword(t *testing.T) {
	cfg := engine.Connection{
		Host:     "db.internal",
		Port:     3306,
		User:     "app",
		Password: "s3cret-pass",
		Database: "appdb",
		SSLMode:  "disable",
	}.Normalized(3306)
	tool := engine.Tool{Name: "mysqldump", Path: "/usr/bin/mysqldump"}
	args := dumpArgs("/tmp/justdb-my.cnf", cfg, tool, []string{"items"}, true, true)
	if ContainsPassword(args, cfg.Password) {
		t.Fatalf("password leaked into argv: %q", args)
	}
	if !strings.HasPrefix(args[0], "--defaults-file=") {
		t.Fatalf("defaults file must be first: %q", args)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--ssl-mode=DISABLED") || !strings.Contains(joined, "--get-server-public-key") || !strings.Contains(joined, "appdb") {
		t.Fatalf("unexpected args: %q", args)
	}
	if strings.Contains(joined, "-p"+cfg.Password) || strings.Contains(joined, "--password=") {
		t.Fatal("password flag present")
	}
}

func TestMariaSSLArgs(t *testing.T) {
	tool := engine.Tool{Name: "mariadb-dump", Path: "/usr/bin/mariadb-dump"}
	args := sslArgs(tool, "disable")
	if len(args) != 1 || args[0] != "--skip-ssl" {
		t.Fatalf("%q", args)
	}
}

func TestClientArgsOmitsEmptyDatabase(t *testing.T) {
	cfg := engine.Connection{Host: "127.0.0.1", Port: 3306, User: "root"}.Normalized(3306)
	tool := engine.Tool{Name: "mysql", Path: "/usr/bin/mysql"}
	joined := strings.Join(clientArgs("/tmp/justdb-my.cnf", cfg, tool), " ")
	if strings.Contains(joined, "--database=") {
		t.Fatalf("did not expect database arg: %q", joined)
	}
}

func TestMysqlFormatCoercesCustom(t *testing.T) {
	got, err := mysqlFormat(engine.FormatCustom)
	if err != nil || got != engine.FormatSQL {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestImportArgsDisableForeignKeys(t *testing.T) {
	cfg := engine.Connection{Host: "127.0.0.1", Port: 3306, User: "root", Database: "app"}.Normalized(3306)
	tool := engine.Tool{Name: "mysql", Path: "/usr/bin/mysql"}
	if joined := strings.Join(importArgs("/tmp/justdb-my.cnf", cfg, tool, false), " "); strings.Contains(joined, "FOREIGN_KEY_CHECKS") {
		t.Fatalf("did not expect foreign key override: %q", joined)
	}
	args := importArgs("/tmp/justdb-my.cnf", cfg, tool, true)
	if args[len(args)-1] != "--init-command=SET FOREIGN_KEY_CHECKS=0" {
		t.Fatalf("expected foreign key override: %q", args)
	}
}

func TestFlavorOf(t *testing.T) {
	cases := []struct {
		name, path, version string
		maria               bool
		release             [3]int
	}{
		{"mysqldump", "/usr/local/mysql/bin/mysqldump", "mysqldump  Ver 8.4.6 for macos15 on x86_64 (MySQL Community Server - GPL)", false, [3]int{8, 4, 6}},
		{"mysqldump", "/usr/local/bin/mysqldump", "mysqldump  Ver 10.13 Distrib 5.7.44, for osx10.19 (x86_64)", false, [3]int{5, 7, 44}},
		{"mysqldump", "/usr/bin/mysqldump", "mysqldump  Ver 8.0.36-28 for Linux on x86_64 (Percona Server (GPL), Release 28, Revision 47601f19)", false, [3]int{8, 0, 36}},
		{"mysqldump", "/usr/bin/mysqldump", "mysqldump  Ver 10.19 Distrib 10.11.6-MariaDB, for debian-linux-gnu (x86_64)", true, [3]int{}},
		{"mariadb-dump", "/opt/homebrew/bin/mariadb-dump", "mariadb-dump from 11.4.2-MariaDB, client 10.19 for osx10.19 (arm64)", true, [3]int{}},
		{"mysqldump", "/usr/bin/mysqldump", "", false, [3]int{}},
	}
	for _, c := range cases {
		got := flavorOf(engine.Tool{Name: c.name, Path: c.path, Version: c.version})
		if got.maria != c.maria || got.release != c.release {
			t.Errorf("%q: got %+v", c.version, got)
		}
	}
}

func TestDumpArgsMatchClientRelease(t *testing.T) {
	cfg := engine.Connection{Host: "db", Port: 3306, User: "app", Database: "appdb", SSLMode: "disable"}.Normalized(3306)
	cases := []struct {
		version string
		include []string
		exclude []string
	}{
		{
			version: "mysqldump  Ver 8.4.6 for macos15 on x86_64 (MySQL Community Server - GPL)",
			include: []string{"--set-gtid-purged=OFF", "--column-statistics=0", "--no-tablespaces", "--get-server-public-key", "--ssl-mode=DISABLED"},
		},
		{
			version: "mysqldump  Ver 10.13 Distrib 5.7.44, for Linux (x86_64)",
			include: []string{"--set-gtid-purged=OFF", "--no-tablespaces", "--get-server-public-key", "--ssl-mode=DISABLED"},
			exclude: []string{"--column-statistics=0"},
		},
		{
			version: "mysqldump  Ver 10.13 Distrib 5.7.10, for Linux (x86_64)",
			include: []string{"--set-gtid-purged=OFF", "--no-tablespaces", "--skip-ssl"},
			exclude: []string{"--column-statistics=0", "--get-server-public-key", "--ssl-mode=DISABLED"},
		},
		{
			version: "mysqldump  Ver 10.19 Distrib 10.11.6-MariaDB, for debian-linux-gnu (x86_64)",
			include: []string{"--skip-ssl"},
			exclude: []string{"--set-gtid-purged=OFF", "--column-statistics=0", "--get-server-public-key", "--ssl-mode=DISABLED"},
		},
	}
	for _, c := range cases {
		tool := engine.Tool{Name: "mysqldump", Path: "/usr/local/bin/mysqldump", Version: c.version}
		args := dumpArgs("/tmp/justdb-my.cnf", cfg, tool, nil, true, true)
		has := map[string]bool{}
		for _, arg := range args {
			has[arg] = true
		}
		for _, want := range c.include {
			if !has[want] {
				t.Errorf("%q: missing %s in %q", c.version, want, args)
			}
		}
		for _, bad := range c.exclude {
			if has[bad] {
				t.Errorf("%q: unexpected %s in %q", c.version, bad, args)
			}
		}
	}
}

func TestDumpArgsContent(t *testing.T) {
	cfg := engine.Connection{Host: "db", Port: 3306, User: "app", Database: "appdb"}.Normalized(3306)
	tool := engine.Tool{Name: "mysqldump", Path: "/usr/bin/mysqldump"}
	cases := []struct {
		schema, data     bool
		include, exclude []string
	}{
		{true, true, []string{"--triggers"}, []string{"--no-data", "--no-create-info", "--skip-triggers"}},
		{true, false, []string{"--no-data", "--triggers"}, []string{"--no-create-info", "--skip-triggers"}},
		{false, true, []string{"--no-create-info", "--skip-triggers"}, []string{"--no-data", "--triggers"}},
	}
	for _, c := range cases {
		args := dumpArgs("/tmp/justdb-my.cnf", cfg, tool, []string{" items ", "", "orders"}, c.schema, c.data)
		joined := " " + strings.Join(args, " ") + " "
		for _, want := range c.include {
			if !strings.Contains(joined, " "+want+" ") {
				t.Errorf("schema=%v data=%v: missing %s in %q", c.schema, c.data, want, args)
			}
		}
		for _, bad := range c.exclude {
			if strings.Contains(joined, " "+bad+" ") {
				t.Errorf("schema=%v data=%v: unexpected %s in %q", c.schema, c.data, bad, args)
			}
		}
		if tail := args[len(args)-3:]; tail[0] != "appdb" || tail[1] != "items" || tail[2] != "orders" {
			t.Errorf("database and tables must end the command: %q", args)
		}
	}
}

func TestCheckDumpNames(t *testing.T) {
	if err := checkDumpNames("appdb", []string{"items", "order-lines"}); err != nil {
		t.Fatal(err)
	}
	for _, tables := range [][]string{{"--result-file=/tmp/x"}, {" -d"}} {
		if err := checkDumpNames("appdb", tables); !errors.Is(err, engine.ErrInvalidExport) {
			t.Errorf("%q: expected ErrInvalidExport, got %v", tables, err)
		}
	}
	if err := checkDumpNames("--all-databases", nil); !errors.Is(err, engine.ErrInvalidExport) {
		t.Errorf("database: expected ErrInvalidExport, got %v", err)
	}
}

func TestParseTables(t *testing.T) {
	got := parseTables("BASE TABLE\titems\nVIEW\tactive items\r\nSYSTEM VIEW\tx\n\nbad line\n")
	want := []engine.Table{{Name: "items"}, {Name: "active items", View: true}, {Name: "x", View: true}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %+v", got)
		}
	}
}

func TestDumpRuns(t *testing.T) {
	sel, err := engine.ExportOptions{Content: "data"}.Selection()
	if err != nil {
		t.Fatal(err)
	}
	if runs := dumpRuns(sel); len(runs) != 1 || runs[0].tables != nil || runs[0].schema || !runs[0].data {
		t.Fatalf("full data-only export: %+v", runs)
	}
	sel, err = engine.ExportOptions{PerTable: []engine.TableContent{
		{Name: "rows", Data: true},
		{Name: "both", Schema: true, Data: true},
		{Name: "view", Schema: true},
		{Name: "both2", Schema: true, Data: true},
	}}.Selection()
	if err != nil {
		t.Fatal(err)
	}
	runs := dumpRuns(sel)
	want := []dumpRun{{[]string{"both", "both2"}, true, true}, {[]string{"view"}, true, false}, {[]string{"rows"}, false, true}}
	if len(runs) != len(want) {
		t.Fatalf("runs: %+v", runs)
	}
	for i := range want {
		if strings.Join(runs[i].tables, ",") != strings.Join(want[i].tables, ",") || runs[i].schema != want[i].schema || runs[i].data != want[i].data {
			t.Fatalf("run %d: got %+v want %+v", i, runs[i], want[i])
		}
	}
}
