package server

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExportFailureLog(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)
	srv := New(Options{DataDir: t.TempDir()})
	request := httptest.NewRequest(http.MethodPost, "/api/export", strings.NewReader(`{"engine":"test-secret","connection":{"password":"test-secret"}}`))
	response := httptest.NewRecorder()
	srv.Handler().ServeHTTP(response, request)
	for _, want := range []string{"export started", "export failed", "status=404", "duration=", "[REDACTED]"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("missing %q in log %q", want, output.String())
		}
	}
	if strings.Contains(output.String(), "test-secret") {
		t.Fatal("password leaked")
	}
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d", response.Code)
	}
}
