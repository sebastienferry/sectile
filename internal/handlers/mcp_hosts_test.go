package handlers

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The hosting ingress connects to the backend over loopback but preserves its
// public Host. An unauthenticated probe stops at the outer auth middleware and
// misses the SDK's rejection, so exercise initialization and the tool catalog.
func TestMCPBehindLoopbackIngress(t *testing.T) {
	t.Setenv("SECTILE_MCP_ALLOWED_HOSTS", "sectile.example.test")
	h, _, cleanup := setupTestHandler(t)
	defer cleanup()
	key := agentTestKey(t, h)
	backend := httptest.NewServer(h.MCPHandler())
	defer backend.Close()
	target, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	ingress := httptest.NewServer(&httputil.ReverseProxy{Rewrite: func(pr *httputil.ProxyRequest) {
		pr.SetURL(target)
		pr.Out.Host = "sectile.example.test"
	}})
	defer ingress.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "ingress-test", Version: "1"}, nil).Connect(ctx,
		&mcp.StreamableClientTransport{Endpoint: ingress.URL + "/mcp", HTTPClient: &http.Client{Transport: testTokenTransport{key}}}, nil)
	if err != nil {
		t.Fatalf("initialize through ingress: %v", err)
	}
	defer session.Close()
	catalog, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools through ingress: %v", err)
	}
	for _, tool := range catalog.Tools {
		if tool.Name == "get_task" {
			return
		}
	}
	t.Fatal("the tool catalog does not contain get_task")
}

func TestMCPHostProtection(t *testing.T) {
	for _, tc := range []struct {
		name, local, host, allowed string
		want                       int
	}{
		{"local IPv4", "127.0.0.1", "127.0.0.1:8090", "", 204},
		{"localhost", "127.0.0.1", "localhost:8090", "", 204},
		{"bare localhost", "127.0.0.1", "localhost", "", 204},
		{"local IPv6", "::1", "[::1]:8090", "", 204},
		{"bare IPv6", "::1", "[::1]", "", 204},
		{"public host blocked by default", "127.0.0.1", "sectile.example.test", "", 403},
		{"IPv6 also guarded", "::1", "sectile.example.test", "", 403},
		{"explicit public host", "127.0.0.1", "sectile.example.test", "sectile.example.test", 204},
		{"explicit host on IPv6", "::1", "sectile.example.test", "sectile.example.test", 204},
		{"case insensitive", "127.0.0.1", "SECTILE.example.test", "sectile.EXAMPLE.test", 204},
		{"list and whitespace", "127.0.0.1", "sectile.example.test", " , other.example.test, sectile.example.test, ", 204},
		{"explicit port", "127.0.0.1", "sectile.example.test:8443", "sectile.example.test:8443", 204},
		{"unlisted port", "127.0.0.1", "sectile.example.test:8443", "sectile.example.test", 403},
		{"port required when configured", "127.0.0.1", "sectile.example.test", "sectile.example.test:8443", 403},
		{"unlisted host", "127.0.0.1", "evil.example.test", "sectile.example.test", 403},
		{"host suffix", "127.0.0.1", "sectile.example.test.evil.test", "sectile.example.test", 403},
		{"localhost suffix", "127.0.0.1", "localhost.evil.test", "sectile.example.test", 403},
		{"no wildcard expansion", "127.0.0.1", "evil.example.test", "*", 403},
		{"URL is not a host", "127.0.0.1", "sectile.example.test", "https://sectile.example.test", 403},
		{"empty entries allow nothing", "127.0.0.1", "evil.example.test", " , , ", 403},
		{"local access remains allowed", "127.0.0.1", "localhost:8090", "sectile.example.test", 204},
		{"nonloopback behavior unchanged", "192.0.2.1", "sectile.example.test", "", 204},
		{"missing socket context unchanged", "", "sectile.example.test", "", 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SECTILE_MCP_ALLOWED_HOSTS", tc.allowed)
			r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			r.Host = tc.host
			// Spoofing these must not let an unlisted Host through the guard.
			r.Header.Set("X-Forwarded-Host", "sectile.example.test")
			r.Header.Set("X-Forwarded-For", "127.0.0.1")
			r.Header.Set("Forwarded", "for=127.0.0.1;host=sectile.example.test;proto=https")
			if tc.local != "" {
				local := &net.TCPAddr{IP: net.ParseIP(tc.local), Port: 8090}
				r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, local))
			}
			rec := httptest.NewRecorder()
			mcpHostProtection(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(rec, r)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestMCPIngressGuards(t *testing.T) {
	t.Setenv("SECTILE_MCP_ALLOWED_HOSTS", "sectile.example.test")
	h, _, cleanup := setupTestHandler(t)
	defer cleanup()
	key := agentTestKey(t, h)
	h.mcpCluster = &mcpCluster{token: "cluster-secret"}
	for _, route := range []struct {
		name    string
		handler http.Handler
	}{
		{"public", h.MCPHandler()},
		{"internal", h.InternalMCPHandler()},
	} {
		for _, tc := range []struct {
			name, host, token, origin string
			want                      int
		}{
			{"unlisted host", "evil.example.test", key, "", 403},
			{"no token", "sectile.example.test", "", "", 401},
			{"invalid token", "sectile.example.test", "not-a-key", "", 401},
			{"foreign browser origin", "sectile.example.test", key, "https://evil.example.test", 403},
			{"same browser origin", "sectile.example.test", key, "https://sectile.example.test", 403},
			{"opaque browser origin", "sectile.example.test", key, "null", 403},
		} {
			t.Run(route.name+"/"+tc.name, func(t *testing.T) {
				r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
				r.Host = tc.host
				r.Header.Set("X-Forwarded-Host", "sectile.example.test")
				r.Header.Set("Authorization", "Bearer "+tc.token)
				r.Header.Set("Origin", tc.origin)
				r.Header.Set(internalAuthHeader, "Bearer cluster-secret")
				r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8090}))
				rec := httptest.NewRecorder()
				route.handler.ServeHTTP(rec, r)
				if rec.Code != tc.want {
					t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
				}
			})
		}
	}
}
