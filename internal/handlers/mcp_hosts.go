package handlers

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
)

// mcpHostProtection extends the SDK's loopback DNS rebinding guard with an
// explicit deployment allowlist. A hosting ingress can reach the server on
// loopback while preserving its public Host. The socket address alone cannot
// distinguish that from a browser targeting a local server through DNS.
//
// Both MCP routes share this guard, inside their existing authentication.
// Forwarded headers are deliberately not consulted: they are request input,
// not evidence that the caller came through a trusted ingress.
func mcpHostProtection(next http.Handler) http.Handler {
	allowed := make(map[string]bool)
	for _, host := range strings.Split(os.Getenv("SECTILE_MCP_ALLOWED_HOSTS"), ",") {
		if host = strings.ToLower(strings.TrimSpace(host)); host != "" {
			allowed[host] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
		if ok && local != nil && mcpLoopbackAddress(local.String()) &&
			!mcpLoopbackAddress(r.Host) && !allowed[strings.ToLower(r.Host)] {
			http.Error(w, fmt.Sprintf("Forbidden: invalid Host header %q", r.Host), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// mcpLoopbackAddress follows the SDK's address check, accepting bare hosts and
// host:port authorities without resolving names through DNS.
func mcpLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = strings.Trim(address, "[]")
	}
	if host == "localhost" {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}
