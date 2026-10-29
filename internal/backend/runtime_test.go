package backend

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/metrics"
)

func TestStandaloneBackendRegistersAPIMCPAuthAndMetricsWithoutWebsite(t *testing.T) {
	runtime, err := Open(Config{
		Database: db.SQLiteConfig(filepath.Join(t.TempDir(), "tasks.db")),
		DataDir:  t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := runtime.Close(); err != nil {
			t.Fatal(err)
		}
	})

	mux := http.NewServeMux()
	RegisterRoutes(mux, runtime.Handler)
	for _, path := range []string{
		handlers.HealthPath,
		"/api/projects",
		"/api/tasks",
		"/api/me",
		handlers.JiraOAuthCallbackPath,
		"/mcp",
		"/ws/agent-connect",
		"/api/v1/agent/config",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		_, pattern := mux.Handler(req)
		if pattern == "" {
			t.Errorf("backend route %s is missing", path)
		}
	}
	if _, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, "/", nil)); pattern != "" {
		t.Fatalf("backend unexpectedly registered website route %q", pattern)
	}

	httpHandler := WrapHTTP(mux, runtime.Handler, runtime.Database)
	for _, tc := range []struct {
		path   string
		status int
	}{
		{handlers.HealthPath, http.StatusOK},
		{metrics.Path, http.StatusOK},
	} {
		response := httptest.NewRecorder()
		httpHandler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if response.Code != tc.status {
			t.Errorf("%s returned %d, want %d", tc.path, response.Code, tc.status)
		}
	}
}
