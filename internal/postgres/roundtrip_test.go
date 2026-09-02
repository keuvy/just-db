package postgres

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"just-db/internal/engine"
	"just-db/internal/job"
	"just-db/internal/proc"
)

func TestRoundTripCustomAndSQL(t *testing.T) {
	cfg := startTestPostgres(t)
	eng := New()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := eng.TestConnection(ctx, cfg); err != nil {
		t.Fatalf("test connection: %v", err)
	}

	if err := execSQL(ctx, cfg, `
		CREATE TABLE items (id int PRIMARY KEY, name text NOT NULL);
		INSERT INTO items (id, name) VALUES (1, 'alpha'), (2, 'beta');
	`); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	customPath := dir + "/items.dump"
	sqlPath := dir + "/items.sql"

	if _, err := job.ExportToFile(ctx, eng, cfg, engine.ExportOptions{Format: engine.FormatCustom}, customPath); err != nil {
		t.Fatalf("export custom: %v", err)
	}
	if _, err := job.ExportToFile(ctx, eng, cfg, engine.ExportOptions{Format: engine.FormatSQL}, sqlPath); err != nil {
		t.Fatalf("export sql: %v", err)
	}

	if err := eng.Import(ctx, cfg, engine.ImportOptions{Format: engine.FormatCustom, Confirm: false}, nil); err != engine.ErrImportNotConfirmed {
		t.Fatalf("expected confirm error, got %v", err)
	}

	restoreCustom := cfg
	restoreCustom.Database = "justdb_custom"
	if err := execSQL(ctx, adminCfg(cfg), `CREATE DATABASE justdb_custom`); err != nil {
		t.Fatal(err)
	}
	if err := job.ImportFromFile(ctx, eng, restoreCustom, engine.ImportOptions{Format: engine.FormatCustom, Confirm: true}, customPath); err != nil {
		t.Fatalf("import custom: %v", err)
	}

	restoreSQL := cfg
	restoreSQL.Database = "justdb_sql"
	if err := execSQL(ctx, adminCfg(cfg), `CREATE DATABASE justdb_sql`); err != nil {
		t.Fatal(err)
	}
	if err := job.ImportFromFile(ctx, eng, restoreSQL, engine.ImportOptions{Format: engine.FormatSQL, Confirm: true}, sqlPath); err != nil {
		t.Fatalf("import sql: %v", err)
	}

	for _, target := range []engine.Connection{restoreCustom, restoreSQL} {
		count, err := queryCount(ctx, target)
		if err != nil {
			t.Fatal(err)
		}
		if count != "2" {
			t.Fatalf("%s: got count %q", target.Database, count)
		}
	}
}

func adminCfg(cfg engine.Connection) engine.Connection {
	admin := cfg
	admin.Database = "postgres"
	return admin
}

func execSQL(ctx context.Context, cfg engine.Connection, sql string) error {
	eng := New()
	cfg, detected, err := eng.prepare(ctx, cfg)
	if err != nil {
		return err
	}
	if !detected.Client.Found {
		return fmt.Errorf("psql not found")
	}
	return proc.Run(ctx, proc.RunOptions{
		Path: detected.Client.Path,
		Args: append(clientArgs(cfg), "--set", "ON_ERROR_STOP=1", "--command", sql),
		Env:  clientEnv(cfg),
	})
}

func queryCount(ctx context.Context, cfg engine.Connection) (string, error) {
	eng := New()
	cfg, detected, err := eng.prepare(ctx, cfg)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	if err := proc.Run(ctx, proc.RunOptions{
		Path:   detected.Client.Path,
		Args:   append(clientArgs(cfg), "--tuples-only", "--no-align", "--command", "SELECT count(*) FROM items"),
		Env:    clientEnv(cfg),
		Stdout: &out,
	}); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func startTestPostgres(t *testing.T) engine.Connection {
	t.Helper()
	if os.Getenv("JUSTDB_TEST_PGHOST") != "" {
		port, _ := strconv.Atoi(envOr("JUSTDB_TEST_PGPORT", "5432"))
		return engine.Connection{
			Host:     os.Getenv("JUSTDB_TEST_PGHOST"),
			Port:     port,
			User:     envOr("JUSTDB_TEST_PGUSER", "justdb"),
			Password: envOr("JUSTDB_TEST_PGPASSWORD", "justdb"),
			Database: envOr("JUSTDB_TEST_PGDATABASE", "justdb"),
			SSLMode:  "disable",
		}
	}

	runtime, err := containerRuntime()
	if err != nil {
		t.Skip("podman/docker not available and JUSTDB_TEST_PGHOST unset")
	}

	port := freePort(t)
	name := fmt.Sprintf("justdb-pg-%d", port)
	image := envOr("JUSTDB_TEST_PGIMAGE", "docker.io/library/postgres:18-alpine")
	run := exec.Command(runtime, "run", "--rm", "-d",
		"--name", name,
		"-e", "POSTGRES_USER=justdb",
		"-e", "POSTGRES_PASSWORD=justdb",
		"-e", "POSTGRES_DB=justdb",
		"-p", fmt.Sprintf("127.0.0.1:%d:5432", port),
		image,
	)
	out, err := run.CombinedOutput()
	if err != nil {
		t.Skipf("could not start postgres container: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command(runtime, "rm", "-f", name).Run()
	})

	cfg := engine.Connection{
		Host:     "127.0.0.1",
		Port:     port,
		User:     "justdb",
		Password: "justdb",
		Database: "justdb",
		SSLMode:  "disable",
	}
	eng := New()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := eng.TestConnection(ctx, cfg)
		cancel()
		if err == nil {
			return cfg
		}
		time.Sleep(400 * time.Millisecond)
	}
	t.Fatalf("postgres container did not become ready on port %d", port)
	return engine.Connection{}
}

func containerRuntime() (string, error) {
	if path, err := exec.LookPath("podman"); err == nil {
		return path, nil
	}
	return exec.LookPath("docker")
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
