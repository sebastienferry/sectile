package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"io"
	"io/fs"
	"net"
	"time"

	"tasks/internal/auth"
	"tasks/internal/backend"
	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/localruntime"
	"tasks/internal/trackerapi"
	"tasks/internal/version"
	"tasks/internal/webui"
)

// isVersionArgument reports whether the single accepted argument was asked for.
// The three spellings are the ones people actually type; anything else is a
// configuration flag the server refuses rather than ignores.
func isVersionArgument(arg string) bool {
	switch arg {
	case "--version", "-version", "version":
		return true
	}
	return false
}

// loadDotEnv reads KEY=VALUE lines from a .env file next to the binary's working
// directory. A real environment variable always wins, so exporting a value in
// the shell overrides the file. Secrets such as SECTILE_GITHUB_TOKEN can then
// live outside the database and outside git, .env being already gitignored.
func loadDotEnv(paths ...string) {
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimPrefix(line, "export ")
			key, value, found := strings.Cut(line, "=")
			if !found {
				continue
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			// Quotes are what a user naturally types around a secret.
			if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' || value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
			if key == "" {
				continue
			}
			if _, already := os.LookupEnv(key); already {
				continue
			}
			_ = os.Setenv(key, value)
		}
		log.Printf("Loaded environment from %s", path)
	}
}

// appDataDir is where the application keeps what belongs to it: the database,
// and the environment file holding the tracker token when the user would rather
// not store it in the database.
//
// It returns an empty string when the user's data directory cannot be resolved
// or created, which callers treat as "use the working directory instead".
func appDataDir() string {
	if configured := strings.TrimSpace(os.Getenv("SECTILE_SERVER_DATA_DIR")); configured != "" {
		if !filepath.IsAbs(configured) {
			log.Fatal("SECTILE_SERVER_DATA_DIR must be an absolute path")
		}
		if err := os.MkdirAll(configured, 0o700); err != nil {
			log.Fatalf("Cannot create SECTILE_SERVER_DATA_DIR: %v", err)
		}
		return configured
	}
	dir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		return ""
	}
	appDir := filepath.Join(dir, "sectile")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return ""
	}
	return appDir
}

// resolveDBPath decides where the database lives.
//
// Three cases, in this order. An explicit DB_PATH wins, always. Then a database
// already sitting in the working directory is kept: someone who has been running
// the program from a checkout must not silently start from an empty board. Only
// otherwise does the database go to the user's data directory, which is what a
// distributed binary needs, since it may be launched from anywhere.
func resolveDBPath(explicit string) (path string, origin string) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, "DB_PATH"
	}
	// An explicit data directory must never adopt a different installation.
	if strings.TrimSpace(os.Getenv("SECTILE_SERVER_DATA_DIR")) != "" {
		return filepath.Join(appDataDir(), "tasks.db"), "SECTILE_SERVER_DATA_DIR"
	}
	if fi, err := os.Stat("tasks.db"); err == nil && fi.Size() > 0 {
		return "tasks.db", "base trouvée dans le répertoire courant"
	}

	appDir := appDataDir()
	if appDir != "" {
		sectileDB := filepath.Join(appDir, "tasks.db")
		if fi, err := os.Stat(sectileDB); err == nil && fi.Size() > 0 {
			return sectileDB, "dossier de données"
		}
		// Fallback to legacy taskacao directory if it exists
		if userDir, err := os.UserConfigDir(); err == nil && userDir != "" {
			legacyDB := filepath.Join(userDir, "taskacao", "tasks.db")
			if fi, err := os.Stat(legacyDB); err == nil && fi.Size() > 0 {
				return legacyDB, "dossier de données (legacy taskacao)"
			}
		}
		return sectileDB, "dossier de données"
	}
	return "tasks.db", "répertoire courant, dossier de données indisponible"
}

