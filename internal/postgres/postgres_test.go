package postgres

import (
	"strings"
	"testing"

	"just-db/internal/engine"
)

func TestDumpArgsNeverIncludePassword(t *testing.T) {
	cfg := engine.Connection{
		Host:     "db.internal",
		Port:     5432,
		User:     "app",
		Password: "s3cret-pass",
		Database: "appdb",
		SSLMode:  "require",
	}.Normalized(5432)
	args, err := dumpArgs(cfg, engine.FormatCustom, dumpRun{tables: []string{"public.items"}, clean: true})
	if err != nil {
		t.Fatal(err)
	}
	if ContainsPassword(args, cfg.Password) {
		t.Fatalf("password leaked into argv: %q", args)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--format=custom") || !strings.Contains(joined, "--table") {
		t.Fatalf("unexpected args: %q", args)
	}
	env := clientEnv(cfg)
	if env["PGPASSWORD"] != cfg.Password {
		t.Fatal("expected PGPASSWORD in child env")
	}
	if env["PGSSLMODE"] != "require" {
		t.Fatalf("ssl: %q", env["PGSSLMODE"])
	}
}

func TestRestoreArgsStdinAndClean(t *testing.T) {
	cfg := engine.Connection{Host: "127.0.0.1", Port: 5432, User: "u", Database: "d"}.Normalized(5432)
	args := restoreArgs(cfg, true, "/tmp/app.dump")
	if ContainsPassword(args, "anything") {
		t.Fatal("empty password check failed unexpectedly")
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--clean") || !strings.HasSuffix(joined, "/tmp/app.dump") {
		t.Fatalf("unexpected restore args: %q", args)
	}
}

func TestCatalogDB(t *testing.T) {
	if catalogDB(engine.Connection{}) != "postgres" {
		t.Fatal("empty connection should use postgres")
	}
	if catalogDB(engine.Connection{Database: "shop"}) != "shop" {
		t.Fatal("explicit database should win")
	}
}

func TestValidate(t *testing.T) {
	if err := (engine.Connection{User: "u"}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestImportEnvDisableForeignKeys(t *testing.T) {
	cfg := engine.Connection{Host: "127.0.0.1", Port: 5432, User: "u", Database: "d"}.Normalized(5432)
	if got := importEnv(cfg, false)["PGOPTIONS"]; got != "-c statement_timeout=0" {
		t.Fatalf("unexpected PGOPTIONS: %q", got)
	}
	if got := importEnv(cfg, true)["PGOPTIONS"]; got != "-c statement_timeout=0 -c session_replication_role=replica" {
		t.Fatalf("unexpected PGOPTIONS: %q", got)
	}
}

func TestDumpArgsContent(t *testing.T) {
	cfg := engine.Connection{Host: "127.0.0.1", Port: 5432, User: "u", Database: "d"}.Normalized(5432)
	cases := []struct {
		format           string
		content          string
		include, exclude []string
	}{
		{engine.FormatSQL, "all", []string{"--clean", "--if-exists"}, []string{"--schema-only", "--data-only"}},
		{engine.FormatSQL, "schema", []string{"--clean", "--schema-only"}, []string{"--data-only"}},
		{engine.FormatSQL, "data", []string{"--data-only"}, []string{"--clean", "--if-exists", "--schema-only"}},
		{engine.FormatCustom, "data", []string{"--format=custom", "--data-only"}, []string{"--clean", "--schema-only"}},
	}
	for _, c := range cases {
		sel, err := engine.ExportOptions{Content: c.content}.Selection()
		if err != nil {
			t.Fatal(err)
		}
		runs, err := dumpRuns(sel, c.format)
		if err != nil || len(runs) != 1 {
			t.Fatalf("%+v: %d runs, %v", c, len(runs), err)
		}
		args, err := dumpArgs(cfg, c.format, runs[0])
		if err != nil {
			t.Fatal(err)
		}
		has := map[string]bool{}
		for _, arg := range args {
			has[arg] = true
		}
		for _, want := range c.include {
			if !has[want] {
				t.Errorf("%+v: missing %s in %q", c, want, args)
			}
		}
		for _, bad := range c.exclude {
			if has[bad] {
				t.Errorf("%+v: unexpected %s in %q", c, bad, args)
			}
		}
	}
}

func TestDumpRunsPerTable(t *testing.T) {
	cfg := engine.Connection{Host: "127.0.0.1", Port: 5432, User: "u", Database: "d"}.Normalized(5432)
	plan := func(format string, tables ...engine.TableContent) []string {
		t.Helper()
		sel, err := engine.ExportOptions{PerTable: tables}.Selection()
		if err != nil {
			t.Fatal(err)
		}
		runs, err := dumpRuns(sel, format)
		if err != nil {
			return []string{err.Error()}
		}
		out := make([]string, 0, len(runs))
		for _, run := range runs {
			args, err := dumpArgs(cfg, format, run)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, strings.Join(args[len(clientArgs(cfg))+2:], " "))
		}
		return out
	}
	both := engine.TableContent{Name: "a", Schema: true, Data: true}
	shape := engine.TableContent{Name: "b", Schema: true}
	rows := engine.TableContent{Name: "c", Data: true}
	cases := []struct {
		format string
		tables []engine.TableContent
		want   []string
	}{
		{engine.FormatCustom, []engine.TableContent{both, shape}, []string{"--format=custom --table a --table b --exclude-table-data b"}},
		{engine.FormatSQL, []engine.TableContent{shape}, []string{"--format=plain --clean --if-exists --schema-only --table b"}},
		{engine.FormatCustom, []engine.TableContent{rows}, []string{"--format=custom --data-only --table c"}},
		{engine.FormatSQL, []engine.TableContent{both, shape, rows}, []string{
			"--format=plain --clean --if-exists --section=pre-data --table a --table b",
			"--format=plain --section=data --table a --table c",
			"--format=plain --section=post-data --table a --table b",
		}},
		{engine.FormatCustom, []engine.TableContent{shape, rows}, []string{"invalid export options: a PostgreSQL custom archive cannot mix data-only tables with tables that include structure; use SQL format"}},
	}
	for _, c := range cases {
		if got := plan(c.format, c.tables...); strings.Join(got, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("%s %+v:\n got %q\nwant %q", c.format, c.tables, got, c.want)
		}
	}
}
