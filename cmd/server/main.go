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
	"os/exec"
	"runtime"
	"time"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"
	"tasks/internal/webui"
)

// loadDotEnv reads KEY=VALUE lines from a .env file next to the binary's working
// directory. A real environment variable always wins, so exporting a value in
// the shell overrides the file. Secrets such as TASKFLOW_JIRA_API_TOKEN can then
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
	appDir := filepath.Join(dir, "taskflow")
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
		taskflowDB := filepath.Join(appDir, "tasks.db")
		if fi, err := os.Stat(taskflowDB); err == nil && fi.Size() > 0 {
			return taskflowDB, "dossier de données"
		}
		// Fallback to legacy taskacao directory if it exists
		if userDir, err := os.UserConfigDir(); err == nil && userDir != "" {
			legacyDB := filepath.Join(userDir, "taskacao", "tasks.db")
			if fi, err := os.Stat(legacyDB); err == nil && fi.Size() > 0 {
				return legacyDB, "dossier de données (legacy taskacao)"
			}
		}
		return taskflowDB, "dossier de données"
	}
	return "tasks.db", "répertoire courant, dossier de données indisponible"
}

// alreadyServing reports whether the port is held by another TaskFlow rather than
// by an unrelated program. The health endpoint is the only honest way to know,
// and it decides between "your window is already open" and "something else is on
// this port".
func alreadyServing(baseURL string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + "/api/health")
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
	return strings.Contains(lower, "taskflow") || strings.Contains(lower, "taskacao")
}

// openBrowser opens the interface once the server listens. It is best effort by
// design: a machine without a browser, or a headless run, must not turn a
// cosmetic step into a failure to start.
func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", ""}
	default:
		cmd = "xdg-open"
	}
	args = append(args, url)
	if err := exec.Command(cmd, args...).Start(); err != nil {
		log.Printf("Navigateur non ouvert (%v). Ouvrez %s à la main.", err, url)
	}
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

	if len(os.Args) >= 2 {
		cmd := strings.ToLower(os.Args[1])
		if cmd == "mcp" {
			if err := runMCPCommand(context.Background(), os.Args[2:]); err != nil {
				log.Fatalf("MCP server: %v", err)
			}
			return
		}
		if cmd == "agent-exec" {
			if err := runAgentExec(os.Args[2:]); err != nil {
				log.Print(err)
				os.Exit(1)
			}
			return
		}
		if cmd == "agent" {
			runAgentCommand(os.Args[2:])
			return
		}
		if cmd == "stage" || cmd == "transition" || cmd == "set-stage" {
			handleCliStageCommand(port, os.Args[2:])
			return
		}
		if cmd == "sync-skills" || cmd == "install-skills" {
			dbPath, _ := resolveDBPath(os.Getenv("DB_PATH"))
			database, err := db.NewDB(dbPath)
			if err != nil {
				log.Fatalf("Fatal database error: %v", err)
			}
			defer database.Close()

			target := "."
			if len(os.Args) > 2 {
				target = os.Args[2]
			}
			n, err := database.WriteAllProjectSkillsToRepo(target)
			if err != nil {
				log.Fatalf("Error writing skills: %v", err)
			}
			fmt.Printf("✅ %d skill files synchronized across all agent directories (.agents, .agy, .claude, .gemini, .skills)\n", n)
			return
		}
	}

	dbPath, dbOrigin := resolveDBPath(os.Getenv("DB_PATH"))

	database, err := db.NewDB(dbPath)
	if err != nil {
		log.Fatalf("Fatal database error: %v", err)
	}
	defer database.Close()

	h := handlers.NewHandler(database)
	h.SetDataDir(appDataDir())

	// Boucle de synchronisation de fond. Elle ne fait rien tant que le réglage
	// est éteint, et ne lit ensuite que ce qui a changé depuis sa passe
	// précédente.
	database.StartAutoSync()

	// Les pas du workflow tournent dans la session PTY de leur tâche : visibles
	// pendant qu'ils travaillent, ouvrables d'un clic, et interrogeables.
	database.SetTerminalRunner(h.TerminalRunner())

	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("/api/health", h.HandleHealth)
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
	mux.HandleFunc("/api/sync/linear", h.HandleSyncLinear)
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

	mux.Handle("/mcp", h.MCPHandler())
	mux.Handle("/api/v1/agent/config", h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentConfig)))
	mux.Handle("/api/v1/agent/projects", h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentProjects)))

	// Remote Agent WebSocket & Dispatch Routes
	mux.HandleFunc("/ws/agent-connect", h.HandleAgentConnect)
	mux.HandleFunc("/api/agent/status", h.HandleAgentStatus)
	mux.HandleFunc("/api/agent/dispatch", h.HandleAgentDispatch)

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
<head><title>TaskFlow API</title></head>
<body style="font-family: system-ui; padding: 2rem; background: #0f172a; color: #f8fafc;">
  <h2>TaskFlow Go API is running!</h2>
  <p>To run the frontend with hot-reloading, run <code>cd web && npm run dev</code></p>
  <p>Or build the bundle via <code>make build</code>, which compiles the interface into the binary.</p>
  <p>API endpoints available at <a href="/api/tasks" style="color: #818cf8;">/api/tasks</a>, <a href="/api/skills" style="color: #818cf8;">/api/skills</a> and <a href="/api/settings" style="color: #818cf8;">/api/settings</a>.</p>
