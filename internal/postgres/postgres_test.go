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
	args, err := dumpArgs(cfg, []string{"public.items"}, engine.FormatCustom)
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

func TestValidate(t *testing.T) {
	err := engine.Connection{User: "u"}.Validate()
	if err == nil {
		t.Fatal("expected database required")
	}
}
