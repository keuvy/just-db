package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"just-db/internal/engine"
	"just-db/internal/job"
	"just-db/internal/profile"
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

func (s *Server) handleListDatabases(w http.ResponseWriter, r *http.Request) {
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
	names, err := eng.ListDatabases(r.Context(), req.Connection.toEngine())
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"databases": names})
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
	format := engine.CoerceFormat(eng.Name(), engine.InferFormat(req.FileName, req.Format))
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
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media == "multipart/form-data" {
		s.handleImportUpload(w, r)
		return
	}
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

func (s *Server) handleImportUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	var conn apiConnection
	if raw := r.FormValue("connection"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &conn); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("connection: %w", err))
			return
		}
	}
	eng, err := s.reg.Get(engine.Name(r.FormValue("engine")))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("file is required"))
		return
	}
	defer file.Close()
	name := filepath.Base(header.Filename)
	if !job.IsDumpName(name) {
		writeError(w, http.StatusBadRequest, job.ErrInvalidDump)
		return
	}
	tmp, err := os.CreateTemp("", "justdb-import-*"+strings.ToLower(filepath.Ext(name)))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, file); err != nil {
		_ = tmp.Close()
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := tmp.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := job.ImportFromFile(r.Context(), eng, conn.toEngine(), engine.ImportOptions{
		Format:       r.FormValue("format"),
		DropExisting: formTrue(r.FormValue("dropExisting")),
		Confirm:      formTrue(r.FormValue("confirm")),
	}, tmpName); err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func formTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (s *Server) handleDumpDownload(w http.ResponseWriter, r *http.Request) {
	path, err := job.ExistingDumpPath(job.BackupsDir(s.opts.DataDir), r.PathValue("name"))
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(path)}))
	http.ServeFile(w, r, path)
}

func (s *Server) handleDumpDelete(w http.ResponseWriter, r *http.Request) {
	if err := job.DeleteDump(job.BackupsDir(s.opts.DataDir), r.PathValue("name")); err != nil {
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

type profileBody struct {
	Engine     engine.Name   `json:"engine"`
	Connection apiConnection `json:"connection"`
}

func (s *Server) handleProfileList(w http.ResponseWriter, r *http.Request) {
	st, err := profile.Open(s.opts.DataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	list, err := st.List()
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": list})
}

func (s *Server) handleProfileGet(w http.ResponseWriter, r *http.Request) {
	st, err := profile.Open(s.opts.DataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	rec, err := st.Get(r.PathValue("name"))
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleProfilePut(w http.ResponseWriter, r *http.Request) {
	var req profileBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := s.reg.Get(req.Engine); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	st, err := profile.Open(s.opts.DataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	rec := profile.Record{
		Name:       r.PathValue("name"),
		Engine:     req.Engine,
		Connection: req.Connection.toEngine(),
	}
	if err := st.Put(rec); err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, rec.Summary())
}

func (s *Server) handleProfileDelete(w http.ResponseWriter, r *http.Request) {
	st, err := profile.Open(s.opts.DataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := st.Delete(r.PathValue("name")); err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
	case errors.Is(err, engine.ErrUnknownEngine),
		errors.Is(err, os.ErrNotExist),
		errors.Is(err, profile.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, engine.ErrInvalidConnection),
		errors.Is(err, job.ErrInvalidDump),
		errors.Is(err, engine.ErrUnsupportedFormat),
		errors.Is(err, engine.ErrImportNotConfirmed),
		errors.Is(err, profile.ErrInvalidName):
		return http.StatusBadRequest
	case errors.Is(err, engine.ErrNotImplemented):
		return http.StatusNotImplemented
	case errors.Is(err, engine.ErrToolsMissing):
		return http.StatusFailedDependency
	case errors.Is(err, profile.ErrDecrypt):
		return http.StatusInternalServerError
	default:
		return http.StatusBadGateway
	}
}