</body>
</html>`)
		})
	}

	handlerWithCORS := h.EnableCORS(mux)

	addr := ":" + port
	url := fmt.Sprintf("http://localhost%s", addr)

	// Le port est réservé avant toute autre chose. Ouvrir le navigateur d'abord,
	// comme le faisait la version précédente, ouvrait un onglet même quand
	// l'écoute échouait ensuite : lancer l'application une seconde fois
	// rechargeait l'onglet de la première, puis mourait sur « address already in
	// use ».
	shouldOpenBrowser := func() bool {
		noBrowserEnv := strings.ToLower(strings.TrimSpace(os.Getenv("TASKFLOW_NO_BROWSER")))
		if noBrowserEnv == "" {
			noBrowserEnv = strings.ToLower(strings.TrimSpace(os.Getenv("TASKACAO_NO_BROWSER")))
		}
		if noBrowserEnv == "1" || noBrowserEnv == "true" || noBrowserEnv == "yes" {
			return false
		}

		openBrowserEnv := strings.ToLower(strings.TrimSpace(os.Getenv("TASKFLOW_OPEN_BROWSER")))
		if openBrowserEnv == "" {
			openBrowserEnv = strings.ToLower(strings.TrimSpace(os.Getenv("TASKACAO_OPEN_BROWSER")))
		}
		if openBrowserEnv == "0" || openBrowserEnv == "false" || openBrowserEnv == "no" {
			return false
		}
		if openBrowserEnv == "1" || openBrowserEnv == "true" || openBrowserEnv == "yes" {
			return true
		}

		// Opening a browser requires an explicit opt-in, including packaged builds.
		return false
	}()

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		if alreadyServing(url) {
			log.Printf("TaskFlow is already running at %s.", url)
			if shouldOpenBrowser {
				openBrowser(url)
			}
			return
		}
		log.Fatalf("Port %s indisponible et occupé par autre chose que TaskFlow: %v", addr, err)
	}

	log.Printf("🚀 TaskFlow Server listening on %s", url)
	log.Printf("   base : %s (%s)", dbPath, dbOrigin)

	if shouldOpenBrowser {
		go openBrowser(url)
	}

	if err := http.Serve(listener, handlerWithCORS); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleCliStageCommand(defaultPort string, args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: taskflow stage <TASK_KEY_OR_ID> <STAGE> [NOTE] [--pr-url <URL>] [--branch <BRANCH>]")
		fmt.Println("Stages: new, clarified, specified, implemented, reviewed, finished")
		fmt.Println("Example: taskflow stage PROJ-123 clarified \"Questions answered, scope validated\"")
		os.Exit(1)
	}

	taskIDOrKey := args[0]
	stage := args[1]
	note := ""
	prURL := ""
	branch := ""

	for i := 2; i < len(args); i++ {
		arg := args[i]
		if arg == "--pr-url" && i+1 < len(args) {
			prURL = args[i+1]
			i++
		} else if arg == "--branch" && i+1 < len(args) {
			branch = args[i+1]
			i++
		} else {
			if note == "" {
				note = arg
			} else {
				note += " " + arg
			}
		}
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%s", defaultPort)
	if envURL := os.Getenv("TASKFLOW_API_URL"); envURL != "" {
		baseURL = strings.TrimRight(envURL, "/")
	}

	// 1. If server is already running, invoke HTTP endpoint
	if alreadyServing(baseURL) {
		payload, _ := json.Marshal(map[string]string{
			"taskId": taskIDOrKey,
			"stage":  stage,
			"note":   note,
			"prUrl":  prURL,
			"branch": branch,
		})
		client := &http.Client{Timeout: 10 * time.Second}
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/tasks/stage", baseURL), strings.NewReader(string(payload)))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				bodyBytes, _ := io.ReadAll(resp.Body)
				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
					var res struct {
						Success bool         `json:"success"`
						Message string       `json:"message"`
						Task    *models.Task `json:"task"`
					}
					_ = json.Unmarshal(bodyBytes, &res)
					if res.Message != "" {
						fmt.Printf("✅ %s\n", res.Message)
					} else {
						fmt.Printf("✅ Tâche %s passée à l'étape %s\n", taskIDOrKey, stage)
					}
					return
				}
				log.Printf("⚠️ Erreur API (%d): %s, repli vers base locale...", resp.StatusCode, string(bodyBytes))
			}
		}
	}

	// 2. Direct local DB fallback
	dbPath, _ := resolveDBPath(os.Getenv("DB_PATH"))
	database, err := db.NewDB(dbPath)
	if err != nil {
		log.Fatalf("❌ Erreur d'ouverture de la base locale: %v", err)
	}
	defer database.Close()

	task, act, err := database.TransitionTaskStage(taskIDOrKey, stage, note, prURL, branch)
	if err != nil {
		log.Fatalf("❌ Impossible de changer l'étape de la tâche: %v", err)
	}
	fmt.Printf("✅ Tâche %s (%s) passée à l'étape « %s » [#%s]\n", task.Key, task.Title, stage, stage)
	if act != nil {
		fmt.Printf("   Activité enregistrée : %s\n", act.ID)
	}
}
