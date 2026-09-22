package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"tasks/internal/testhome"
	"testing"
)

func TestResolveDBPathPrecedence(t *testing.T) {
	// Scenario 1: Explicit DB_PATH always wins
	t.Run("explicit DB_PATH wins", func(t *testing.T) {
		explicit := "/custom/path/my_tasks.db"
		path, origin := resolveDBPath(explicit)
		if path != explicit || origin != "DB_PATH" {
			t.Fatalf("expected (%q, 'DB_PATH'), got (%q, %q)", explicit, path, origin)
		}
	})

	// Scenario 2: Working directory tasks.db takes precedence over user config dir
	t.Run("working directory tasks.db wins over user config", func(t *testing.T) {
		tempDir := t.TempDir()
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tempDir); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(cwd) }()

		// Create tasks.db in the working directory
		localDB := filepath.Join(tempDir, "tasks.db")
		if err := os.WriteFile(localDB, []byte("local"), 0644); err != nil {
			t.Fatal(err)
		}

		path, origin := resolveDBPath("")
		if path != "tasks.db" || origin != "base trouvée dans le répertoire courant" {
			t.Fatalf("expected ('tasks.db', 'base trouvée dans le répertoire courant'), got (%q, %q)", path, origin)
		}
	})

	// Scenario 3 & 4: User config dir sectile vs taskacao
	t.Run("user config directory sectile then legacy taskacao", func(t *testing.T) {
		tempDir := t.TempDir()
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tempDir); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(cwd) }()

		// Mock user config dir via HOME and XDG_CONFIG_HOME
		mockHome := tempDir
		testhome.Set(t, mockHome)
		mockConfigDir := filepath.Join(tempDir, "mock_config")
		t.Setenv("XDG_CONFIG_HOME", mockConfigDir)

		userDir, err := os.UserConfigDir()
		if err != nil {
			t.Fatal(err)
		}

		sectileDir := filepath.Join(userDir, "sectile")
		taskacaoDir := filepath.Join(userDir, "taskacao")
		if err := os.MkdirAll(sectileDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(taskacaoDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Only taskacao exists first
		legacyDB := filepath.Join(taskacaoDir, "tasks.db")
		if err := os.WriteFile(legacyDB, []byte("legacy"), 0644); err != nil {
			t.Fatal(err)
		}

		path, origin := resolveDBPath("")
		if path != legacyDB || origin != "dossier de données (legacy taskacao)" {
			t.Fatalf("expected legacy taskacao db (%q), got (%q, %q)", legacyDB, path, origin)
		}

		// Now sectile DB also exists -> sectile must take precedence over taskacao
		sectileDB := filepath.Join(sectileDir, "tasks.db")
		if err := os.WriteFile(sectileDB, []byte("sectile"), 0644); err != nil {
			t.Fatal(err)
		}

		path, origin = resolveDBPath("")
		if path != sectileDB || origin != "dossier de données" {
			t.Fatalf("expected sectile db (%q), got (%q, %q)", sectileDB, path, origin)
		}
	})
}

func TestAlreadyServing(t *testing.T) {
	tests := []struct {
		name     string
		response string
		status   int
		expected bool
	}{
		{
			name:     "sectile-api service identifier",
			response: `{"status":"ok","service":"sectile-api"}`,
			status:   http.StatusOK,
			expected: true,
		},
		{
			name:     "sectile service identifier",
			response: `{"status":"ok","service":"sectile-api"}`,
			status:   http.StatusOK,
			expected: true,
		},
		{
			name:     "taskacao legacy service",
			response: `{"status":"ok","service":"taskacao"}`,
			status:   http.StatusOK,
			expected: true,
		},
		{
			name:     "unrelated service",
			response: `{"status":"ok","service":"something-else"}`,
			status:   http.StatusOK,
			expected: false,
		},
		{
			name:     "http error status",
			response: `{"status":"error"}`,
			status:   http.StatusInternalServerError,
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/health" {
					w.WriteHeader(tc.status)
					fmt.Fprint(w, tc.response)
					return
				}
				http.NotFound(w, r)
			}))
			defer server.Close()

			got := alreadyServing(server.URL)
			if got != tc.expected {
				t.Fatalf("alreadyServing(%s) = %v; want %v", tc.name, got, tc.expected)
			}
		})
	}
}
