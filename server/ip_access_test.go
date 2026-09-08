package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIPAllowlist(t *testing.T) {
	tests := []struct {
		name, allowed, trusted, peer, forwarded, path string
		want                                          int
	}{
		{"disabled", "", "", "192.0.2.1:1234", "", "/api/health", 200},
		{"direct allowed", "203.0.113.10", "", "203.0.113.10:1234", "", "/api/health", 200},
		{"direct denied", "203.0.113.10", "", "192.0.2.1:1234", "", "/api/health", 403},
		{"spoofed header", "203.0.113.10", "", "192.0.2.1:1234", "203.0.113.10", "/api/health", 403},
		{"proxy allowed", "203.0.113.10", "10.0.0.2", "10.0.0.2:1234", "203.0.113.10", "/api/health", 200},
		{"proxy spoofed prefix", "203.0.113.10", "10.0.0.2", "10.0.0.2:1234", "203.0.113.10, 192.0.2.1", "/api/health", 403},
		{"multiple trusted hops", "203.0.113.10", "10.0.0.0/24", "10.0.0.2:1234", "203.0.113.10, 10.0.0.3", "/api/health", 200},
		{"malformed forwarded", "203.0.113.10", "10.0.0.2", "10.0.0.2:1234", "not-an-ip", "/api/health", 403},
		{"missing forwarded", "203.0.113.10", "10.0.0.2", "10.0.0.2:1234", "", "/api/health", 403},
		{"ipv6 CIDR", "2001:db8::/64", "", "[2001:db8::123]:1234", "", "/api/health", 200},
		{"mapped ipv4", "203.0.113.10", "", "[::ffff:203.0.113.10]:1234", "", "/api/health", 200},
		{"multiple entries", "192.0.2.1, 203.0.113.10", "", "203.0.113.10:1234", "", "/api/health", 200},
		{"invalid config", "invalid", "", "203.0.113.10:1234", "", "/api/health", 403},
		{"invalid proxy config", "203.0.113.10", "invalid", "203.0.113.10:1234", "", "/api/health", 403},
		{"health probe", "203.0.113.10", "", "127.0.0.1:1234", "", "/health", 200},
		{"UI denied", "203.0.113.10", "", "192.0.2.1:1234", "", "/", 403},
		{"dump denied", "203.0.113.10", "", "192.0.2.1:1234", "", "/api/dumps/a.sql", 403},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := New(Options{AllowedIPs: tt.allowed, TrustedProxies: tt.trusted})
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.RemoteAddr = tt.peer
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tt.forwarded)
			}
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("got %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestIPAllowlistStillRequiresAuth(t *testing.T) {
	srv := New(Options{AllowedIPs: "203.0.113.10", AuthUser: "admin", AuthPassword: "secret"})
	req := httptest.NewRequest(http.MethodGet, "/api/defaults", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestInvalidIPConfigStopsStartup(t *testing.T) {
	for _, opts := range []Options{{AllowedIPs: "bad"}, {TrustedProxies: "bad"}} {
		if err := New(opts).ListenAndServe(); err == nil {
			t.Fatal("invalid config accepted")
		}
	}
}
