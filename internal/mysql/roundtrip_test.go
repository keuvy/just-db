package mysql

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

func TestRoundTripSQL(t *testing.T) {
	cfg := startTestMySQL(t)
	eng := New()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := eng.TestConnection(ctx, cfg); err != nil {
		t.Fatalf("test connection: %v", err)
	}

	if err := execSQL(ctx, cfg, `
		CREATE TABLE items (id int PRIMARY KEY, name varchar(64) NOT NULL);
		INSERT INTO items (id, name) VALUES (1, 'alpha'), (2, 'beta');
	`); err != nil {
		t.Fatal(err)
	}

	path := t.TempDir() + "/items.sql"
	if _, err := job.ExportToFile(ctx, eng, cfg, engine.ExportOptions{Format: engine.FormatSQL}, path); err != nil {
		t.Fatalf("export: %v", err)
	}

	if err := eng.Import(ctx, cfg, engine.ImportOptions{Format: engine.FormatSQL, Confirm: false}, nil); err != engine.ErrImportNotConfirmed {
		t.Fatalf("expected confirm error, got %v", err)
	}

	restore := cfg
	restore.User = envOr("JUSTDB_TEST_MYSQL_ROOT", "root")
	restore.Database = "justdb_restore"
	if err := execSQL(ctx, adminCfg(cfg), "CREATE DATABASE justdb_restore CHARACTER SET utf8mb4"); err != nil {
		t.Fatal(err)
	}
	if err := job.ImportFromFile(ctx, eng, restore, engine.ImportOptions{Format: engine.FormatSQL, Confirm: true}, path); err != nil {
		t.Fatalf("import new db: %v", err)
	}

	if err := job.ImportFromFile(ctx, eng, cfg, engine.ImportOptions{
		Format:       engine.FormatSQL,
		DropExisting: true,
		Confirm:      true,
	}, path); err != nil {
		t.Fatalf("import drop-existing: %v", err)
	}

	for _, target := range []engine.Connection{cfg, restore} {
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
	admin.User = envOr("JUSTDB_TEST_MYSQL_ROOT", "root")
	admin.Database = "mysql"
	return admin
}

func execSQL(ctx context.Context, cfg engine.Connection, sql string) error {
	eng := New()
	cfg, detected, defaults, cleanup, err := eng.prepare(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	if !detected.Client.Found {
		return fmt.Errorf("mysql client not found")
	}
	return proc.Run(ctx, proc.RunOptions{
		Path: detected.Client.Path,
		Args: append(clientArgs(defaults, cfg, detected.Client), "--execute", sql),
	})
}

func queryCount(ctx context.Context, cfg engine.Connection) (string, error) {
	eng := New()
	cfg, detected, defaults, cleanup, err := eng.prepare(ctx, cfg)
	if err != nil {
		return "", err
	}
	defer cleanup()
	var out strings.Builder
	if err := proc.Run(ctx, proc.RunOptions{
		Path:   detected.Client.Path,
		Args:   append(clientArgs(defaults, cfg, detected.Client), "--batch", "--skip-column-names", "--execute", "SELECT count(*) FROM items"),
		Stdout: &out,
	}); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func startTestMySQL(t *testing.T) engine.Connection {
	t.Helper()
	if os.Getenv("JUSTDB_TEST_MYSQLHOST") != "" {
		port, _ := strconv.Atoi(envOr("JUSTDB_TEST_MYSQLPORT", "3306"))
		return engine.Connection{
			Host:     os.Getenv("JUSTDB_TEST_MYSQLHOST"),
			Port:     port,
			User:     envOr("JUSTDB_TEST_MYSQLUSER", "justdb"),
			Password: envOr("JUSTDB_TEST_MYSQLPASSWORD", "justdb"),
			Database: envOr("JUSTDB_TEST_MYSQLDATABASE", "justdb"),
			SSLMode:  "disable",
		}
	}

	runtime, err := containerRuntime()
	if err != nil {
		t.Skip("podman/docker not available and JUSTDB_TEST_MYSQLHOST unset")
	}

	port := freePort(t)
	name := fmt.Sprintf("justdb-my-%d", port)
	image := envOr("JUSTDB_TEST_MYSQLIMAGE", "docker.io/library/mysql:8.4")
	run := exec.Command(runtime, "run", "--rm", "-d",
		"--name", name,
		"-e", "MYSQL_ROOT_PASSWORD=justdb",
		"-e", "MYSQL_USER=justdb",
		"-e", "MYSQL_PASSWORD=justdb",
		"-e", "MYSQL_DATABASE=justdb",
		"-p", fmt.Sprintf("127.0.0.1:%d:3306", port),
		image,
	)
	out, err := run.CombinedOutput()
	if err != nil {
		t.Skipf("could not start mysql container: %v\n%s", err, out)
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
	var lastErr error
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := eng.TestConnection(ctx, cfg)
		cancel()
		if err == nil {
			return cfg
		}
		lastErr = err
		time.Sleep(800 * time.Millisecond)
	}
	t.Fatalf("mysql container did not become ready on port %d: %v", port, lastErr)
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
