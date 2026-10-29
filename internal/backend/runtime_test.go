package backend

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/metrics"
	"tasks/internal/storelock"
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
		"/api/desktop-oauth/apps",
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

	httpHandler := runtime.HTTPHandler(mux)
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

func TestLocalBackendOwnsStoreBeforeDatabaseOpenAndReleasesItAfterClose(t *testing.T) {
	root := filepath.Join(t.TempDir(), "local")
	config := Config{
		Database:  db.SQLiteConfig(filepath.Join(root, "tasks.db")),
		DataDir:   root,
		StoreRoot: root,
	}
	first, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := Open(config); !errors.Is(err, storelock.ErrOwned) || second != nil {
		t.Fatalf("second Local backend: runtime=%v error=%v", second, err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLocalBackendReleasesOwnershipOnDatabaseOpenFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "local")
	if runtime, err := Open(Config{
		Database:  db.SQLiteConfig(root),
		StoreRoot: root,
	}); err == nil || runtime != nil {
		t.Fatalf("database directory accepted as SQLite file: runtime=%v error=%v", runtime, err)
	}
	lock, err := storelock.Acquire(root)
	if err != nil {
		t.Fatalf("failed startup retained Local ownership: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
}
