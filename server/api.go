package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"just-db/internal/engine"
	"just-db/internal/job"
)

type apiConnection struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"sslMode"`
}

type testRequest struct {
	Engine     engine.Name   `json:"engine"`
	Connection apiConnection `json:"connection"`
}

type exportRequest struct {
	Engine     engine.Name   `json:"engine"`
	Connection apiConnection `json:"connection"`
	Format     string        `json:"format"`
	Tables     []string      `json:"tables"`
	FileName   string        `json:"fileName"`
}

type importRequest struct {
	Engine       engine.Name   `json:"engine"`
	Connection   apiConnection `json:"connection"`
	Format       string        `json:"format"`
	FileName     string        `json:"fileName"`
	DropExisting bool          `json:"dropExisting"`
	Confirm      bool          `json:"confirm"`
}

func (s *Server) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	var req testRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	eng, err := s.reg.Get(req.Engine)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := eng.TestConnection(r.Context(), req.Connection.toEngine()); err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	var req exportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	eng, err := s.reg.Get(req.Engine)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	format := engine.InferFormat(req.FileName, req.Format)
	if format == "" {
		format = engine.FormatCustom
	}
	name := req.FileName
	if strings.TrimSpace(name) == "" {
		name = job.DefaultFileName(eng.Name(), req.Connection.Database, format)
	}
	path, err := s.dumpPath(name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := job.ExportToFile(r.Context(), eng, req.Connection.toEngine(), engine.ExportOptions{
		Format: format,
		Tables: req.Tables,
	}, path)
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	var req importRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	eng, err := s.reg.Get(req.Engine)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	path, err := s.dumpPath(req.FileName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := job.ImportFromFile(r.Context(), eng, req.Connection.toEngine(), engine.ImportOptions{
		Format:       req.Format,
		DropExisting: req.DropExisting,
		Confirm:      req.Confirm,
	}, path); err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDumps(w http.ResponseWriter, r *http.Request) {
	dumps, err := job.ListDumps(job.BackupsDir(s.opts.DataDir))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dumps": dumps})
}

func (s *Server) dumpPath(name string) (string, error) {
	safe, err := job.SafeFileName(name)
	if err != nil {
		return "", err
	}
	dir, err := filepath.Abs(job.BackupsDir(s.opts.DataDir))
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, safe)
	rel, err := filepath.Rel(dir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("dump path escapes backups directory")
	}
	return path, nil
}

func (c apiConnection) toEngine() engine.Connection {
	return engine.Connection{
		Host:     c.Host,
		Port:     c.Port,
		User:     c.User,
		Password: c.Password,
		Database: c.Database,
		SSLMode:  c.SSLMode,
	}
}

func statusFor(err error) int {
	switch {
	case errors.Is(err, engine.ErrUnknownEngine), errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, engine.ErrInvalidConnection),
		errors.Is(err, engine.ErrUnsupportedFormat),
		errors.Is(err, engine.ErrImportNotConfirmed):
		return http.StatusBadRequest
	case errors.Is(err, engine.ErrNotImplemented):
		return http.StatusNotImplemented
	case errors.Is(err, engine.ErrToolsMissing):
		return http.StatusFailedDependency
	default:
		return http.StatusBadGateway
	}
}
