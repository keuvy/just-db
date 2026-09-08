package server

import (
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"strings"
)

func parseIPRanges(value string) ([]netip.Prefix, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var ranges []netip.Prefix
	for _, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)
		prefix, err := netip.ParsePrefix(entry)
		if err != nil {
			addr, addrErr := netip.ParseAddr(entry)
			if addrErr != nil {
				return nil, fmt.Errorf("invalid IP or CIDR %q", entry)
			}
			addr = addr.Unmap()
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		ranges = append(ranges, prefix.Masked())
	}
	return ranges, nil
}

func containsIP(ranges []netip.Prefix, ip netip.Addr) bool {
	for _, prefix := range ranges {
		if prefix.Contains(ip.Unmap()) {
			return true
		}
	}
	return false
}

// Walk from the trusted peer toward the client, ignoring spoofable entries
// beyond the first untrusted hop.
func clientIP(r *http.Request, trusted []netip.Prefix) netip.Addr {
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}
	}
	ip := peer.Addr().Unmap()
	if !containsIP(trusted, ip) {
		return ip
	}
	forwarded := strings.Join(r.Header.Values("X-Forwarded-For"), ",")
	if forwarded == "" {
		return ip
	}
	chain := strings.Split(forwarded, ",")
	for i := len(chain) - 1; i >= 0 && containsIP(trusted, ip); i-- {
		ip, err = netip.ParseAddr(strings.TrimSpace(chain[i]))
		if err != nil {
			return netip.Addr{}
		}
		ip = ip.Unmap()
	}
	return ip
}

func (s *Server) withIPAllowlist(next http.Handler) http.Handler {
	allowed, allowErr := parseIPRanges(s.opts.AllowedIPs)
	trusted, trustErr := parseIPRanges(s.opts.TrustedProxies)
	if allowErr == nil && trustErr == nil && len(allowed) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Keep the container's liveness probe reachable.
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		ip := clientIP(r, trusted)
		if allowErr != nil || trustErr != nil || !containsIP(allowed, ip) {
			log.Printf("access denied client_ip=%q", ip.String())
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "IP not allowed"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
