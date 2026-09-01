package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"just-db/internal/appmeta"
	"just-db/internal/engine"
	"just-db/internal/registry"
)

type Options struct {
	Listen  string
	DataDir string
	UIDir   string
}

type Server struct {
	opts Options
	reg  *registry.Registry
	mux  *http.ServeMux
}

func New(opts Options) *Server {
	if opts.Listen == "" {
		opts.Listen = "0.0.0.0:8080"
	}
	if opts.DataDir == "" {
		opts.DataDir = "./data"
	}
	s := &Server{opts: opts, reg: registry.New(), mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ListenAddr() string { return s.opts.Listen }
func (s *Server) DataDir() string    { return s.opts.DataDir }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/engines", s.handleEngines)
	s.mux.HandleFunc("GET /api/tools", s.handleEngines)
	s.mux.Handle("/", s.uiHandler())
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) ListenAndServe() error {
	if err := os.MkdirAll(s.opts.DataDir, 0o755); err != nil {
		return err
	}
	log.Printf("%s %s listening on %s (data %s)", appmeta.Name, appmeta.Version, s.opts.Listen, s.opts.DataDir)
	srv := &http.Server{
		Addr:              s.opts.Listen,
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"name":    appmeta.Name,
		"version": appmeta.Version,
		"mode":    "server",
		"dataDir": s.opts.DataDir,
	})
}

func (s *Server) handleEngines(w http.ResponseWriter, r *http.Request) {
	infos, err := s.reg.Infos(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"engines": infos})
}

func (s *Server) uiHandler() http.Handler {
	dir := s.opts.UIDir
	if dir == "" {
		dir = os.Getenv("JUSTDB_UI_DIR")
	}
	if dir == "" {
		dir = "frontend/dist"
	}
	index := filepath.Join(dir, "index.html")
	if _, err := os.Stat(index); err == nil {
		return http.FileServer(http.Dir(dir))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/index.html") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(fallbackHTML))
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	msg := err.Error()
	code := status
	if errors.Is(err, engine.ErrUnknownEngine) {
		code = http.StatusNotFound
	}
	writeJSON(w, code, map[string]string{"error": msg})
}

const fallbackHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>just-db</title>
  <style>
    :root { color-scheme: dark; }
    body { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; margin: 2rem; background: #101114; color: #e8e6e3; }
    a { color: #9be7c4; }
  </style>
</head>
<body>
  <h1>just-db</h1>
  <p>Server is up. Build the UI with <code>npm --prefix frontend run build</code> or open <a href="/api/engines">/api/engines</a>.</p>
</body>
</html>
`
