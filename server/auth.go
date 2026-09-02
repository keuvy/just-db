package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func (s *Server) withAuth(next http.Handler) http.Handler {
	if s.opts.AuthUser == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		user, pass, ok := r.BasicAuth()
		if !ok || !secureEqual(user, s.opts.AuthUser) || !secureEqual(pass, s.opts.AuthPassword) {
			w.Header().Set("WWW-Authenticate", `Basic realm="just-db"`)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isPublicPath(path string) bool {
	return path == "/health" || path == "/api/health"
}

func secureEqual(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func (s *Server) authEnabled() bool {
	return strings.TrimSpace(s.opts.AuthUser) != ""
}