// alreadyServing reports whether the port is held by another Sectile rather than
// by an unrelated program. The health endpoint is the only honest way to know,
// and it decides between "your window is already open" and "something else is on
// this port".
func alreadyServing(baseURL string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + handlers.HealthPath)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "sectile") || strings.Contains(lower, "sectile") || strings.Contains(lower, "taskacao")
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if len(os.Args) == 2 && os.Args[1] == "--local-runtime-info" {
		_ = json.NewEncoder(os.Stdout).Encode(localruntime.Info())
		return nil
	}
	// .env.local last: it overrides nothing already exported, but is the usual
	// place for a machine-specific secret.
	// Search the working directory first for development, then the application
	// data directory for launches from arbitrary locations. The first value
	// found wins, so a developer can override it from the checkout.
	loadDotEnv(".env", ".env.local", filepath.Join(appDataDir(), ".env"))
	for _, warning := range trackerapi.RemovedTokenVariableWarnings(os.Getenv) {
		log.Printf("⚠️  %s", warning)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
		if localruntime.Active() {
			port = fmt.Sprint(localruntime.Port)
		}
	}

	// Version and public runtime discovery are read-only arguments. Everything else is
	// configuration, and configuration arrives through the environment: a flag
	// the server silently ignored is how a deployment ends up running with
	// settings nobody applied.
	if len(os.Args) > 1 {
		if isVersionArgument(os.Args[1]) {
			fmt.Println("sectile-server " + version.String())
			return nil
		}
		log.Fatal("sectile-server accepts configuration through environment variables; use sectile-agent for local execution and MCP")
	}

	if localruntime.Active() {
		if err := localruntime.Validate(os.Getenv("SECTILE_SERVER_HOST"), port, os.Getenv("SECTILE_SERVER_DATA_DIR")); err != nil {
			log.Fatal(err)
		}
	}

	log.Printf("Sectile server %s", version.String())

	dbConfig, dbTarget, dbOrigin, err := resolveDBConfig(osGetenv)
	if err != nil {
		log.Fatalf("Database configuration: %v", err)
	}

	if err := applyInstanceTiming(osGetenv); err != nil {
		log.Fatalf("Configuration: %v", err)
	}
	shutdownGrace, err := shutdownGraceFromEnv(osGetenv)
	if err != nil {
		log.Fatalf("Configuration: %v", err)
	}

	// A declared identity provider must be reachable before the backend starts.
	var provider *auth.Provider
	if auth.Configured() {
		provider, err = auth.Discover(context.Background())
		if err != nil {
			log.Fatalf("Identity provider: %v", err)
		}
		log.Printf("🔐 Sign-in enabled through %s", provider.Issuer())
	}

	internalPort := internalPortFromEnv(osGetenv)
	instanceAddress := ""
	if dbConfig.Driver == db.DriverPostgres {
		instanceAddress = internalURL(osGetenv, internalPort)
	}
	// An unavailable PostgreSQL store must fail rather than fall back to an
	// empty SQLite board. Local store ownership will precede Open in phase 2.
	runtime, err := backend.Open(backend.Config{
		Database:         dbConfig,
		DataDir:          appDataDir(),
		InstanceAddress:  instanceAddress,
		IdentityProvider: provider,
	})
	if err != nil {
		log.Fatalf("Fatal backend error: %v", err)
	}
	defer runtime.Close()
	database := runtime.Database
	h := runtime.Handler
	log.Printf("   instance : %s", database.InstanceID())
	if database.Shared() {
		startInternalListener(h, internalPort)
	}
	if os.Getenv("SECTILE_SERVER_TOKEN") != "" {
		log.Printf("⚠️  %s", handlers.SharedServerTokenWarning)
	}

	mux := http.NewServeMux()

	backend.RegisterRoutes(mux, h)

	// Serve embedded website assets first, then the development build directory.
	//
	// Embedded assets keep the server self-contained. The disk fallback allows
	// frontend rebuilds without recompiling the server during development.
	uiFS, uiEmbedded := webui.FS()
	webDistDir := "./internal/webui/dist"
	if !uiEmbedded {
		if _, err := os.Stat(webDistDir); err == nil {
			uiFS = os.DirFS(webDistDir)
			uiEmbedded = true
			log.Printf("Interface servie depuis %s", webDistDir)
		}
	}

	if uiEmbedded {
		fileServer := http.FileServer(http.FS(uiFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Route API non trouvée: %s", r.URL.Path)})
				return
			}
			// index.html must never be cached: it is the file that names the
			// hashed bundle, so a stale copy keeps serving the previous build and
			// the app looks unchanged after a rebuild. The assets themselves are
			// content-hashed, so they can be cached hard.
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-store, must-revalidate")
			}
			name := strings.TrimPrefix(r.URL.Path, "/")
			if name == "" {
				name = "index.html"
			}
			if _, err := fs.Stat(uiFS, name); err != nil {
				// Let index.html resolve website application routes.
				index, err := fs.ReadFile(uiFS, "index.html")
				if err != nil {
					http.Error(w, "interface indisponible", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(index)
				return
			}
			fileServer.ServeHTTP(w, r)
		})
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Route API non trouvée: %s", r.URL.Path)})
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Sectile API</title></head>
<body style="font-family: system-ui; padding: 2rem; background: #0f172a; color: #f8fafc;">
  <h2>Sectile Go API is running!</h2>
  <p>To run the frontend with hot-reloading, run <code>cd web && npm run dev</code></p>
  <p>Or build the bundle via <code>make build</code>, which compiles the interface into the binary.</p>
  <p>API endpoints available at <a href="/api/tasks" style="color: #818cf8;">/api/tasks</a>, <a href="/api/skills" style="color: #818cf8;">/api/skills</a> and <a href="/api/settings" style="color: #818cf8;">/api/settings</a>.</p>
</body>
</html>`)
		})
	}

	// The instrumentation wraps the whole chain, so a request the session
	// guard refuses is counted as the 401 or 403 it received. Compression sits
	// just inside it, so every answer, the refusals included, leaves compressed
	// for a client that accepts it.

	handlerWithCORS := backend.WrapHTTP(mux, h, database)

	handlerWithCORS = localRuntimeProbe(handlerWithCORS, os.Getenv("SECTILE_LOCAL_RUNTIME_KEY"))

	addr, err := listenAddress(os.Getenv("SECTILE_SERVER_HOST"), port)
	if err != nil {
		return err
	}
	urlHost := os.Getenv("SECTILE_SERVER_HOST")
	if urlHost == "" {
		urlHost = "localhost"
	}
	url := "http://" + net.JoinHostPort(urlHost, port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		if alreadyServing(url) {
			log.Printf("Sectile is already running at %s.", url)
			return nil
		}
		return fmt.Errorf("Port %s indisponible et occupé par autre chose que Sectile: %w", addr, err)
	}

	log.Printf("🚀 Sectile Server listening on %s", url)
	log.Printf("   base : %s - %s (%s)", database.EngineName(), dbTarget, dbOrigin)

	// A stop asked by the orchestrator, or by Ctrl+C, drains the instance
	// rather than dropping it: see drain.
	server := &http.Server{Handler: handlerWithCORS}
	stopping, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopSignals()
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	select {
	case err := <-served:
		h.BeginDrain()
		h.CloseAgentConnections()
		_ = server.Close()
		return fmt.Errorf("Server failed: %w", err)
	case <-stopping.Done():
		stopSignals()
		drain(h, runtime.StopInstance, server, shutdownGrace)
	}
	return nil
}
