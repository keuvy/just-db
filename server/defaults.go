package server

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"just-db/internal/engine"
	"just-db/internal/job"
)

type Defaults struct {
	Engine     engine.Name       `json:"engine"`
	Connection engine.Connection `json:"connection"`
	DataDir    string            `json:"dataDir"`
	BackupsDir string            `json:"backupsDir"`
	Auth       bool              `json:"auth"`
}

func DefaultsFromEnv() (engine.Name, engine.Connection) {
	name := engine.Name(strings.ToLower(strings.TrimSpace(os.Getenv("JUSTDB_ENGINE"))))
	if name == "" {
		name = engine.Postgres
	}
	port, _ := strconv.Atoi(os.Getenv("JUSTDB_PORT"))
	password := os.Getenv("JUSTDB_PASSWORD")
	if password == "" {
		password = os.Getenv("PGPASSWORD")
	}
	if password == "" {
		password = os.Getenv("MYSQL_PWD")
	}
	return name, engine.Connection{
		Host:     os.Getenv("JUSTDB_HOST"),
		Port:     port,
		User:     os.Getenv("JUSTDB_USER"),
		Password: password,
		Database: os.Getenv("JUSTDB_DATABASE"),
		SSLMode:  envOrDefault("JUSTDB_SSLMODE", ""),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (s *Server) handleDefaults(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, Defaults{
		Engine:     s.opts.DefaultEngine,
		Connection: s.opts.DefaultConn,
		DataDir:    s.opts.DataDir,
		BackupsDir: job.BackupsDir(s.opts.DataDir),
		Auth:       s.authEnabled(),
	})
}
