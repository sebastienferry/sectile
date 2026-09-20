package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"io"
	"io/fs"
	"net"
	"time"

	"tasks/internal/auth"
	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/webui"
)

// loadDotEnv reads KEY=VALUE lines from a .env file next to the binary's working
// directory. A real environment variable always wins, so exporting a value in
// the shell overrides the file. Secrets such as SECTILE_TRACKER_TOKEN can then
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
	// .env.local last: it overrides nothing already exported, but is the usual
	// place for a machine-specific secret.
	// Le répertoire courant d'abord, pour la boucle de développement, puis le
	// dossier de données : une application lancée depuis n'importe où n'a pas de
	// répertoire courant qui lui appartienne. La première valeur trouvée gagne,
	// donc un développeur garde la main depuis son dépôt.
	loadDotEnv(".env", ".env.local", filepath.Join(appDataDir(), ".env"))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	if len(os.Args) > 1 {
		log.Fatal("sectile-server accepts configuration through environment variables; use sectile-agent for local execution and MCP")
	}

	dbPath, dbOrigin := resolveDBPath(os.Getenv("DB_PATH"))

	database, err := db.NewDB(dbPath)
	if err != nil {
		log.Fatalf("Fatal database error: %v", err)
	}
	defer database.Close()

	h := handlers.NewHandler(database)
	h.SetDataDir(appDataDir())
	h.SetPullOnConnect(true)
	if os.Getenv("SECTILE_SERVER_TOKEN") != "" {
		log.Printf("⚠️  %s", handlers.SharedServerTokenWarning)
	}

	// A declared provider that cannot be reached is a configuration error:
	// starting without it would silently serve the interface to everyone.
	if auth.Configured() {
		provider, err := auth.Discover(context.Background())
		if err != nil {
			log.Fatalf("Identity provider: %v", err)
		}
		h.SetIdentityProvider(provider)
		log.Printf("🔐 Sign-in enabled through %s", provider.Issuer())
	}

	// Pull active running/queued tasks from any available local agent on startup.
	h.TryPullLocalAgentTasks()

	// Boucle de synchronisation de fond. Elle ne fait rien tant que le réglage
	// est éteint, et ne lit ensuite que ce qui a changé depuis sa passe
	// précédente.
	database.StartAutoSync()

	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc(handlers.HealthPath, h.HandleHealth)
	mux.HandleFunc("/api/cli-status", h.HandleCliStatus)
	mux.HandleFunc("/api/git-status", h.HandleGitStatus)
	mux.HandleFunc("/api/git/status", h.HandleGitStatus)
	mux.HandleFunc("/api/git/branches", h.HandleGitBranches)
	mux.HandleFunc("/api/git-branches", h.HandleGitBranches)
	mux.HandleFunc("/api/git/branches/clean", h.HandleGitCleanBranches)
	mux.HandleFunc("/api/git/branches/delete", h.HandleGitDeleteBranch)
	mux.HandleFunc("/api/git/checkout", h.HandleGitCheckout)
	mux.HandleFunc("/api/git-checkout", h.HandleGitCheckout)
	mux.HandleFunc("/api/sync/all", h.HandleSyncAll)
	mux.HandleFunc("/api/sync/github", h.HandleSyncGithub)
	mux.HandleFunc("/api/sync/jira", h.HandleSyncJira)
	mux.HandleFunc("/api/sync/auto", h.HandleAutoSyncStatus)
	mux.HandleFunc("/api/setup/tracker", h.HandleTrackerSetup)
	mux.HandleFunc("/api/setup/tracker/check", h.HandleTrackerSetup)
	mux.HandleFunc("/api/skills", h.HandleSkills)
	mux.HandleFunc("/api/spec-framework/status", h.HandleSpecFrameworkStatus)
	mux.HandleFunc("/api/spec-framework/install", h.HandleSpecFrameworkInstall)
	mux.HandleFunc("/api/macros/", h.HandleMacroRoute)
	mux.HandleFunc("/api/projects", h.HandleProjects)
	mux.HandleFunc("/api/projects/", h.HandleProjectDetail)
	mux.HandleFunc("/api/tasks", h.HandleTasks)
	mux.HandleFunc("/api/tasks/stage", h.HandleTasks)
	mux.HandleFunc("/api/tasks/transition", h.HandleTasks)
	mux.HandleFunc("/api/tasks/postback", h.HandleTaskPostBack)
	mux.HandleFunc("/api/events", h.HandleEventsSSE)
	mux.HandleFunc("/api/events/sse", h.HandleEventsSSE)
	mux.HandleFunc("/api/tasks/facets", h.HandleTaskFacets)
	mux.HandleFunc("/api/teams", h.HandleTeams)
	mux.HandleFunc("/api/teams/", h.HandleTeams)
	mux.HandleFunc("/api/tasks/pins", h.HandleTaskPins)
	mux.HandleFunc("/api/tasks/", h.HandleTaskDetail)
	mux.HandleFunc("/api/activities", h.HandleActivities)
	mux.HandleFunc("/api/activities/", h.HandleActivityDetail)
	mux.HandleFunc("/api/settings", h.HandleSettings)
	mux.HandleFunc("/api/open-editor", h.HandleOpenEditor)
	mux.HandleFunc("/api/editor/open", h.HandleOpenEditor)
	mux.HandleFunc("/api/open-terminal", h.HandleOpenExternalTerminal)
	mux.HandleFunc("/api/terminal/external", h.HandleOpenExternalTerminal)

	// Interactive PTY Terminal Routes & WebSocket
	mux.HandleFunc("/ws/terminal", h.HandleTerminalWs)
	mux.HandleFunc("/api/terminal/sessions", h.HandleTerminalSessions)
	mux.HandleFunc("/api/terminal/send", h.HandleTerminalSend)
	mux.HandleFunc("/api/terminal/reset", h.HandleTerminalReset)

	// Sign-in routes exist only when a provider is configured; the interface
	// asks /api/me which of the two modes it is in.
	mux.HandleFunc("/auth/login", h.HandleLogin)
	mux.HandleFunc("/auth/callback", h.HandleAuthCallback)
	mux.HandleFunc("/auth/logout", h.HandleLogout)
	// Local sign-in exists only without a provider: an e-mail, no password, the
	// temporary mode of a team that has not connected its identity provider yet.
	mux.HandleFunc("/auth/local", h.HandleLocalSignIn)
	mux.HandleFunc("/api/me", h.HandleCurrentUser)
	mux.HandleFunc("/api/me/tracker-credentials", h.HandleUserTrackerCredentials)
	mux.HandleFunc("/api/me/tracker-credentials/", h.HandleUserTrackerCredentials)
	mux.HandleFunc("/api/me/project-bookmarks", h.HandleUserProjectBookmarks)
	mux.HandleFunc("/api/me/project-bookmarks/", h.HandleUserProjectBookmarks)
	// The admin's users view: list accounts and change roles.
	mux.HandleFunc("/api/users", h.HandleUsers)
	mux.HandleFunc("/api/users/", h.HandleUsers)

	mux.Handle("/mcp", h.MCPHandler())
	mux.HandleFunc("/api/mcp/sessions", h.HandleMCPSessions)
	// Pairing binds one workstation to one user; the code is the only
	// unauthenticated credential, and it is single use and short lived.
	mux.HandleFunc("/api/pairing-codes", h.HandlePairingCode)
	mux.HandleFunc("/api/devices", h.HandleDeviceCredentials)
	mux.HandleFunc("/api/v1/agent/pair", h.HandleAgentPair)
	mux.Handle("/api/v1/agent/identity", h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentIdentity)))
	mux.Handle("/api/v1/agent/config", h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentConfig)))
	mux.Handle("/api/v1/agent/projects", h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentProjects)))
	mux.Handle("/api/v1/agent/run-output", h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentRunOutput)))

	// Remote Agent WebSocket & Dispatch Routes
	mux.HandleFunc("/ws/agent-connect", h.HandleAgentConnect)
	mux.HandleFunc("/api/agent/status", h.HandleAgentStatus)
	mux.HandleFunc("/api/agent/dispatch", h.HandleAgentDispatch)
	mux.HandleFunc("/api/agent/pull", h.HandleAgentPull)

	// Interface : la copie embarquée d'abord, le dossier de build ensuite.
	//
	// L'embarqué est ce qui fait tenir l'application dans un fichier. Le repli
	// disque sert la boucle de développement, où l'on rebuild le front sans
	// recompiler le serveur.
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
				// Route de l'application : c'est index.html qui la résout.
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

	handlerWithCORS := h.EnableCORS(h.RequireSession(mux))

	addr := ":" + port
	url := fmt.Sprintf("http://localhost%s", addr)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		if alreadyServing(url) {
			log.Printf("Sectile is already running at %s.", url)
			return
		}
		log.Fatalf("Port %s indisponible et occupé par autre chose que Sectile: %v", addr, err)
	}

	log.Printf("🚀 Sectile Server listening on %s", url)
	log.Printf("   base : %s (%s)", dbPath, dbOrigin)

	if err := http.Serve(listener, handlerWithCORS); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
