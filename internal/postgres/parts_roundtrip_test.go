package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"just-db/internal/engine"
	"just-db/internal/job"
	"just-db/internal/proc"
)

// TestRoundTripTablesAndContent exports chosen tables as structure, then as
// rows, and restores both into an empty database.
func TestRoundTripTablesAndContent(t *testing.T) {
	server := startTestPostgres(t)
	eng := New()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Own source database, so a shared test server can run the other roundtrip too.
	cfg := server
	cfg.Database = "justdb_parts_src"
	if err := execSQL(ctx, adminCfg(server), "CREATE DATABASE justdb_parts_src"); err != nil {
		t.Fatal(err)
	}
	if err := execSQL(ctx, cfg, `
		CREATE TABLE items (id int PRIMARY KEY, name text NOT NULL);
		CREATE TABLE logs (id int PRIMARY KEY);
		CREATE TABLE "Audit Log" (id int PRIMARY KEY);
		CREATE VIEW active_items AS SELECT id FROM items;
		INSERT INTO items VALUES (1, 'alpha'), (2, 'beta');
		INSERT INTO "Audit Log" VALUES (1);
	`); err != nil {
		t.Fatal(err)
	}

	tables, err := eng.ListTables(ctx, cfg)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	want := []engine.Table{{Name: `public."Audit Log"`}, {Name: "public.active_items", View: true}, {Name: "public.items"}, {Name: "public.logs"}}
	if len(tables) != len(want) {
		t.Fatalf("tables: %+v", tables)
	}
	for i := range want {
		if tables[i] != want[i] {
			t.Fatalf("tables: %+v", tables)
		}
	}

	dir := t.TempDir()
	structure, rowsSQL, rowsCustom := dir+"/structure.sql", dir+"/rows.sql", dir+"/rows.dump"
	chosen := []string{"public.items", `public."Audit Log"`}
	if _, err := job.ExportToFile(ctx, eng, cfg, engine.ExportOptions{Tables: chosen, Content: engine.ContentSchema}, structure); err != nil {
		t.Fatalf("export structure: %v", err)
	}
	if _, err := job.ExportToFile(ctx, eng, cfg, engine.ExportOptions{Tables: chosen, Content: engine.ContentData}, rowsSQL); err != nil {
		t.Fatalf("export sql rows: %v", err)
	}
	if _, err := job.ExportToFile(ctx, eng, cfg, engine.ExportOptions{Tables: chosen, Content: engine.ContentData}, rowsCustom); err != nil {
		t.Fatalf("export custom rows: %v", err)
	}
	assertDump(t, structure, []string{"CREATE TABLE public.items", `CREATE TABLE public."Audit Log"`}, []string{"COPY ", "logs"})
	assertDump(t, rowsSQL, []string{"COPY public.items"}, []string{"CREATE TABLE", "DROP ", "logs"})

	for _, rows := range []string{rowsSQL, rowsCustom} {
		target := cfg
		target.Database = "justdb_parts_" + strings.TrimPrefix(rows[strings.LastIndex(rows, ".")+1:], ".")
		if err := execSQL(ctx, adminCfg(cfg), "CREATE DATABASE "+target.Database); err != nil {
			t.Fatal(err)
		}
		if err := job.ImportFromFile(ctx, eng, target, engine.ImportOptions{Format: engine.FormatSQL, Confirm: true}, structure); err != nil {
			t.Fatalf("import structure: %v", err)
		}
		if got := queryValue(ctx, t, target, "SELECT count(*) FROM items"); got != "0" {
			t.Fatalf("structure restore copied rows: %q", got)
		}
		if got := queryValue(ctx, t, target, "SELECT string_agg(tablename, ',' ORDER BY tablename) FROM pg_tables WHERE schemaname = 'public'"); got != "Audit Log,items" {
			t.Fatalf("expected only chosen tables, got %q", got)
		}
		if err := job.ImportFromFile(ctx, eng, target, engine.ImportOptions{Confirm: true}, rows); err != nil {
			t.Fatalf("import %s: %v", rows, err)
		}
		if got := queryValue(ctx, t, target, `SELECT (SELECT count(*) FROM items) || ',' || (SELECT count(*) FROM "Audit Log")`); got != "2,1" {
			t.Fatalf("%s: got %q", rows, got)
		}
	}

	// Mixed choices per table: rows for items, structure only for "Audit Log"
	// and the view, rows only for logs, which the target already has.
	if err := execSQL(ctx, cfg, `INSERT INTO logs VALUES (1), (2), (3)`); err != nil {
		t.Fatal(err)
	}
	perTable := []engine.TableContent{
		{Name: "public.items", Schema: true, Data: true},
		{Name: `public."Audit Log"`, Schema: true},
		{Name: "public.active_items", Schema: true},
		{Name: "public.logs", Data: true},
	}
	if _, err := job.ExportToFile(ctx, eng, cfg, engine.ExportOptions{PerTable: perTable}, dir+"/mixed.dump"); !errors.Is(err, engine.ErrInvalidExport) {
		t.Fatalf("custom archive should reject the mix, got %v", err)
	}
	mixed := dir + "/mixed.sql"
	if _, err := job.ExportToFile(ctx, eng, cfg, engine.ExportOptions{PerTable: perTable}, mixed); err != nil {
		t.Fatalf("export mixed: %v", err)
	}
	assertDump(t, mixed, []string{"CREATE TABLE public.items", `CREATE TABLE public."Audit Log"`, "CREATE VIEW public.active_items", "COPY public.items", "COPY public.logs"}, []string{"CREATE TABLE public.logs", `COPY public."Audit Log"`})
	target := cfg
	target.Database = "justdb_parts_mixed"
	if err := execSQL(ctx, adminCfg(server), "CREATE DATABASE justdb_parts_mixed"); err != nil {
		t.Fatal(err)
	}
	if err := execSQL(ctx, target, "CREATE TABLE logs (id int PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := job.ImportFromFile(ctx, eng, target, engine.ImportOptions{Format: engine.FormatSQL, Confirm: true}, mixed); err != nil {
		t.Fatalf("import mixed: %v", err)
	}
	got := queryValue(ctx, t, target, `SELECT (SELECT count(*) FROM items) || ',' || (SELECT count(*) FROM "Audit Log") || ',' || (SELECT count(*) FROM logs) || ',' || (SELECT count(*) FROM active_items)`)
	if got != "2,0,3,2" {
		t.Fatalf("mixed restore items,audit,logs,view: got %q", got)
	}

	// Structure for every table and rows for some fit in one custom archive.
	archive := dir + "/partial.dump"
	if _, err := job.ExportToFile(ctx, eng, cfg, engine.ExportOptions{PerTable: perTable[:3]}, archive); err != nil {
		t.Fatalf("export partial archive: %v", err)
	}
	target.Database = "justdb_parts_archive"
	if err := execSQL(ctx, adminCfg(server), "CREATE DATABASE justdb_parts_archive"); err != nil {
		t.Fatal(err)
	}
	if err := job.ImportFromFile(ctx, eng, target, engine.ImportOptions{Format: engine.FormatCustom, Confirm: true}, archive); err != nil {
		t.Fatalf("import partial archive: %v", err)
	}
	if got := queryValue(ctx, t, target, `SELECT (SELECT count(*) FROM items) || ',' || (SELECT count(*) FROM "Audit Log")`); got != "2,0" {
		t.Fatalf("partial archive items,audit: got %q", got)
	}
}

func assertDump(t *testing.T, path string, include, exclude []string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range include {
		if !strings.Contains(string(raw), want) {
			t.Errorf("%s: missing %q", path, want)
		}
	}
	for _, bad := range exclude {
		if strings.Contains(string(raw), bad) {
			t.Errorf("%s: unexpected %q", path, bad)
		}
	}
}

func queryValue(ctx context.Context, t *testing.T, cfg engine.Connection, sql string) string {
	t.Helper()
	eng := New()
	cfg, detected, err := eng.prepare(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := proc.Run(ctx, proc.RunOptions{
		Path:   detected.Client.Path,
		Args:   append(clientArgs(cfg), "--no-psqlrc", "--tuples-only", "--no-align", "--command", sql),
		Env:    clientEnv(cfg),
		Stdout: &out,
	}); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out.String())
}
