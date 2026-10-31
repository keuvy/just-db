package mysql

import (
	"context"
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
	server := startTestMySQL(t)
	eng := New()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Own source database, so a shared test server can run the other roundtrip too.
	cfg := adminCfg(server)
	cfg.Database = "justdb_parts_src"
	if err := execSQL(ctx, adminCfg(server), "CREATE DATABASE justdb_parts_src"); err != nil {
		t.Fatal(err)
	}
	if err := execSQL(ctx, cfg, `
		CREATE TABLE items (id int PRIMARY KEY, name varchar(64) NOT NULL);
		CREATE TABLE logs (id int PRIMARY KEY);
		CREATE VIEW active_items AS SELECT id FROM items;
		INSERT INTO items VALUES (1, 'alpha'), (2, 'beta');
		INSERT INTO logs VALUES (1);
	`); err != nil {
		t.Fatal(err)
	}

	tables, err := eng.ListTables(ctx, cfg)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	want := []engine.Table{{Name: "active_items", View: true}, {Name: "items"}, {Name: "logs"}}
	if len(tables) != len(want) || tables[0] != want[0] || tables[1] != want[1] || tables[2] != want[2] {
		t.Fatalf("tables: %+v", tables)
	}

	dir := t.TempDir()
	structure, rows := dir+"/structure.sql", dir+"/rows.sql"
	if _, err := job.ExportToFile(ctx, eng, cfg, engine.ExportOptions{Tables: []string{"items"}, Content: engine.ContentSchema}, structure); err != nil {
		t.Fatalf("export structure: %v", err)
	}
	if _, err := job.ExportToFile(ctx, eng, cfg, engine.ExportOptions{Tables: []string{"items"}, Content: engine.ContentData}, rows); err != nil {
		t.Fatalf("export rows: %v", err)
	}
	assertDump(t, structure, []string{"CREATE TABLE `items`"}, []string{"INSERT INTO", "`logs`"})
	assertDump(t, rows, []string{"INSERT INTO `items`"}, []string{"CREATE TABLE", "DROP TABLE", "`logs`"})

	target := adminCfg(server)
	target.Database = "justdb_parts"
	if err := execSQL(ctx, adminCfg(server), "CREATE DATABASE justdb_parts"); err != nil {
		t.Fatal(err)
	}
	if err := job.ImportFromFile(ctx, eng, target, engine.ImportOptions{Format: engine.FormatSQL, Confirm: true}, structure); err != nil {
		t.Fatalf("import structure: %v", err)
	}
	if got := queryValue(ctx, t, target, "SELECT count(*) FROM items"); got != "0" {
		t.Fatalf("structure restore copied rows: %q", got)
	}
	if got := queryValue(ctx, t, target, "SELECT count(*) FROM information_schema.tables WHERE table_schema = DATABASE()"); got != "1" {
		t.Fatalf("expected only items, got %q tables", got)
	}
	if err := job.ImportFromFile(ctx, eng, target, engine.ImportOptions{Format: engine.FormatSQL, Confirm: true}, rows); err != nil {
		t.Fatalf("import rows: %v", err)
	}
	if got := queryValue(ctx, t, target, "SELECT count(*) FROM items"); got != "2" {
		t.Fatalf("rows restore: got %q", got)
	}

	// Mixed choices per table: rows for items, structure only for logs and
	// the view, rows only for extra, which the target already has.
	if err := execSQL(ctx, cfg, `
		CREATE TABLE extra (id int PRIMARY KEY);
		INSERT INTO extra VALUES (1), (2), (3);
		CREATE TRIGGER items_log AFTER INSERT ON items FOR EACH ROW INSERT INTO logs VALUES (NEW.id + 100);
	`); err != nil {
		t.Fatal(err)
	}
	mixed := dir + "/mixed.sql"
	if _, err := job.ExportToFile(ctx, eng, cfg, engine.ExportOptions{PerTable: []engine.TableContent{
		{Name: "items", Schema: true, Data: true},
		{Name: "logs", Schema: true},
		{Name: "active_items", Schema: true},
		{Name: "extra", Data: true},
	}}, mixed); err != nil {
		t.Fatalf("export mixed: %v", err)
	}
	assertDump(t, mixed, []string{"CREATE TABLE `items`", "CREATE TABLE `logs`", "VIEW `active_items`", "INSERT INTO `extra`", "items_log"}, []string{"CREATE TABLE `extra`", "INSERT INTO `logs`"})
	target.Database = "justdb_parts_mixed"
	if err := execSQL(ctx, adminCfg(server), "CREATE DATABASE justdb_parts_mixed; CREATE TABLE justdb_parts_mixed.extra (id int PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := job.ImportFromFile(ctx, eng, target, engine.ImportOptions{Format: engine.FormatSQL, Confirm: true}, mixed); err != nil {
		t.Fatalf("import mixed: %v", err)
	}
	// logs stays empty: the trigger is created after the items rows load.
	got := queryValue(ctx, t, target, "SELECT concat((SELECT count(*) FROM items), ',', (SELECT count(*) FROM logs), ',', (SELECT count(*) FROM extra), ',', (SELECT count(*) FROM active_items), ',', (SELECT count(*) FROM information_schema.triggers WHERE trigger_schema = DATABASE()))")
	if got != "2,0,3,2,1" {
		t.Fatalf("mixed restore items,logs,extra,view,triggers: got %q", got)
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
	cfg, detected, defaults, cleanup, err := eng.prepare(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	var out strings.Builder
	if err := proc.Run(ctx, proc.RunOptions{
		Path:   detected.Client.Path,
		Args:   append(clientArgs(defaults, cfg, detected.Client), "--batch", "--skip-column-names", "--execute", sql),
		Stdout: &out,
	}); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out.String())
}
