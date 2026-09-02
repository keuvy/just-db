package mysql

import (
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
	args := dumpArgs("/tmp/justdb-my.cnf", cfg, tool, []string{"items"})
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

func TestMysqlFormatCoercesCustom(t *testing.T) {
	got, err := mysqlFormat(engine.FormatCustom)
	if err != nil || got != engine.FormatSQL {
		t.Fatalf("got %q %v", got, err)
	}
}
