package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/auth"
	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/taskmcp"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Event struct {
	Type     string               `json:"type"` // e.g. "task_updated", "postback"
	Task     *models.Task         `json:"task,omitempty"`
	Activity *models.TaskActivity `json:"activity,omitempty"`
	Error    string               `json:"error,omitempty"`
}

type Handler struct {
	db *db.DB
	// dataDir is where the application keeps its own files, the environment file
	// holding the tracker token included. Empty when the process could not
	// resolve one, in which case the token can only go to the database.
	dataDir         string
	subscribers     map[chan Event]bool
	subMu           sync.RWMutex
	agentDispatcher *AgentDispatcher
	pullOnConnect   bool
	// mcpSessions owns the lifecycle of MCP client sessions and of the runs
	// they start, so a client that disappears cannot leave a task active.
	mcpSessions *taskmcp.SessionRegistry
	// identityProvider is nil when no OpenID Connect provider is configured,
	// which leaves the interface on its single implicit user.
	identityProvider *auth.Provider
	// agentPingInterval and agentReadTimeout tune the WebSocket keepalive that
	// detects agents which vanished without closing their connection. Set once
	// at construction; tests shorten them to observe a drop quickly.
	agentPingInterval time.Duration
	agentReadTimeout  time.Duration
}

func (h *Handler) SetPullOnConnect(enable bool) {
	h.pullOnConnect = enable
}

func NewHandler(database *db.DB) *Handler {
	// A typed nil database would satisfy the closer interface and panic on the
	// first disconnection, so the registry is given one only when it exists.
	var runs taskmcp.RunCloser
	var notes taskmcp.RunNoter
	if database != nil {
		runs, notes = database, database
	}
	h := &Handler{
		db:                database,
		subscribers:       make(map[chan Event]bool),
		agentDispatcher:   NewAgentDispatcher(),
		mcpSessions:       taskmcp.NewSessionRegistryWith(runs, notes, mcpSilenceNotice()),
		agentPingInterval: defaultAgentPingInterval,
		agentReadTimeout:  defaultAgentReadTimeout,
	}
	if database != nil {
		database.SetAgentOperations(h.agentDispatcher.CallOperation)
		database.RegisterPostBackListener(func(task *models.Task, activity *models.TaskActivity, err error) {
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			h.BroadcastEvent(Event{
				Type:     "task_updated",
				Task:     task,
				Activity: activity,
				Error:    errStr,
			})
		})
		database.OnRelayedEvent(h.deliverRelayedEvent)
	}
	return h
}

func (h *Handler) SubscribeEvents() chan Event {
	h.subMu.Lock()
	defer h.subMu.Unlock()
	ch := make(chan Event, 20)
	h.subscribers[ch] = true
	return ch
}

func (h *Handler) UnsubscribeEvents(ch chan Event) {
	h.subMu.Lock()
	defer h.subMu.Unlock()
	if _, ok := h.subscribers[ch]; ok {
		delete(h.subscribers, ch)
		close(ch)
	}
}

// localOnlyEvents are delivered to this instance's browsers and never relayed:
// terminal output is emitted per chunk, and nothing in another instance reads it.
var localOnlyEvents = map[string]bool{"agent_pty_output": true}

// BroadcastEvent delivers an event to this instance's browsers, then relays it
// to the other instances sharing the database, which deliver it to theirs.
func (h *Handler) BroadcastEvent(event Event) {
	h.broadcastLocal(event)
	if h.db == nil || localOnlyEvents[event.Type] {
		return
	}
	taskID, activityID := "", ""
	if event.Task != nil {
		taskID = event.Task.ID
	}
	if event.Activity != nil {
		activityID = event.Activity.ID
	}
	h.db.PublishEvent(event.Type, taskID, activityID, event.Error)
}

func (h *Handler) broadcastLocal(event Event) {
	h.subMu.RLock()
	defer h.subMu.RUnlock()
	for ch := range h.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

// deliverRelayedEvent turns what another instance published back into the
// event its browsers received, with the task and activity as they now stand,
// and delivers it here only: relaying it again would echo it between instances.
func (h *Handler) deliverRelayedEvent(msg db.BusMessage) {
	event := Event{Type: msg.Type, Error: msg.Error}
	if msg.TaskID != "" {
		if task, err := h.db.GetTaskByID(msg.TaskID); err == nil {
			event.Task = task
		}
	}
	if msg.ActivityID != "" {
		if activity, err := h.db.GetActivityByID(msg.ActivityID); err == nil {
			event.Activity = activity
		}
	}
	h.broadcastLocal(event)
}

// SetDataDir tells the handler where the application's own files live.
func (h *Handler) SetDataDir(dir string) {
	h.dataDir = dir
}

func (h *Handler) EnableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// describeActiveRun names the run that blocks a launch, and when it started,
// so the refusal says what is already happening rather than that something is.
func describeActiveRun(a *models.TaskActivity) string {
	name := a.SkillName
	if strings.TrimSpace(name) == "" {
		name = a.SkillID
	}
	// A queued run has not started, so it has no time to name.
	if a.Status == string(models.ActivityStatusQueued) || a.Status == string(models.ActivityStatusPending) {
		return fmt.Sprintf("A run of %s is queued on this task.", name)
	}
	started := "an unknown time"
	if a.StartedAt != nil {
		started = a.StartedAt.Format(time.RFC3339)
	}
	if a.WaitingSince != nil {
		return fmt.Sprintf("A run of %s started at %s is still active on this task, waiting for user input.", name, started)
	}
	return fmt.Sprintf("A run of %s started at %s is still active on this task.", name, started)
}

// writeTaskBusy answers a launch the database refused because the task already
// carries an active run, with the body the busy check answers, and reports
// whether it did. Any other error is left to the caller.
func writeTaskBusy(w http.ResponseWriter, err error) bool {
	var busy *db.TaskBusyError
	if !errors.As(err, &busy) {
		return false
	}
	body := map[string]string{"error": "Another run is active on this task."}
	if busy.Active != nil {
		body = map[string]string{"error": describeActiveRun(busy.Active), "activeRunId": busy.Active.ID}
	}
	writeJSON(w, http.StatusConflict, body)
	return true
}

func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "sectile-api"})
}

func (h *Handler) HandleCliStatus(w http.ResponseWriter, r *http.Request) {
	var result []models.CliStatus
	if err := h.db.AgentOperation(agentprotocol.Operation{ProjectID: r.URL.Query().Get("projectId"), Action: "cli_status"}, &result); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) HandleGitStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	target := r.URL.Query().Get("path")
	if target == "" {
		target = r.URL.Query().Get("projectId")
	}
	if target == "" {
		target = r.URL.Query().Get("repoPath")
	}

	status, err := h.db.GetGitStatus(target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) HandleGitBranches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	target := r.URL.Query().Get("path")
	if target == "" {
		target = r.URL.Query().Get("projectId")
	}
	if target == "" {
		target = r.URL.Query().Get("repoPath")
	}

	info, err := h.db.GetGitBranches(target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *Handler) HandleGitCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Branch    string `json:"branch"`
		Path      string `json:"path"`
		ProjectID string `json:"projectId"`
		Create    bool   `json:"create"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	target := req.Path
	if target == "" {
		target = req.ProjectID
	}
	if target == "" {
		target = r.URL.Query().Get("projectId")
	}

	status, err := h.db.SwitchGitBranch(target, req.Branch, req.Create)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Bascule effectuée sur la branche '%s'", req.Branch),
		"status":  status,
		"branch":  req.Branch,
	})
}

func (h *Handler) HandleGitCleanBranches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Path      string `json:"path"`
		ProjectID string `json:"projectId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	target := req.Path
	if target == "" {
		target = req.ProjectID
	}
	if target == "" {
		target = r.URL.Query().Get("projectId")
	}

	res, err := h.db.CleanAllLocalBranches(target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) HandleGitDeleteBranch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Branch       string `json:"branch"`
		Path         string `json:"path"`
		ProjectID    string `json:"projectId"`
		DeleteRemote bool   `json:"deleteRemote"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	branchName := req.Branch
	if branchName == "" {
		branchName = r.URL.Query().Get("branch")
	}
	if branchName == "" {
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) > 0 {
			last := pathParts[len(pathParts)-1]
			if last != "delete" && last != "branches" {
				branchName = last
			}
		}
	}

	if branchName == "" {
		writeError(w, http.StatusBadRequest, "Nom de branche requis")
		return
	}

	target := req.Path
	if target == "" {
		target = req.ProjectID
	}
	if target == "" {
		target = r.URL.Query().Get("projectId")
	}

	err := h.db.DeleteGitBranch(target, branchName, req.DeleteRemote)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"branch":  branchName,
		"message": fmt.Sprintf("Branche '%s' supprimée avec succès.", branchName),
	})
}

func (h *Handler) HandleSyncAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		ProjectID string `json:"projectId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	activity, err := h.db.EnqueueSyncAs(h.webSessionUser(r), "all", "", req.ProjectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":  "Synchronisation globale ajoutée à la file d'attente",
		"activity": activity,
	})
}

func (h *Handler) HandleSyncGithub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Repo      string `json:"repo"`
		ProjectID string `json:"projectId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	activity, err := h.db.EnqueueSyncAs(h.webSessionUser(r), "github", req.Repo, req.ProjectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":  "Synchronisation GitHub ajoutée à la file d'attente",
		"activity": activity,
	})
}

// HandleSyncJira queues a Jira synchronisation, like the GitHub one: the job
// resolves the project's adapter and imports its work items.
//
//	POST /api/sync/jira {projectKey, projectId}
func (h *Handler) HandleSyncJira(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req struct {
		ProjectKey string `json:"projectKey"`
		ProjectID  string `json:"projectId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	activity, err := h.db.EnqueueSyncAs(h.webSessionUser(r), "jira", req.ProjectKey, req.ProjectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":  "Synchronisation Jira ajoutée à la file d'attente",
		"activity": activity,
	})
}

// HandleSpecFrameworkStatus reports whether GitHub Spec Kit / OpenSpec are
// installed on the host and initialized in a project working directory.
// GET /api/spec-framework/status?projectId=…&repoPath=…&framework=speckit|openspec
func (h *Handler) HandleSpecFrameworkStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	target := r.URL.Query().Get("projectId")
	if target == "" {
		target = r.URL.Query().Get("repoPath")
	}

	statuses := h.db.GetSpecFrameworkStatus(target, r.URL.Query().Get("framework"))
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"frameworks": statuses,
	})
}

// HandleSpecFrameworkInstall bootstraps a Spec-Driven Design toolchain
// (GitHub Spec Kit or OpenSpec) in a project working directory.
// POST /api/spec-framework/install
func (h *Handler) HandleSpecFrameworkInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.SpecFrameworkInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requête JSON invalide: "+err.Error())
		return
	}

	res, err := h.db.InstallSpecFramework(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// The install itself may have failed while the request was well-formed; the
	// result carries the per-command detail, so return 200 with installed=false
	// rather than an opaque 500.
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) HandleSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	skills := h.db.GetAvailableSkills()
	writeJSON(w, http.StatusOK, skills)
}

func (h *Handler) HandleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		projects, err := h.db.GetProjectsForUser(h.webSessionUser(r))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, projects)

	case http.MethodPost:
		var req models.CreateProjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Payload invalide: "+err.Error())
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			writeError(w, http.StatusBadRequest, "Le nom du projet est obligatoire")
			return
		}
		if err := agentconfig.ValidModelConfig(agentconfig.ModelConfig{Model: req.AIModel, SkillModels: req.AISkillModels}); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// The creator owns the project: the background synchronisation has no
		// acting user of its own and reads under that account.
		userID := h.webSessionUser(r)
		project, err := h.db.CreateProjectAs(userID, req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if userID != "" && project != nil {
			_ = h.db.BookmarkProject(userID, project.ID)
			project.Bookmarked = true
		}
		writeJSON(w, http.StatusCreated, project)

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// isMacroSegment tells the path segment that introduces a macro sub-action.
//
// The two spellings are the same route. "epics" is what the tracker calls the
// container and what the URLs were written with; "macros" is what the product
// calls it and what the interface asks for. A sub-action that only answered one
// of them fell through to the generic macro handler, where "move" and
// "push-horizons" were read as a macro key: the call answered 200 and created a
// macro named after the action it was supposed to run.
func isMacroSegment(segment string) bool {
	return segment == "macros" || segment == "epics"
}

func (h *Handler) HandleProjectDetail(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	rawPath = strings.Trim(rawPath, "/")
	if rawPath == "" {
		writeError(w, http.StatusBadRequest, "ID du projet obligatoire")
		return
	}

	parts := strings.Split(rawPath, "/")
	id, err := url.PathUnescape(parts[0])
	if err != nil {
		id = parts[0]
	}

	// Sub-action: /api/projects/detected-statuses: live status detection for draft project
	if id == "detected-statuses" && r.Method == http.MethodGet {
		trackerName := r.URL.Query().Get("tracker")
		repo := r.URL.Query().Get("repo")
		repoPath := r.URL.Query().Get("repoPath")
		projID := r.URL.Query().Get("projectId")

		var statuses []string
		// columns is filled only for a saved project on a tracker with boards:
		// there, detection mirrors the board instead of inventing one column per
		// status. detectErr travels with the payload so a failed read is shown
		// rather than read as "no column".
		var columns []models.TrackerColumn
		detectErr := ""
		if projID != "" {
			var statusErr error
			statuses, statusErr = h.db.GetProjectTrackerStatuses(h.actingContext(r), projID)
			if statusErr != nil {
				detectErr = statusErr.Error()
			}
			cols, err := h.db.DetectProjectBoardColumns(h.actingContext(r), projID)
			switch {
			case tracker.IsUnsupported(err):
				// A tracker without boards keeps the historical payload.
			case err != nil:
				if detectErr == "" {
					detectErr = err.Error()
				}
			default:
				columns = cols
				// The palette must hold everything the board groups, even a
				// status the project status list did not return.
				seen := map[string]bool{}
				for _, st := range statuses {
					seen[strings.ToLower(st)] = true
				}
				for _, col := range cols {
					for _, st := range col.Statuses {
						if key := strings.ToLower(strings.TrimSpace(st)); key != "" && !seen[key] {
							seen[key] = true
							statuses = append(statuses, st)
						}
					}
				}
			}
		} else {
			dummyProj := &models.Project{
				IssueTracker: trackerName,
				GithubRepo:   repo,
				RepoPath:     repoPath,
			}
			// Temporary DB query for draft project
			_ = dummyProj
			// Query tracker HTTP metadata for a draft project
			seen := map[string]bool{}
			if trackerName == "github" {
				rRepo := models.CleanGithubRepo(repo)
				if rRepo != "" {

					parts := strings.Split(rRepo, "/")
					if len(parts) == 2 {
						gqlQuery, _ := trackerapi.GithubStatusQuery(rRepo)

						if output, err := h.db.TrackerGraphQL(h.actingContext(r), gqlQuery); err == nil {
							var gqlRes struct {
								Data struct {
									Repository struct {
										ProjectsV2 struct {
											Nodes []struct {
												Fields struct {
													Nodes []struct {
														Name    string `json:"name"`
														Options []struct {
															Name string `json:"name"`
														} `json:"options"`
													} `json:"nodes"`
												} `json:"fields"`
											} `json:"nodes"`
										} `json:"projectsV2"`
									} `json:"repository"`
									User struct {
										ProjectsV2 struct {
											Nodes []struct {
												Fields struct {
													Nodes []struct {
														Name    string `json:"name"`
														Options []struct {
															Name string `json:"name"`
														} `json:"options"`
													} `json:"nodes"`
												} `json:"fields"`
											} `json:"nodes"`
										} `json:"projectsV2"`
									} `json:"user"`
								} `json:"data"`
							}
							if json.Unmarshal(output, &gqlRes) == nil {
								allProjects := append(gqlRes.Data.Repository.ProjectsV2.Nodes, gqlRes.Data.User.ProjectsV2.Nodes...)
								for _, pNode := range allProjects {
									for _, fNode := range pNode.Fields.Nodes {
										if strings.EqualFold(fNode.Name, "Status") || strings.EqualFold(fNode.Name, "Statut") || len(fNode.Options) > 0 {
											for _, opt := range fNode.Options {
												name := strings.TrimSpace(opt.Name)
												if name != "" && !seen[strings.ToLower(name)] {
													seen[strings.ToLower(name)] = true
													statuses = append(statuses, name)
												}
											}
										}
									}
								}
							}
						}
					}
				}
				if len(statuses) == 0 {
					for _, s := range []string{"open", "closed"} {
						if !seen[s] {
							seen[s] = true
							statuses = append(statuses, s)
						}
					}
				}
			}
		}

		type StatusItem struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		var result []StatusItem
		for idx, s := range statuses {
			result = append(result, StatusItem{
				ID:   fmt.Sprintf("st-%d", idx),
				Name: s,
			})
		}
		payload := map[string]interface{}{"statuses": result}
		if columns != nil {
			payload["columns"] = columns
		}
		if detectErr != "" {
			payload["error"] = detectErr
		}
		writeJSON(w, http.StatusOK, payload)
		return
	}

	// Sub-action: /api/projects/{id}/epics/create: create an epic, the container
	// a split needs as a target
	if len(parts) >= 3 && isMacroSegment(parts[1]) && parts[2] == "create" && r.Method == http.MethodPost {
		var req struct {
			Title   string            `json:"title"`
			Horizon string            `json:"horizon"`
			Fields  map[string]string `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		meta, err := h.db.CreateEpic(id, req.Title, req.Horizon, req.Fields)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, meta)
		return
	}

	// Sub-action: /api/projects/{id}/epics/fields: ce que l'instance impose pour
	// créer un épic, au delà du titre. PE exige « Epic Type » et la création
	// échouait en 400 sans que l'interface puisse le demander.
	if len(parts) >= 3 && isMacroSegment(parts[1]) && parts[2] == "fields" && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, []string{})
		return
	}

	// Sub-action: /api/projects/{id}/epics/move: cut stories out of an epic into
	// another one, created on the fly when only a title is given
	if len(parts) >= 3 && isMacroSegment(parts[1]) && parts[2] == "move" && r.Method == http.MethodPost {
		var req struct {
			TaskIDs       []string          `json:"taskIds"`
			TargetEpicKey string            `json:"targetEpicKey"`
			NewEpicTitle  string            `json:"newEpicTitle"`
			Fields        map[string]string `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		activity, err := h.db.MoveTasksToEpic(h.actingContext(r), id, req.TaskIDs, req.TargetEpicKey, req.NewEpicTitle, req.Fields)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"queued":        true,
			"activity":      activity,
			"targetEpicKey": strings.ToUpper(strings.TrimSpace(req.TargetEpicKey)),
			"count":         len(req.TaskIDs),
		})
		return
	}

	// Sub-action: /api/projects/{id}/issue-types: the work item types the
	// project's tracker exposes, for the picker in the project settings.
	if len(parts) >= 2 && parts[1] == "issue-types" && r.Method == http.MethodGet {
		types, err := h.db.ListProjectIssueTypesAs(h.actingContext(r), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, types)
		return
	}

	// Sub-action: /api/projects/{id}/team-move: set the team of a batch of work
	// items, which is what triaging a backlog does.
	if len(parts) >= 2 && parts[1] == "team-move" && r.Method == http.MethodPost {
		var req struct {
			TaskIDs  []string `json:"taskIds"`
			TeamID   string   `json:"teamId"`
			TeamName string   `json:"teamName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		activity, err := h.db.SetTasksTeam(h.actingContext(r), id, req.TaskIDs, req.TeamID, req.TeamName)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"queued":   true,
			"activity": activity,
			"count":    len(req.TaskIDs),
		})
		return
	}

	// Sub-action: /api/projects/{id}/sprint-move: send a batch of work items to a
	// sprint, which is what planning from the roadmap does.
	if len(parts) >= 2 && parts[1] == "sprint-move" && r.Method == http.MethodPost {
		var req struct {
			TaskIDs    []string `json:"taskIds"`
			SprintID   string   `json:"sprintId"`
			SprintName string   `json:"sprintName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		activity, err := h.db.SetTasksSprint(h.actingContext(r), id, req.TaskIDs, req.SprintID, req.SprintName)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"queued":   true,
			"activity": activity,
			"count":    len(req.TaskIDs),
		})
		return
	}

	// Sub-action: /api/projects/{id}/epics/push-horizons: mirror the locally
	// classified epics whose Jira label is missing or stale
	if len(parts) >= 3 && isMacroSegment(parts[1]) && parts[2] == "push-horizons" {
		switch r.Method {
		case http.MethodGet:
			pending, err := h.db.PendingHorizonPushes(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, pending)
			return
		case http.MethodPost:
			activity, err := h.db.EnqueueTrackerOp(h.actingContext(r), db.TrackerOp{
				Kind:      db.TrackerOpPushHorizons,
				ProjectID: id,
			})
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusAccepted, map[string]interface{}{"queued": true, "activity": activity})
			return
		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
	}

	// Sub-action: /api/projects/{id}/macros/import-horizons: read the roadmap
	// labels back from the tracker, so a classification made there wins over
	// ours instead of being overwritten by the next push.
	//
	// Synchronous, unlike the push: it writes nothing on the tracker, and its
	// answer is the report the caller came for.
	if len(parts) >= 3 && isMacroSegment(parts[1]) && parts[2] == "import-horizons" && r.Method == http.MethodPost {
		note, err := h.db.ImportMacroHorizons(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"imported": true, "note": note})
		return
	}

	// Sub-action: /api/projects/{id}/macros/{key}/migrate: migrate macro and attached tasks to another project
	if len(parts) >= 4 && (parts[1] == "macros" || parts[1] == "epics") && parts[3] == "migrate" && r.Method == http.MethodPost {
		macroKey, err := url.PathUnescape(parts[2])
		if err != nil {
			macroKey = parts[2]
		}
		var req struct {
			TargetProjectID string `json:"targetProjectId"`
			MigrateTasks    bool   `json:"migrateTasks"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		meta, count, err := h.db.MigrateMacro(id, macroKey, req.TargetProjectID, req.MigrateTasks)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"macro":           meta,
			"epic":            meta,
			"migratedTasks":   count,
			"targetProjectId": req.TargetProjectID,
		})
		return
	}

	// Sub-action: /api/projects/{id}/macros/{key}/story: turn a shaping todo into
	// a real story under that macro
	if len(parts) >= 4 && (parts[1] == "macros" || parts[1] == "epics") && parts[3] == "story" && r.Method == http.MethodPost {
		macroKey, err := url.PathUnescape(parts[2])
		if err != nil {
			macroKey = parts[2]
		}
		var req struct {
			TodoID string `json:"todoId"`
			Title  string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		if strings.TrimSpace(req.TodoID) == "" {
			task, err := h.db.CreateStoryUnderMacro(id, macroKey, req.Title)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"task": task, "storyKey": task.Key})
			return
		}
		meta, task, err := h.db.CreateStoryFromMacroTodo(id, macroKey, req.TodoID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"macro": meta, "epic": meta, "storyKey": task.Key, "task": task})
		return
	}

	// Sub-action: /api/projects/{id}/macros: the macro metadata Sectile owns:
	// horizon (NOW / NEXT / LATER), shaping notes and todos.
	if len(parts) >= 2 && (parts[1] == "macros" || parts[1] == "epics") {
		// Creation: /api/projects/{id}/macros/create
		if len(parts) >= 3 && parts[2] == "create" && r.Method == http.MethodPost {
			var req struct {
				Title   string            `json:"title"`
				Horizon string            `json:"horizon"`
				Fields  map[string]string `json:"fields,omitempty"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "Invalid macro payload: "+err.Error())
				return
			}
			created, err := h.db.CreateMacro(id, req.Title, req.Horizon, req.Fields)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, created)
			return
		}

		// Refinement: /api/projects/{id}/macros/{key}/refine
		if len(parts) >= 4 && parts[3] == "refine" && r.Method == http.MethodPost {
			key := parts[2]
			if decoded, err := url.PathUnescape(parts[2]); err == nil {
				key = decoded
			}
			todos, proposed, framework, err := h.db.RefineMacro(id, key)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"key":           key,
				"todos":         todos,
				"proposedTasks": proposed,
				"specFramework": framework,
			})
			return
		}

		// Slicing: /api/projects/{id}/macros/{key}/slicing produces the macro's
		// todo lines from the SDD artefacts of the project's repository.
		//
		// Rien n'est écrit dans le dépôt ni sur le tracker, et aucune story
		// n'est créée : c'est une lecture, et la découpe reste modifiable.
		if len(parts) >= 4 && parts[3] == "slicing" && r.Method == http.MethodPost {
			key := parts[2]
			if decoded, err := url.PathUnescape(parts[2]); err == nil {
				key = decoded
			}
			var req struct {
				Source string `json:"source"`
			}
			// Un corps absent vaut la source par défaut : le geste courant ne
			// doit pas exiger une charge utile pour être appelable.
			_ = json.NewDecoder(r.Body).Decode(&req)
			// Les stories déjà créées ne sont pas une source de fichier : elles
			// sont routées avant la normalisation, qui ne connaît que le dépôt
			// et ferait retomber « stories » sur tasks.md en silence.
			var meta *models.MacroMeta
			var origin string
			var err error
			if strings.EqualFold(strings.TrimSpace(req.Source), models.MacroTodoFromStories) {
				meta, origin, err = h.db.TodosFromMacroStories(id, key)
			} else {
				meta, origin, err = h.db.TodosFromSDD(id, key, db.NormalizeSlicingSource(req.Source))
			}
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"macro":  meta,
				"epic":   meta,
				"origin": origin,
			})
			return
		}

		switch r.Method {
		case http.MethodGet:
			macros, err := h.db.GetProjectMacros(id)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, macros)
			return
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			var req struct {
				Key            string              `json:"key"`
				Title          *string             `json:"title,omitempty"`
				Horizon        *string             `json:"horizon,omitempty"`
				Description    *string             `json:"description,omitempty"`
				FramingComment *string             `json:"framingComment,omitempty"`
				Todos          *[]models.MacroTodo `json:"todos,omitempty"`
				Closed         *bool               `json:"closed,omitempty"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "Invalid macro payload: "+err.Error())
				return
			}
			key := req.Key
			if len(parts) >= 3 && parts[2] != "" {
				if decoded, err := url.PathUnescape(parts[2]); err == nil {
					key = decoded
				}
			}
			saved, err := h.db.UpdateMacro(id, key, req.Title, req.Horizon, req.Description, req.FramingComment, req.Todos, req.Closed)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			labelNote := ""
			if req.Horizon != nil {
				labelNote = "label roadmap en file d'attente"
				if _, err := h.db.EnqueueTrackerOp(h.actingContext(r), db.TrackerOp{
					Kind:      db.TrackerOpEpicHorizon,
					ProjectID: id,
					TaskKey:   key,
					EpicKey:   key,
					Horizon:   saved.Horizon,
				}); err != nil {
					labelNote = "label roadmap non mis en file : " + err.Error()
					log.Printf("[macros] label roadmap non mis en file pour %s: %v", key, err)
				}
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"macro": saved, "epic": saved, "labelNote": labelNote})
			return
		case http.MethodDelete:
			key := ""
			if len(parts) >= 3 && parts[2] != "" {
				if decoded, err := url.PathUnescape(parts[2]); err == nil {
					key = decoded
				}
			}
			if key == "" {
				writeError(w, http.StatusBadRequest, "Clé de macro obligatoire")
				return
			}
			if err := h.db.DeleteMacro(id, key); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"status": "deleted", "key": key})
			return
		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
	}

	// Sub-action: /api/projects/{id}/boards: the tracker's boards, for the picker
	if len(parts) >= 2 && parts[1] == "boards" && r.Method == http.MethodGet {
		boards, err := h.db.ListProjectTrackerBoardsAs(h.actingContext(r), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, boards)
		return
	}

	// Sub-action: /api/projects/{id}/board-columns: import the columns of a
	// tracker board as a starting point for the project's own columns
	if len(parts) >= 2 && parts[1] == "board-columns" && r.Method == http.MethodPost {
		var req struct {
			BoardID string `json:"boardId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		proj, err := h.db.ImportProjectBoardColumns(h.actingContext(r), id, req.BoardID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, proj)
		return
	}

	// Sub-action: /api/projects/{id}/tracker-statuses: the statuses actually
	// seen on this project's tickets, to assign them to columns
	if len(parts) >= 2 && parts[1] == "tracker-statuses" && r.Method == http.MethodGet {
		statuses, err := h.db.GetProjectTrackerStatuses(h.actingContext(r), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, statuses)
		return
	}

	// Sub-action: /api/projects/{id}/skills-status, /api/projects/{id}/skills, or query repoPath
	if (len(parts) >= 2 && (parts[1] == "skills-status" || parts[1] == "skills")) || id == "skills-status" || id == "skills" {
		target := id
		if len(parts) >= 2 {
			target = id
		} else if qPath := r.URL.Query().Get("repoPath"); qPath != "" {
			target = qPath
		}
		status, err := h.db.GetProjectSkillsStatus(target)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, status)
		return
	}

	// Sub-action: /api/projects/{id}/skill-editor
	//   GET                          → the five workflow skills, content included
	//   PUT    /{skillId}            → save the edited content and regenerate the files
	//   POST   /{skillId}/reset      → back to the built-in template
	//   POST   /{skillId}/import     → take the file on disk as the new content
	//   PUT    /{skillId}/mode       → pin the skill's execution mode, or clear it
	if len(parts) >= 2 && parts[1] == "skill-editor" {
		skillID := ""
		if len(parts) >= 3 {
			skillID = parts[2]
		}
		sub := ""
		if len(parts) >= 4 {
			sub = parts[3]
		}

		switch {
		case r.Method == http.MethodGet && skillID == "":
			entries, err := h.db.ListProjectSkillEditor(id)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, entries)
			return

		case r.Method == http.MethodPut && skillID != "":
			var payload struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
				return
			}
			entry, err := h.db.SaveProjectSkillContent(id, skillID, payload.Content)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, entry)
			return

		case r.Method == http.MethodPost && skillID != "" && sub == "reset":
			entry, err := h.db.ResetProjectSkillContent(id, skillID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, entry)
			return

		case r.Method == http.MethodPut && skillID != "" && sub == "mode":
			var payload struct {
				Mode string `json:"mode"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
				return
			}
			if err := h.db.SetProjectSkillMode(id, skillID, payload.Mode); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			entries, err := h.db.ListProjectSkillEditor(id)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			for _, entry := range entries {
				if entry.ID == models.NormalizeSkillID(skillID) {
					writeJSON(w, http.StatusOK, entry)
					return
				}
			}
			writeError(w, http.StatusNotFound, "skill introuvable après enregistrement")
			return

		case r.Method == http.MethodPost && skillID != "" && sub == "import":
			entry, err := h.db.ImportProjectSkillFromRepo(id, skillID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, entry)
			return
		}

		writeError(w, http.StatusMethodNotAllowed, "Action non supportée sur skill-editor")
		return
	}

	// Sub-action: /api/projects/{id}/install-skills or /api/projects/install-skills
	if ((len(parts) >= 2 && parts[1] == "install-skills") || id == "install-skills") && r.Method == http.MethodPost {
		target := id
		var payload struct {
			RepoPath          string `json:"repoPath"`
			ProjectID         string `json:"projectId"`
			SpecFramework     string `json:"specFramework"`
			AIProvider        string `json:"aiProvider"`
			AICommandTemplate string `json:"aiCommandTemplate"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload.RepoPath != "" {
			target = payload.RepoPath
		} else if payload.ProjectID != "" {
			target = payload.ProjectID
		}

		status, err := h.db.InstallProjectSkills(target, payload.SpecFramework, payload.AIProvider, payload.AICommandTemplate)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message": "Skills IA installées avec succès dans le projet",
			"status":  status,
		})
		return
	}

	// Sub-action: /api/projects/{id}/spec-framework-status
	if len(parts) >= 2 && parts[1] == "spec-framework-status" {
		statuses := h.db.GetSpecFrameworkStatus(id, r.URL.Query().Get("framework"))
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"frameworks": statuses,
		})
		return
	}

	// Sub-action: /api/projects/{id}/install-spec-framework
	if len(parts) >= 2 && parts[1] == "install-spec-framework" && r.Method == http.MethodPost {
		var payload models.SpecFrameworkInstallRequest
		_ = json.NewDecoder(r.Body).Decode(&payload)
		payload.ProjectID = id

		res, err := h.db.InstallSpecFramework(payload)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, res)
		return
	}

	// Sub-action: /api/projects/{id}/init-git or /api/projects/init-git
	if ((len(parts) >= 2 && parts[1] == "init-git") || id == "init-git") && r.Method == http.MethodPost {
		target := id
		var payload struct {
			RepoPath  string `json:"repoPath"`
			ProjectID string `json:"projectId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload.RepoPath != "" {
			target = payload.RepoPath
		} else if payload.ProjectID != "" {
			target = payload.ProjectID
		}

		res, err := h.db.InitProjectGit(target)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, res)
		return
	}

	// Sub-action: /api/projects/{id}/detect-statuses or /api/projects/detect-statuses
	if (len(parts) >= 2 && parts[1] == "detect-statuses") || id == "detect-statuses" {
		target := id
		if len(parts) >= 2 {
			target = id
		}
		tracker := r.URL.Query().Get("tracker")
		repo := r.URL.Query().Get("repo")

		if r.Method == http.MethodPost {
			var body struct {
				ProjectID    string `json:"projectId"`
				IssueTracker string `json:"issueTracker"`
				GithubRepo   string `json:"githubRepo"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				if body.ProjectID != "" {
					target = body.ProjectID
				}
				if body.IssueTracker != "" {
					tracker = body.IssueTracker
				}
				if body.GithubRepo != "" {
					repo = body.GithubRepo
				}
			}
		}

		// `tracker` is the tracker name here, so the package is spelled out.
		statuses, err := h.db.DetectTrackerStatuses(h.actingContext(r), target, tracker, repo)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, statuses)
		return
	}

	switch r.Method {
	case http.MethodGet:
		project, err := h.db.GetProjectByID(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if project == nil {
			writeError(w, http.StatusNotFound, "Projet non trouvé")
			return
		}
		writeJSON(w, http.StatusOK, project)

	case http.MethodPut, http.MethodPatch:
		var req models.UpdateProjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Payload invalide: "+err.Error())
			return
		}
		var requested agentconfig.ModelConfig
		if req.AIModel != nil {
			requested.Model = *req.AIModel
		}
		if req.AISkillModels != nil {
			requested.SkillModels = *req.AISkillModels
		}
		if err := agentconfig.ValidModelConfig(requested); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Saving an ownerless project adopts the person saving it, so its
		// background synchronisation stops running as the server.
		project, err := h.db.UpdateProjectAs(h.webSessionUser(r), id, req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, project)

	case http.MethodDelete:
		if err := h.db.DeleteProject(id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Projet supprimé avec succès"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *Handler) HandleTasks(w http.ResponseWriter, r *http.Request) {
	if (r.URL.Path == "/api/tasks/postback" || strings.HasSuffix(r.URL.Path, "/postback")) && r.Method == http.MethodPost {
		h.HandleTaskPostBack(w, r)
		return
	}

	if (r.URL.Path == "/api/tasks/stage" || r.URL.Path == "/api/tasks/transition") && r.Method == http.MethodPost {
		var req struct {
			TaskID  string `json:"taskId"`
			TaskKey string `json:"taskKey"`
			ID      string `json:"id"`
			Key     string `json:"key"`
			Stage   string `json:"stage"`
			Label   string `json:"label"`
			Note    string `json:"note"`
			Comment string `json:"comment"`
			PrURL   string `json:"prUrl"`
			Branch  string `json:"branch"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		targetID := req.TaskID
		if targetID == "" {
			targetID = req.TaskKey
		}
		if targetID == "" {
			targetID = req.ID
		}
		if targetID == "" {
			targetID = req.Key
		}
		if targetID == "" {
			writeError(w, http.StatusBadRequest, "Paramètre 'taskId' ou 'taskKey' manquant")
			return
		}
		stage := req.Stage
		if stage == "" {
			stage = req.Label
		}
		if strings.TrimSpace(stage) == "" {
			writeError(w, http.StatusBadRequest, "Paramètre 'stage' manquant (ex: 'clarified', 'specified', 'implemented', 'reviewed', 'finished')")
			return
		}
		note := req.Note
		if note == "" {
			note = req.Comment
		}
		task, act, err := h.db.TransitionTaskStageBy(h.webSessionUser(r), targetID, stage, note, req.PrURL, req.Branch)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success":  true,
			"message":  fmt.Sprintf("Tâche %s passée à l'étape « %s »", task.Key, stage),
			"task":     task,
			"activity": act,
		})
		return
	}

	if r.URL.Path == "/api/tasks/migrate" && r.Method == http.MethodPost {
		var req struct {
			TaskIDs         []string `json:"taskIds"`
			TargetProjectID string   `json:"targetProjectId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		count, err := h.db.MigrateTasks(req.TaskIDs, req.TargetProjectID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"migratedCount":   count,
			"targetProjectId": req.TargetProjectID,
		})
		return
	}

	if r.URL.Path == "/api/tasks/batch" && r.Method == http.MethodPost {
		var reqs []models.CreateTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid batch payload: "+err.Error())
			return
		}
		p := h.webPrincipal(r)
		var defaultCreator, defaultAvatar string
		if !p.Anonymous() {
			defaultCreator = p.Name
			if defaultCreator == "" {
				defaultCreator = p.UserID
			}
			if settings, err := h.db.UserSettings(p.UserID); err == nil && settings != nil {
				if strings.TrimSpace(settings.UserName) != "" && defaultCreator == p.UserID {
					defaultCreator = settings.UserName
				}
				defaultAvatar = settings.UserAvatar
			}
		}
		created := make([]models.Task, 0, len(reqs))
		for _, req := range reqs {
			if strings.TrimSpace(req.Title) == "" {
				continue
			}
			if !p.Anonymous() {
				if req.Creator == "" {
					req.Creator = defaultCreator
				}
				if req.CreatorAvatar == "" {
					req.CreatorAvatar = defaultAvatar
				}
			}
			t, err := h.db.CreateTaskAs(h.actingContext(r), req)
			if err == nil && t != nil {
				created = append(created, *t)
			}
		}
		writeJSON(w, http.StatusCreated, created)
		return
	}

	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query().Get("q")
		status := r.URL.Query().Get("status")
		priority := r.URL.Query().Get("priority")
		label := r.URL.Query().Get("label")
		projectID := r.URL.Query().Get("projectId")
		sprint := r.URL.Query().Get("sprint")
		team := r.URL.Query().Get("team")
		assignee := r.URL.Query().Get("assignee")
		macro := r.URL.Query().Get("macro")
		if macro == "" {
			macro = r.URL.Query().Get("epic")
		}
		// Statuts du tracker à afficher, répétables : ?trackerStatus=Draft&trackerStatus=Selected
		trackerStatuses := r.URL.Query()["trackerStatus"]
		// Types de tickets à afficher, répétables : ?issueType=Bug&issueType=Story
		issueTypes := r.URL.Query()["issueType"]
		// pinned=1 : les seuls tickets épinglés, le raccourci vers les chantiers
		// en cours quand le board en porte trois cents.
		pinnedOnly := r.URL.Query().Get("pinned") == "1" || r.URL.Query().Get("pinned") == "true"

		// viewId: a saved view replaces the project with its own selection,
		// and the other filters narrow it further (#387).
		scope := db.TaskScope{UserID: h.webSessionUser(r), ProjectID: projectID, ViewID: r.URL.Query().Get("viewId")}
		tasks, err := h.db.GetTasksInScope(scope, q, status, priority, label, sprint, team, assignee, macro, trackerStatuses, issueTypes, pinnedOnly)
		if errors.Is(err, db.ErrBoardViewNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, tasks)

	case http.MethodPost:
		var req models.CreateTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
			return
		}
		if strings.TrimSpace(req.Title) == "" {
			writeError(w, http.StatusBadRequest, "Task title is required")
			return
		}

		if p := h.webPrincipal(r); !p.Anonymous() {
			creator := p.Name
			if creator == "" {
				creator = p.UserID
			}
			var avatar string
			if settings, err := h.db.UserSettings(p.UserID); err == nil && settings != nil {
				if strings.TrimSpace(settings.UserName) != "" && creator == p.UserID {
					creator = settings.UserName
				}
				avatar = settings.UserAvatar
			}
			if req.Creator == "" {
				req.Creator = creator
			}
			if req.CreatorAvatar == "" {
				req.CreatorAvatar = avatar
			}
		}

		// The creation carries whoever asked for it, so a tracker that
		// attributes it to an account uses theirs when they stored one.
		task, err := h.db.CreateTaskAs(h.actingContext(r), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, task)

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// HandleTrackerSetup checks tracker credentials, and saves them once they are
// known good.
//
//	POST /api/setup/tracker/check  {tracker, siteUrl, project, email, token}
//	POST /api/setup/tracker        {tracker, siteUrl, project, email, token, storeTokenInFile}
//
// Checking before saving is the point: a wrong site or a stale token never
// reaches the settings, and the answer names what is wrong instead of leaving a
// sync to fail later with nothing to show. `tracker` selects the fields that
// matter: GitHub, GitLab and Jira are checked against the instance and
// persisted here. storeTokenInFile is accepted for older clients and ignored:
// no file store exists, the token goes to the user configuration.
func (h *Handler) HandleTrackerSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Tracker          string `json:"tracker"`
		SiteURL          string `json:"siteUrl"`
		Project          string `json:"project"`
		Email            string `json:"email"`
		Token            string `json:"token"`
		StoreTokenInFile bool   `json:"storeTokenInFile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
		return
	}

	trackerName := strings.ToLower(strings.TrimSpace(req.Tracker))
	checkOnly := strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/check")
	verified := ""
	if trackerName == "github" || trackerName == "gitlab" || trackerName == "jira" {
		// A check is the one call somebody waits in front of, so how long it
		// actually took is worth knowing: it separates a slow instance from a
		// slow screen, which look identical from a chair.
		started := time.Now()
		account, err := h.db.CheckTrackerCredentials(h.actingContext(r), trackerName, req.SiteURL, req.Email, req.Token)
		log.Printf("[TrackerSetup] vérification %s en %s", trackerName, time.Since(started).Round(time.Millisecond))
		if err != nil {
			// Nothing is persisted on a failed check: the user configuration
			// keeps the parameters that were working.
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		verified = account
	}

	if checkOnly {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "account": verified})
		return
	}

	if trackerName == "github" || trackerName == "gitlab" || trackerName == "jira" {
		settings, err := h.db.SaveTrackerCredentials(trackerName, req.SiteURL, req.Project, req.Email, req.Token)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, settings)
		return
	}

	settings, err := h.db.GetSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// HandleAutoSyncStatus reports what the background sync loop has been doing:
// whether it runs, when it last ran, how much it actually imported, and whether
// the tracker asked it to step back. Switching it on or off goes through the
// settings, like every other preference.
func (h *Handler) HandleAutoSyncStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, h.db.AutoSyncStatus())
}

// HandleTaskFacets serves the distinct sprint and team values present on the
// board, so the UI shows those filters only for trackers that feed them.
func (h *Handler) HandleTaskFacets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	facets, err := h.db.GetTaskFacetsInScope(db.TaskScope{
		UserID:    h.webSessionUser(r),
		ProjectID: r.URL.Query().Get("projectId"),
		ViewID:    r.URL.Query().Get("viewId"),
	})
	if errors.Is(err, db.ErrBoardViewNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, facets)
}

// HandleTeams serves the teams carried by a project's work items and the people
// in them. The team is optional on a work item, so an empty list is a normal
// answer, not an error: the UI then simply shows no team filter.
//
//	GET  /api/teams?projectId=&members=1   the project's teams
//	GET  /api/teams/search?projectId=&q=   lookup of the instance's teams, by name
//	GET  /api/teams/members?team=<name>    the people of one team, by its label
//	GET  /api/teams/workload?projectId=&team=<name>
//	POST /api/teams/refresh                {projectId, teamId} re-read from Jira
func (h *Handler) HandleTeams(w http.ResponseWriter, r *http.Request) {
	sub := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/teams"), "/")

	switch sub {
	case "":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		withMembers := r.URL.Query().Get("members") == "1" || r.URL.Query().Get("members") == "true"
		teams, err := h.db.ListProjectTeams(r.URL.Query().Get("projectId"), withMembers)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, teams)

	case "search":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		teams, err := h.db.SearchTrackerTeamsAs(h.actingContext(r), r.URL.Query().Get("projectId"), r.URL.Query().Get("q"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, teams)

	case "members":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		members, err := h.db.MembersForTeamName(r.URL.Query().Get("team"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, members)

	case "workload":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		team := strings.TrimSpace(r.URL.Query().Get("team"))
		if team == "" {
			writeError(w, http.StatusBadRequest, "Le nom de l'équipe est requis")
			return
		}
		load, err := h.db.GetTeamWorkload(r.URL.Query().Get("projectId"), team)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, load)

	case "refresh":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		var req struct {
			ProjectID string `json:"projectId"`
			TeamID    string `json:"teamId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		if strings.TrimSpace(req.TeamID) == "" {
			writeError(w, http.StatusBadRequest, "L'identifiant de l'équipe est requis : il n'arrive qu'avec une synchronisation Jira")
			return
		}
		team, err := h.db.RefreshTeamMembersNowAs(h.actingContext(r), req.ProjectID, req.TeamID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, team)

	default:
		writeError(w, http.StatusNotFound, "Unknown teams endpoint")
	}
}

func (h *Handler) HandleTaskDetail(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	rawPath = strings.Trim(rawPath, "/")
	if rawPath == "" {
		writeError(w, http.StatusBadRequest, "Task ID is required")
		return
	}

	var subAction string
	var id string

	switch {
	case strings.HasSuffix(rawPath, "/cancel-run"):
		subAction = "cancel-run"
		id = strings.TrimSuffix(rawPath, "/cancel-run")
	case strings.HasSuffix(rawPath, "/run-skill"):
		subAction = "run-skill"
		id = strings.TrimSuffix(rawPath, "/run-skill")
	case strings.HasSuffix(rawPath, "/move"):
		subAction = "move"
		id = strings.TrimSuffix(rawPath, "/move")
	case strings.HasSuffix(rawPath, "/activities"):
		subAction = "activities"
		id = strings.TrimSuffix(rawPath, "/activities")
	case strings.HasSuffix(rawPath, "/comment"):
		subAction = "comment"
		id = strings.TrimSuffix(rawPath, "/comment")
	case strings.HasSuffix(rawPath, "/convert"):
		subAction = "convert"
		id = strings.TrimSuffix(rawPath, "/convert")
	case strings.HasSuffix(rawPath, "/git-diff"):
		subAction = "git-diff"
		id = strings.TrimSuffix(rawPath, "/git-diff")
	case strings.HasSuffix(rawPath, "/diff"):
		subAction = "git-diff"
		id = strings.TrimSuffix(rawPath, "/diff")
	case strings.HasSuffix(rawPath, "/checkout-branch"):
		subAction = "checkout-branch"
		id = strings.TrimSuffix(rawPath, "/checkout-branch")
	case strings.HasSuffix(rawPath, "/checkout"):
		subAction = "checkout-branch"
		id = strings.TrimSuffix(rawPath, "/checkout")
	case strings.HasSuffix(rawPath, "/worktree"):
		subAction = "worktree"
		id = strings.TrimSuffix(rawPath, "/worktree")
	case strings.HasSuffix(rawPath, "/pin"):
		subAction = "pin"
		id = strings.TrimSuffix(rawPath, "/pin")
	case strings.HasSuffix(rawPath, "/sync"):
		subAction = "sync"
		id = strings.TrimSuffix(rawPath, "/sync")
	case strings.HasSuffix(rawPath, "/clone"):
		subAction = "clone"
		id = strings.TrimSuffix(rawPath, "/clone")
	case strings.HasSuffix(rawPath, "/duplicate"):
		subAction = "clone"
		id = strings.TrimSuffix(rawPath, "/duplicate")
	case strings.HasSuffix(rawPath, "/migrate"):
		subAction = "migrate"
		id = strings.TrimSuffix(rawPath, "/migrate")
	case strings.HasSuffix(rawPath, "/tty-agent"):
		subAction = "tty-agent"
		id = strings.TrimSuffix(rawPath, "/tty-agent")
	case strings.HasSuffix(rawPath, "/tty-skill"):
		subAction = "tty-skill"
		id = strings.TrimSuffix(rawPath, "/tty-skill")
	case strings.HasSuffix(rawPath, "/advance/confirm"):
		subAction = "advance-confirm"
		id = strings.TrimSuffix(rawPath, "/advance/confirm")
	case strings.HasSuffix(rawPath, "/advance"):
		subAction = "advance"
		id = strings.TrimSuffix(rawPath, "/advance")
	case rawPath == "stage" || strings.HasSuffix(rawPath, "/stage"):
		subAction = "stage"
		id = strings.TrimSuffix(strings.TrimSuffix(rawPath, "/stage"), "stage")
	case rawPath == "transition" || strings.HasSuffix(rawPath, "/transition"):
		subAction = "stage"
		id = strings.TrimSuffix(strings.TrimSuffix(rawPath, "/transition"), "transition")
	case rawPath == "workflow-label" || strings.HasSuffix(rawPath, "/workflow-label"):
		subAction = "stage"
		id = strings.TrimSuffix(strings.TrimSuffix(rawPath, "/workflow-label"), "workflow-label")
	case strings.HasSuffix(rawPath, "/macro"):
		subAction = "macro"
		id = strings.TrimSuffix(rawPath, "/macro")
	case strings.HasSuffix(rawPath, "/epic"):
		subAction = "macro"
		id = strings.TrimSuffix(rawPath, "/epic")
	case strings.HasSuffix(rawPath, "/team"):
		subAction = "team"
		id = strings.TrimSuffix(rawPath, "/team")
	case strings.HasSuffix(rawPath, "/sprint"):
		subAction = "sprint"
		id = strings.TrimSuffix(rawPath, "/sprint")
	case strings.HasSuffix(rawPath, "/assignable"):
		subAction = "assignable"
		id = strings.TrimSuffix(rawPath, "/assignable")
	case strings.HasSuffix(rawPath, "/comments"):
		subAction = "comments"
		id = strings.TrimSuffix(rawPath, "/comments")
	case strings.HasSuffix(rawPath, "/tracker-status"):
		subAction = "tracker-status"
		id = strings.TrimSuffix(rawPath, "/tracker-status")
	case strings.HasSuffix(rawPath, "/messages"):
		subAction = "messages"
		id = strings.TrimSuffix(rawPath, "/messages")
	case strings.HasSuffix(rawPath, "/chat/stream"):
		subAction = "chat"
		id = strings.TrimSuffix(rawPath, "/chat/stream")
	case strings.HasSuffix(rawPath, "/chat"):
		subAction = "chat"
		id = strings.TrimSuffix(rawPath, "/chat")
	case strings.HasSuffix(rawPath, "/postback"):
		subAction = "postback"
		id = strings.TrimSuffix(rawPath, "/postback")
	default:
		id = rawPath
	}

	if unescaped, err := url.PathUnescape(id); err == nil && unescaped != "" {
		id = unescaped
	}
	id = strings.TrimSpace(id)

	if subAction == "postback" && r.Method == http.MethodPost {
		h.HandleTaskPostBack(w, r)
		return
	}

	if subAction == "cancel-run" && r.Method == http.MethodPost {
		h.handleCancelRemoteRun(w, r, id)
		return
	}

	// Sub-action: /api/tasks/{id}/move
	if subAction == "move" && (r.Method == http.MethodPatch || r.Method == http.MethodPost) {
		var req models.MoveTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid move payload: "+err.Error())
			return
		}
		task, err := h.db.MoveTask(id, req.Status, req.Position)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, task)
		return
	}

	// Sub-action: /api/tasks/{id}/run-skill
	if subAction == "run-skill" && r.Method == http.MethodPost {
		var req models.RunSkillRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid skill request: "+err.Error())
			return
		}
		if req.SkillID == "" {
			writeError(w, http.StatusBadRequest, "Skill ID is required")
			return
		}
		// An absent mode means "no override", which is not the same as
		// interactive: the precedence still falls through to the skill and then
		// to the project. Anything else is a client mistake, not a fallback.
		if !models.ValidSkillMode(req.Mode) {
			writeError(w, http.StatusBadRequest, "mode invalide : "+req.Mode)
			return
		}
		// The model is checked here as well as on the agent: it is placed on a
		// command line run through sh -c, so neither side takes the other's word.
		if err := agentconfig.ValidModel(req.Model); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if req.WithComments || strings.Contains(req.Prompt, "--with-comments") {
			comments, err := h.db.GetTaskCommentsAs(h.actingContext(r), id)
			if err == nil && len(comments) > 0 {
				var commentStr strings.Builder
				commentStr.WriteString("\n\n---\nTask Comments Context:\n")
				for i, c := range comments {
					commentStr.WriteString(fmt.Sprintf("%d. [%s]: %s\n", i+1, c.Author, c.Body))
				}
				req.Prompt = strings.TrimSpace(req.Prompt + commentStr.String())
			}
		}

		task, err := h.db.GetTaskByID(id)
		if err != nil || task == nil {
			writeError(w, http.StatusNotFound, "Task not found")
			return
		}

		// A task already carrying an active run is busy: a second launch would
		// start an agent in parallel on the same work. The check happens before
		// anything is recorded, so a refused launch leaves no trace at all.
		active, activeErr := h.db.ActiveRunOnTask(task.ID)
		if activeErr != nil {
			writeError(w, http.StatusInternalServerError, "Cannot check the task for an active run")
			return
		}
		if active != nil {
			if !req.Force {
				writeJSON(w, http.StatusConflict, map[string]string{
					"error":       describeActiveRun(active),
					"activeRunId": active.ID,
				})
				return
			}
			// Forcing steps over another session's run, so it follows the rule
			// cancel-run enforces: the owner, or an admin. The active run is
			// left exactly as it is; force is not a cancellation.
			if _, ok := h.requireOwnerOrAdmin(w, r, active.UserID); !ok {
				return
			}
		}

		projectID := "default"
		if task.ProjectID != "" {
			projectID = task.ProjectID
		}
		userID := h.webSessionUser(r)
		ac := h.agentDispatcher.Route(userID, projectID)

		// 1. If a local agent daemon is connected, delegate the execution directly to it!
		if ac != nil {
			log.Printf("🚀 [Dispatch] Delegating skill %s on task %s (%s) to connected local agent (device=%s)", req.SkillID, task.Key, task.ID, ac.DeviceID)

			// The mode is resolved before the run is recorded: it is what tells,
			// once the run is over, whether anything was supposed to come back
			// from it without a user closing a session.
			mode := h.db.ResolveTaskSkillMode(projectID, req.SkillID, req.Mode)
			// The engine the server resolves is what the run shows until the
			// agent reports the one it really built its command line with.
			provider, model := h.db.ResolveTaskEngine(projectID, req.SkillID, req.Model)
			// The run is recorded before the launch record, and its insert is the
			// busy check that holds across server instances: a launch that lost
			// the race to another one is refused here and leaves no trace.
			// "Launch anyway" records a concurrent run, which the database lets
			// sit next to the active one.
			remoteRun, runErr := h.db.StartAgentRun(task.ID, req.SkillID, db.RunLaunch{Mode: mode, Provider: provider, Model: model, UserID: userID, Force: req.Force})
			if writeTaskBusy(w, runErr) {
				return
			}

			activityID := uuid.New().String()
			now := time.Now()
			act := models.TaskActivity{
				ID:        activityID,
				TaskID:    task.ID,
				SkillID:   "agent_launch",
				SkillName: req.SkillID,
				Action:    fmt.Sprintf("Exécution de %s sur l'agent local", req.SkillID),
				Status:    string(models.ActivityStatusRunning),
				Summary:   fmt.Sprintf("Exécution en cours sur l'agent local (%s)", ac.DeviceID),
				Output:    "",
				Steps: []string{
					fmt.Sprintf("Tâche ciblée : %s - %s", task.Key, task.Title),
					fmt.Sprintf("Déléguée à l'agent local (%s)...", ac.DeviceID),
				},
				Prompt:    req.Prompt,
				CreatedAt: now,
				StartedAt: &now,
				UserID:    userID,
			}
			_ = h.db.AddTaskActivity(act)

			if runErr != nil {
				act.Status = "failed"
				act.Error = runErr.Error()
				finished := time.Now()
				act.CompletedAt = &finished
				_ = h.db.FinishAgentLaunch(act)
				writeError(w, http.StatusInternalServerError, "Cannot track remote execution")
				return
			}
			launchCtx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
			defer cancel()
			err := h.agentDispatcher.DispatchAndWait(launchCtx, ac.UserID, ac.ProjectID, task.ID, agentconfig.Dispatch{
				SchemaVersion: agentconfig.Version, TaskKey: task.Key, TaskID: task.ID, ProjectID: projectID,
				SkillID: req.SkillID, Action: req.SkillID, Prompt: req.Prompt, RunID: remoteRun.ID,
				Mode: mode, Model: strings.TrimSpace(req.Model),
			})
			finished := time.Now()
			act.CompletedAt = &finished
			act.Status = "completed"
			act.Summary = "Native client launched; workflow stages are reported through MCP."
			if err != nil {
				act.Status = "failed"
				act.Error = err.Error()
				act.Summary = "Local agent launch failed."
				// An unconfirmed launch is not a failed one: the agent may be running the
				// skill right now. Closing its run here would make its own finish_run be
				// refused as already finished, and the work would complete unrecorded.
				if errors.Is(err, ErrLaunchUnconfirmed) {
					act.Summary = "Local agent launch not confirmed; its run stays open until the agent reports."
				} else {
					_, _ = h.db.FinishRemoteRun(task.ID, remoteRun.ID, "failed", err.Error())
				}
			}
			_ = h.db.FinishAgentLaunch(act)
			if err != nil {
				writeError(w, http.StatusBadGateway, "Erreur lors de la délégation à l'agent local: "+err.Error())
				return
			}

			writeJSON(w, http.StatusOK, models.RunSkillResponse{
				Task:     *task,
				Activity: act,
				Message:  fmt.Sprintf("Skill %s déléguée à l'agent local (%s)", req.SkillID, ac.DeviceID),
			})
			return
		}

		writeError(w, http.StatusConflict, "Connect the local agent to launch this skill.")
		return
	}

	// Sub-action: /api/tasks/{id}/activities
	if subAction == "activities" && r.Method == http.MethodGet {
		activities, err := h.db.GetTaskActivities(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, activities)
		return
	}

	// Sub-action: /api/tasks/{id}/comment
	if subAction == "comment" && r.Method == http.MethodPost {
		var req struct {
			Body string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid comment payload: "+err.Error())
			return
		}
		if req.Body == "" {
			writeError(w, http.StatusBadRequest, "Comment body is required")
			return
		}
		if err := h.db.AddTaskCommentAs(h.actingContext(r), id, req.Body); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	// Sub-action: /api/tasks/{id}/convert
	if subAction == "convert" && r.Method == http.MethodPost {
		var req models.ConvertTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid convert payload: "+err.Error())
			return
		}
		if req.Target == "" {
			writeError(w, http.StatusBadRequest, "Target tracker is required ('github')")
			return
		}
		task, err := h.db.ConvertTaskToRemote(id, req.Target)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, task)
		return
	}

	// Sub-action: /api/tasks/{id}/git-diff
	if subAction == "git-diff" && r.Method == http.MethodGet {
		diffRes, err := h.db.GetTaskGitDiff(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, diffRes)
		return
	}

	// Sub-action: /api/tasks/{id}/checkout-branch
	if subAction == "checkout-branch" && (r.Method == http.MethodPost || r.Method == http.MethodPut) {
		task, err := h.db.GetTaskByID(id)
		if err != nil || task == nil {
			writeError(w, http.StatusNotFound, "Tâche non trouvée")
			return
		}
		repoPath := h.db.ResolveTaskRepoPath(task)
		branch, err := h.db.EnsureTaskGitBranch(repoPath, task)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		status, _ := h.db.GetGitStatus(repoPath)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message":  fmt.Sprintf("Bascule effectuée avec succès sur la branche '%s'", branch),
			"branch":   branch,
			"repoPath": repoPath,
			"status":   status,
		})
		return
	}

	// Sub-action: /api/tasks/{id}/pin: épingler ou désépingler un ticket pour
	// pouvoir basculer vite d'un chantier à l'autre.
	if subAction == "pin" && (r.Method == http.MethodPost || r.Method == http.MethodDelete) {
		if r.Method == http.MethodDelete {
			if err := h.db.SetTaskPinnedBy(h.webPrincipal(r).Actor(), id, false); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"pinned": false})
			return
		}
		pinned, err := h.db.ToggleTaskPinnedBy(h.webPrincipal(r).Actor(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"pinned": pinned})
		return
	}

	// Sub-action: /api/tasks/{id}/tty-agent: open the task's session and start
	// the agent configured on its project. Nothing else: typing the skill call is
	// a separate, deliberate gesture.
	if subAction == "tty-agent" && r.Method == http.MethodPost {
		var req struct {
			Force bool `json:"force"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		launch, err := h.db.StartAgentInTTY(id, req.Force)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, launch)
		return
	}

	// Sub-action: /api/tasks/{id}/tty-skill: type the skill call into the agent
	// already running in the task's session.
	if subAction == "tty-skill" && r.Method == http.MethodPost {
		var req struct {
			SkillID string `json:"skillId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if strings.TrimSpace(req.SkillID) == "" {
			writeError(w, http.StatusBadRequest, "skillId manquant")
			return
		}
		if task, err := h.db.GetTaskByID(id); err == nil && task != nil {
			userID := h.webSessionUser(r)
			if ac := h.agentDispatcher.Route(userID, task.ProjectID); ac != nil {
				err := h.agentDispatcher.Dispatch(userID, task.ProjectID, "dispatch_step", task.ID, map[string]string{
					"taskKey": task.Key, "taskId": task.ID, "projectId": task.ProjectID, "skillId": req.SkillID, "action": req.SkillID,
				})
				if err != nil {
					writeError(w, http.StatusBadGateway, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"success": true, "taskId": task.ID, "message": "Skill dispatched to local agent"})
				return
			}
		}
		launch, err := h.db.InjectSkillInTTY(id, req.SkillID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, launch)
		return
	}

	// Sub-action: /api/tasks/{id}/tty-external: open a native external terminal for the task
	if (subAction == "tty-external" || subAction == "terminal-external") && r.Method == http.MethodPost {
		var req struct {
			Command         string `json:"command"`
			SkillID         string `json:"skillId"`
			TerminalCommand string `json:"terminalCommand"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}
		res, err := h.launchTaskExternalTerminal(r.Context(), h.webSessionUser(r), id, req.Command, req.SkillID, req.TerminalCommand)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, res)
		return
	}

	// Sub-action: /api/tasks/{id}/advance/confirm: the user says the interactive
	// session is over. Taskacao applies the move the worker applies for headless
	// steps: stage label, internal status, and transition on the tracker. The
	// repo skill only produces text in the terminal, it never touches the ticket.
	if subAction == "advance-confirm" && r.Method == http.MethodPost {
		var req struct {
			SkillID string `json:"skillId"`
			Note    string `json:"note"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if strings.TrimSpace(req.SkillID) == "" {
			writeError(w, http.StatusBadRequest, "skillId manquant")
			return
		}
		task, act, err := h.db.CompleteInteractiveStep(id, req.SkillID, req.Note)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"task": task, "activity": act})
		return
	}

	// Sub-action: /api/tasks/{id}/stage (or /transition or /workflow-label): switch the agentic
	// workflow label and stage on a story, updating local state and queueing tracker updates.
	if (subAction == "stage" || subAction == "transition" || subAction == "workflow-label") && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodGet) {
		if r.Method == http.MethodGet {
			targetID := id
			if targetID == "" {
				targetID = r.URL.Query().Get("id")
			}
			if targetID == "" {
				targetID = r.URL.Query().Get("key")
			}
			task, err := h.db.GetTaskByID(targetID)
			if err != nil || task == nil {
				writeError(w, http.StatusNotFound, "Tâche non trouvée")
				return
			}
			stage := h.db.StageOfTask(task)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"taskId":  task.ID,
				"taskKey": task.Key,
				"stage":   stage,
				"status":  task.Status,
				"labels":  task.Labels,
			})
			return
		}

		var req struct {
			TaskID  string `json:"taskId"`
			TaskKey string `json:"taskKey"`
			ID      string `json:"id"`
			Key     string `json:"key"`
			Stage   string `json:"stage"`
			Label   string `json:"label"`
			Note    string `json:"note"`
			Comment string `json:"comment"`
			PrURL   string `json:"prUrl"`
			Branch  string `json:"branch"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		targetID := id
		if targetID == "" {
			targetID = req.TaskID
		}
		if targetID == "" {
			targetID = req.TaskKey
		}
		if targetID == "" {
			targetID = req.ID
		}
		if targetID == "" {
			targetID = req.Key
		}
		if targetID == "" {
			targetID = r.URL.Query().Get("id")
		}
		if targetID == "" {
			targetID = r.URL.Query().Get("key")
		}
		if strings.TrimSpace(targetID) == "" {
			writeError(w, http.StatusBadRequest, "Identifiant ou clé de tâche manquant")
			return
		}
		stage := req.Stage
		if stage == "" {
			stage = req.Label
		}
		if stage == "" {
			stage = r.URL.Query().Get("stage")
		}
		if stage == "" {
			stage = r.URL.Query().Get("label")
		}
		if strings.TrimSpace(stage) == "" {
			writeError(w, http.StatusBadRequest, "Paramètre 'stage' (ou 'label') manquant (ex: 'clarified', 'specified', 'implemented', 'reviewed', 'finished')")
			return
		}
		note := req.Note
		if note == "" {
			note = req.Comment
		}
		task, act, err := h.db.TransitionTaskStageBy(h.webSessionUser(r), targetID, stage, note, req.PrURL, req.Branch)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success":  true,
			"message":  fmt.Sprintf("Tâche %s passée à l'étape « %s »", task.Key, stage),
			"task":     task,
			"activity": act,
		})
		return
	}

	// Sub-action: /api/tasks/{id}/advance: one step of the agentic workflow, or
	// the full chain up to the project's stop stage
	if subAction == "advance" && r.Method == http.MethodPost {
		var req struct {
			Auto bool   `json:"auto"`
			Mode string `json:"mode"`
			// Model is the one-off model picked for this launch, empty when the
			// user kept the configured one.
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if !models.ValidSkillMode(req.Mode) {
			writeError(w, http.StatusBadRequest, "mode invalide : "+req.Mode)
			return
		}
		if err := agentconfig.ValidModel(req.Model); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		task, err := h.db.GetTaskByID(id)
		if err != nil || task == nil {
			writeError(w, http.StatusNotFound, "Tâche non trouvée")
			return
		}

		if req.Auto {
			_, act, err := h.db.EnqueueFullChainRun(task.ID)
			if writeTaskBusy(w, err) {
				return
			}
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"mode": "auto", "activity": act})
			return
		}

		stage := h.db.StageOfTask(task)
		step, ok := db.NextStep(stage)
		if !ok {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("aucun pas suivant depuis l'étape %s", stage))
			return
		}
		_, act, err := h.db.EnqueueSkillOnTaskWithOverrides(task.ID, step.SkillID, "", req.Mode, req.Model)
		if writeTaskBusy(w, err) {
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"mode": "queued", "stage": stage, "skillId": step.SkillID, "label": step.Label, "activity": act,
		})
		return
	}

	// Sub-action: /api/tasks/{id}/macro (or /epic): attach the ticket to a macro, or detach
	// it with an empty key.
	if (subAction == "macro" || subAction == "epic") && (r.Method == http.MethodPost || r.Method == http.MethodPut) {
		var req struct {
			MacroKey string `json:"macroKey"`
			EpicKey  string `json:"epicKey"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		key := req.MacroKey
		if key == "" {
			key = req.EpicKey
		}
		// L'écriture part dans la file d'activités : la réponse porte l'activité
		// à suivre, pas un ticket déjà modifié.
		task, activity, err := h.db.SetTaskMacro(h.actingContext(r), id, key)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]interface{}{"queued": true, "task": task, "activity": activity})
		return
	}

	// Sub-action: /api/tasks/{id}/team: change the ticket's team, or clear it with
	// an empty id. The team is optional on a work item, so clearing it is a
	// legitimate instruction and not a missing parameter.
	if subAction == "team" && (r.Method == http.MethodPost || r.Method == http.MethodPut) {
		var req struct {
			TeamID   string `json:"teamId"`
			TeamName string `json:"teamName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		task, activity, err := h.db.SetTaskTeam(h.actingContext(r), id, req.TeamID, req.TeamName)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]interface{}{"queued": true, "task": task, "activity": activity})
		return
	}

	// Sub-action: /api/tasks/{id}/sprint: move the ticket to a sprint of the
	// project's board, or back to the backlog with an empty id.
	if subAction == "sprint" && (r.Method == http.MethodPost || r.Method == http.MethodPut) {
		var req struct {
			SprintID   string `json:"sprintId"`
			SprintName string `json:"sprintName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		task, activity, err := h.db.SetTaskSprint(h.actingContext(r), id, req.SprintID, req.SprintName)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]interface{}{"queued": true, "task": task, "activity": activity})
		return
	}

	// Sub-action: /api/tasks/{id}/assignable: who this ticket can be assigned to.
	// With no query it answers the ticket's team; typing searches the instance,
	// which is what allows assigning someone outside the team.
	if subAction == "assignable" && r.Method == http.MethodGet {
		people, err := h.db.SearchAssignableUsersAs(h.actingContext(r), id, r.URL.Query().Get("q"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, people)
		return
	}

	// Sub-action: /api/tasks/{id}/comments: read and write the ticket's comments
	if subAction == "comments" {
		switch r.Method {
		case http.MethodGet:
			comments, err := h.db.GetTaskCommentsAs(h.actingContext(r), id)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, comments)
			return
		case http.MethodPost:
			var req struct {
				Body string `json:"body"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "Invalid comment payload: "+err.Error())
				return
			}
			comments, err := h.db.PostTaskCommentAs(r.Context(), h.webPrincipal(r).Actor(), id, req.Body)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, comments)
			return
		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
	}

	// Sub-action: /api/tasks/{id}/tracker-status: move a card to a board column,
	// which means transitioning the ticket to that column's status. The local
	// status is written straight away, so the card stays where it was dropped,
	// and the tracker transition runs in the activity queue: it takes seconds,
	// and its refusal belongs in an activity rather than in a timed-out request.
	if subAction == "tracker-status" && (r.Method == http.MethodPost || r.Method == http.MethodPut) {
		var req struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		task, activity, err := h.db.MoveTaskToTrackerStatus(h.actingContext(r), id, req.Status)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]interface{}{"queued": true, "task": task, "activity": activity})
		return
	}

	// Sub-action: /api/tasks/{id}/worktree
	if subAction == "worktree" {
		if r.Method == http.MethodGet {
			info, err := h.db.GetTaskWorktreeInfo(id)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, info)
			return
		} else if r.Method == http.MethodDelete {
			task, err := h.db.GetTaskByID(id)
			if err != nil || task == nil {
				writeError(w, http.StatusNotFound, "Tâche non trouvée")
				return
			}
			repoPath := h.db.ResolveTaskRepoPath(task)
			if err := h.db.RemoveTaskWorktree(repoPath, task.ID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"message": "Worktree supprimé avec succès"})
			return
		}
	}

	// Sub-action: /api/tasks/{id}/sync: perform a unit two-way sync (update tracker and rsync local state)
	if subAction == "sync" && (r.Method == http.MethodPost || r.Method == http.MethodGet) {
		// A synchronisation a person triggered on one ticket also rediscovers
		// its pull requests, whatever the bounding rule of the background pass.
		task, err := h.db.ForceSyncSingleTask(h.actingContext(r), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Échec de la synchronisation unitaire: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message": "Synchronisation unitaire effectuée avec succès",
			"task":    task,
		})
		return
	}

	// Sub-action: /api/tasks/{id}/migrate: migrate task to another compatible project
	if subAction == "migrate" && r.Method == http.MethodPost {
		var req struct {
			TargetProjectID string `json:"targetProjectId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid payload: "+err.Error())
			return
		}
		count, err := h.db.MigrateTasks([]string{id}, req.TargetProjectID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"migratedCount":   count,
			"targetProjectId": req.TargetProjectID,
		})
		return
	}

	// Sub-action: /api/tasks/{id}/clone: clone/duplicate a task or story
	if subAction == "clone" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		var req models.CloneTaskRequest
		if r.Body != nil && r.Body != http.NoBody {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}
		cloned, err := h.db.CloneTask(id, req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, cloned)
		return
	}

	switch r.Method {
	case http.MethodGet:
		task, err := h.db.GetTaskByID(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if task == nil {
			writeError(w, http.StatusNotFound, "Task not found")
			return
		}
		writeJSON(w, http.StatusOK, task)

	case http.MethodPut, http.MethodPatch:
		var req models.UpdateTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
			return
		}
		task, err := h.db.UpdateTaskBy(h.webPrincipal(r).Actor(), id, req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, task)

	case http.MethodDelete:
		if err := h.db.DeleteTask(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Task deleted successfully"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// composedSettings is what /api/settings answers: the deployment row for the
// shared configuration, the caller's own row for the preferences, and the
// account's own identity for userName and userEmail, which are projections of
// that identity rather than fields anyone types (ADR 0015).
func (h *Handler) composedSettings(userID string) (*models.Settings, error) {
	deployment, err := h.db.GetSettings()
	if err != nil {
		return nil, err
	}
	personal, err := h.db.UserSettings(userID)
	if err != nil {
		return nil, err
	}
	composed := *deployment
	composed.Theme = personal.Theme
	composed.AccentColor = personal.AccentColor
	composed.Language = personal.Language
	composed.Density = personal.Density
	composed.DefaultView = personal.DefaultView
	composed.DetailMode = personal.DetailMode
	composed.UIScale = personal.UIScale
	composed.UserName = personal.UserName
	composed.UserEmail = personal.UserEmail
	composed.UserAvatar = personal.UserAvatar
	composed.EditorCommand = personal.EditorCommand
	composed.ExternalTerminalCommand = personal.ExternalTerminalCommand
	if user, err := h.db.GetUser(userID); err == nil && user != nil {
		// Name() is the chain the rest of the application already shows: the
		// chosen name, then the one the sign-in supplied, then the address,
		// then the id. It never answers empty, so it is taken as is; the
		// address is only taken when the account has one, since a personal row
		// may hold something the identity does not.
		composed.UserName = user.Name()
		if user.Email != "" {
			composed.UserEmail = user.Email
		}
	}
	return &composed, nil
}

// maskJiraToken strips the Jira API token from anything sent to the client and
// replaces it with two flags: whether a token is configured at all, and whether
// it comes from the environment rather than the database. An empty incoming
// token means "keep the stored one", so the UI can leave its field blank.
func (h *Handler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	caller := h.webPrincipal(r)
	switch r.Method {
	case http.MethodGet:
		settings, err := h.composedSettings(caller.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, settings)

	case http.MethodPost, http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid settings payload: "+err.Error())
			return
		}
		var req models.Settings
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid settings payload: "+err.Error())
			return
		}
		if err := agentconfig.ValidModelConfig(agentconfig.ModelConfig{Model: req.AIModel, SkillModels: req.AISkillModels}); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := agentconfig.ValidProviderModels(req.AIProviderModels); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		// An empty command template is a value, not an omission: it hands both
		// execution modes back to the provider. Only the raw payload tells the
		// two apart, so presence of the key is what carries the intent.
		var sent map[string]json.RawMessage
		_ = json.Unmarshal(body, &sent)
		// userName and userEmail are projections of the account's identity:
		// they are answered on a read and ignored on a write. Dropping them
		// here, rather than from personalSettingsKeys, keeps a member's
		// whole-row post a silent ignore instead of an admin-only refusal.
		delete(sent, "userEmail")
		delete(sent, "userName")
		current, err := h.db.GetSettings()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// The payload carries both halves. A member may change the personal
		// keys and the tracker ones; touching the rest of the deployment's is
		// refused by naming the keys, so the interface can say which.
		if !caller.IsAdmin() {
			if offending := memberSettingsViolations(*current, sent); len(offending) > 0 {
				writeError(w, http.StatusForbidden, msgAdminOnly+": "+strings.Join(offending, ", "))
				return
			}
		}

		// The personal keys go to the caller's own row. An anonymous caller has
		// no row: the guard already refuses it, this is only belt and braces.
		if caller.UserID != "" {
			personal, err := h.db.UserSettings(caller.UserID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			personalReq, err := memberSettingsPayload(*personal, sent)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if _, err := h.db.UpdateUserSettings(caller.UserID, personalReq); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		// The deployment keys go to the shared row. A member reaches it for the
		// tracker keys only; the payload filter, not this branch, is what keeps
		// the rest of the row theirs to read and an admin's to change.
		{
			deploymentReq, err := deploymentSettingsPayload(*current, sent, caller.IsAdmin())
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			var clear []string
			if caller.IsAdmin() {
				for _, name := range []string{"aiCommandTemplate", "aiCommandTemplateAutonomous"} {
					if _, ok := sent[name]; ok {
						clear = append(clear, name)
					}
				}
			}
			if _, err := h.db.UpdateSettings(deploymentReq, clear...); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		saved, err := h.composedSettings(caller.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, saved)
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *Handler) HandleActivities(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		projectID := r.URL.Query().Get("projectId")

		// Check for /api/activities/stats or general query
		if strings.HasSuffix(r.URL.Path, "/stats") {
			stats, err := h.db.GetActivityStats(projectID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, stats)
			return
		}

		status := r.URL.Query().Get("status")
		skillID := r.URL.Query().Get("skillId")
		taskID := r.URL.Query().Get("taskId")
		q := r.URL.Query().Get("q")
		limit := 100
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
				limit = parsed
			}
		}

		activities, err := h.db.GetActivities(projectID, status, skillID, taskID, q, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, activities)

	case http.MethodDelete:
		count, err := h.db.ClearCompletedActivities()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message": fmt.Sprintf("%d activités terminées supprimées", count),
			"count":   count,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *Handler) HandleActivityDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/activities/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "Activity ID or action required")
		return
	}

	// Sub-action: /api/activities/stats
	if parts[0] == "stats" && r.Method == http.MethodGet {
		projectID := r.URL.Query().Get("projectId")
		stats, err := h.db.GetActivityStats(projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, stats)
		return
	}

	// Sub-action: /api/activities/clear
	if parts[0] == "clear" && (r.Method == http.MethodPost || r.Method == http.MethodDelete) {
		count, err := h.db.ClearCompletedActivities()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message": fmt.Sprintf("%d activités terminées supprimées", count),
			"count":   count,
		})
		return
	}

	id := parts[0]

	// Sub-action: /api/activities/{id}/retry
	if len(parts) >= 2 && parts[1] == "retry" && r.Method == http.MethodPost {
		act, err := h.db.RetryActivity(id)
		if writeTaskBusy(w, err) {
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, act)
		return
	}

	// Sub-action: /api/activities/{id}/cancel
	if len(parts) >= 2 && parts[1] == "cancel" && r.Method == http.MethodPost {
		if err := h.db.CancelActivity(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Activité annulée avec succès"})
		return
	}

	// Sub-action: /api/activities/{id}/waiting
	// Reported by a Claude Code hook through the local agent loopback when the
	// session blocks on the user, and again when it resumes.
	if len(parts) >= 2 && parts[1] == "waiting" && r.Method == http.MethodPost {
		var body struct {
			Waiting *bool `json:"waiting"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Waiting == nil {
			writeError(w, http.StatusBadRequest, "Body must be {\"waiting\": true|false}")
			return
		}
		if err := h.db.SetRemoteRunWaiting(id, *body.Waiting); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"waiting": *body.Waiting})
		return
	}

	// Sub-action: /api/activities/{id}/engine
	// Reported by the local agent once it has built the command line, which is
	// the only place the workstation override is applied. It corrects what the
	// launcher recorded from its own resolution.
	if len(parts) >= 2 && parts[1] == "engine" && r.Method == http.MethodPost {
		var body struct {
			Provider string `json:"provider"`
			Model    string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "Body must be {\"provider\": string, \"model\": string}")
			return
		}
		// The provider never reaches a command line, but it is stored and
		// displayed, so it is held to the same shape as the model rather than
		// persisted verbatim.
		if err := agentconfig.ValidModel(body.Provider); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := agentconfig.ValidModel(body.Model); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.db.SetRemoteRunEngine(id, body.Provider, body.Model); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"provider": body.Provider, "model": body.Model})
		return
	}

	switch r.Method {
	case http.MethodGet:
		act, err := h.db.GetActivityByID(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if act == nil {
			writeError(w, http.StatusNotFound, "Activity not found")
			return
		}
		writeJSON(w, http.StatusOK, act)

	case http.MethodDelete:
		if err := h.db.DeleteActivity(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Activité supprimée avec succès"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// HandleTerminalWs upgrades the connection to WebSocket and streams the interactive PTY session
func (h *Handler) HandleTerminalWs(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusGone, "Terminal consoles are owned by sectile-agent. Open the local desktop console.")
}

func (h *Handler) HandleTerminalSessions(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusGone, "Terminal consoles are owned by sectile-agent. Open the local desktop console.")
}

func (h *Handler) HandleTerminalSend(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusGone, "Terminal consoles are owned by sectile-agent. Open the local desktop console.")
}

func (h *Handler) HandleTerminalReset(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusGone, "Terminal consoles are owned by sectile-agent. Open the local desktop console.")
}

func (h *Handler) HandleAgentConnect(w http.ResponseWriter, r *http.Request) {
	// Extract authentication token from Authorization header or query param.
	token := ""
	if auth := r.Header.Get("Authorization"); auth != "" {
		token = strings.TrimPrefix(auth, "Bearer ")
	}
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		writeError(w, http.StatusUnauthorized, "Missing agent authentication token")
		return
	}

	// Resolve the user from the API key. An expired key is refused by name so
	// the agent log tells its owner to renew rather than to check for a typo.
	credential, err := h.resolveAgentCredential(token)
	if errors.Is(err, db.ErrAPIKeyExpired) {
		writeError(w, http.StatusUnauthorized, agentAuthMessage(err))
		return
	}
	if err != nil {
		writeError(w, http.StatusForbidden, "Invalid agent token")
		return
	}
	userID := credential.UserID

	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		projectID = "default"
	}
	deviceID := r.URL.Query().Get("deviceId")
	if deviceID == "" {
		deviceID = "unknown"
	}

	// Upgrade to WebSocket.
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[AgentConnect] WebSocket upgrade failed: %v", err)
		return
	}

	// Register the agent, potentially rebinding an existing session.
	ac := h.agentDispatcher.Register(userID, projectID, deviceID, conn)

	// Keepalive: without a read deadline a silently dropped connection stays
	// registered forever, and every operation routed to it stalls for its full
	// timeout. The deadline is refreshed by each frame the agent sends, pongs
	// included; gorilla answers the server pings from the agent's own read
	// loop, which stays responsive because operations run in goroutines.
	_ = conn.SetReadDeadline(time.Now().Add(h.agentReadTimeout))
	conn.SetPongHandler(func(string) error {
		ac.Touch()
		return conn.SetReadDeadline(time.Now().Add(h.agentReadTimeout))
	})
	go ac.Keepalive(h.agentPingInterval)

	// Broadcast agent connection event to SSE subscribers.
	h.BroadcastEvent(Event{
		Type: "agent_connected",
	})

	// Pull active running/queued tasks from the newly connected agent if enabled.
	if h.pullOnConnect {
		go h.pullAndApplyAgentTasks(ac)
	}

	// Read loop: handle messages from the local agent (pty_output, step_status,
	// heartbeat responses). The loop exits when the connection closes.
	defer func() {
		h.agentDispatcher.Unregister(userID, projectID, conn)
		_ = conn.Close()
		h.BroadcastEvent(Event{
			Type: "agent_disconnected",
		})
		log.Printf("[AgentConnect] Agent disconnected: user=%s project=%s device=%s", userID, projectID, deviceID)
	}()

	for {
		_, msgData, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				silence := time.Since(ac.LastSeen()).Round(time.Second)
				log.Printf("[AgentConnect] Read loop ended for device=%s after %s of silence: %v",
					deviceID, silence, err)
				// Hanging up without saying why leaves the agent log with a bare
				// "close 1006 (abnormal closure)". Name the silence so the user
				// reading the agent log can tell a keepalive timeout from a
				// rebound session or a server restart. The read deadline is
				// matched on Timeout() rather than os.ErrDeadlineExceeded:
				// gorilla replaces a temporary network error with one of its
				// own and the original is no longer in the chain.
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					ac.Close(agentCloseKeepaliveTimeout, fmt.Sprintf("no frame received for %s", silence))
				}
			}
			break
		}
		ac.Touch()
		_ = conn.SetReadDeadline(time.Now().Add(h.agentReadTimeout))

		var msg AgentMessage
		if err := json.Unmarshal(msgData, &msg); err != nil {
			log.Printf("[AgentConnect] Malformed message from agent: %v", err)
			continue
		}

		switch msg.Type {
		case "heartbeat":
			_ = ac.Send(AgentMessage{
				MsgID: msg.MsgID,
				Type:  "heartbeat",
			})
		case "pty_output":
			// Relay terminal output to the browser SSE or WebSocket subscribers.
			h.BroadcastEvent(Event{
				Type: "agent_pty_output",
			})
		case "workspace_result":
			h.agentDispatcher.ReportOperation(ac, msg)
		case "step_status":
			h.agentDispatcher.ReportLaunchStatus(ac, msg)
			// The local agent reports progress on a dispatched workflow step.
			h.BroadcastEvent(Event{
				Type: "agent_step_status",
			})
		case "running_tasks":
			h.agentDispatcher.ReportRunningTasks(ac, msg)
		default:
			log.Printf("[AgentConnect] Unknown message type from agent: %s", msg.Type)
		}
	}
}

// resolveAgentUser maps a machine bearer credential to the user it is bound
// to, or to an empty string when it is refused for any reason.
func (h *Handler) resolveAgentUser(token string) string {
	credential, err := h.resolveAgentCredential(token)
	if err != nil {
		return ""
	}
	return credential.UserID
}

// HandleAgentStatus returns the list of currently connected local agents.
func (h *Handler) HandleAgentStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agents": h.agentDispatcher.ConnectedAgents(),
	})
}

// HandleAgentDispatch allows the Web UI to send a workflow command to the
// user's connected local agent. It enforces the identity guard: the requesting
// session's user ID must match the agent's authenticated user ID.
func (h *Handler) HandleAgentDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		UserID    string          `json:"userId"`
		ProjectID string          `json:"projectId"`
		TaskID    string          `json:"taskId"`
		Action    string          `json:"action"` // dispatch_step, pty_input, pty_resize
		Payload   json.RawMessage `json:"payload,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// A member's dispatch reaches their own agent, whatever user the body
	// names; only an admin may address another user's agent.
	caller := h.webPrincipal(r)
	if caller.Anonymous() {
		writeError(w, http.StatusUnauthorized, msgSignIn)
		return
	}
	if req.UserID == "" || (req.UserID != caller.UserID && !caller.IsAdmin()) {
		req.UserID = caller.UserID
	}
	if req.ProjectID == "" {
		req.ProjectID = "default"
	}

	ac := h.agentDispatcher.Route(req.UserID, req.ProjectID)
	if ac == nil {
		writeError(w, http.StatusPreconditionRequired, "No local agent connected. Start 'sectile-agent' on your workstation.")
		return
	}

	if err := h.agentDispatcher.Dispatch(req.UserID, req.ProjectID, req.Action, req.TaskID, req.Payload); err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("Failed to dispatch to local agent: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Command dispatched to local agent"})
}

func (h *Handler) pullAndApplyAgentTasks(ac *AgentConn) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	tasks, err := h.agentDispatcher.PullTasks(ctx, ac)
	if err != nil {
		log.Printf("[AgentConnect] Could not pull running tasks from %s: %v", ac.DeviceID, err)
		return
	}
	log.Printf("[AgentConnect] Pulled %d running/queued tasks from agent %s", len(tasks), ac.DeviceID)
	h.ApplyAgentRunningTasksFor(ac.UserID, tasks)
}

func (h *Handler) pullAndApplyRemoteAgentTasks(location db.AgentLocation) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tasks, err := h.agentDispatcher.PullRemoteTasks(ctx, location)
	if err != nil {
		log.Printf("[AgentConnect] Could not pull running tasks from %s on instance %s: %v", location.DeviceID, location.InstanceID, err)
		return
	}
	h.ApplyAgentRunningTasksFor(location.UserID, tasks)
}

// EnableAgentCluster makes this instance record its agents in the shared
// store and forward work for agents other instances hold. Without a server
// key the instances cannot authenticate each other: agents connected here
// keep working, and forwarding refuses with the reason.
func (h *Handler) EnableAgentCluster() error {
	token, err := h.db.InternalToken()
	h.agentDispatcher.SetCluster(h.db, token, err)
	return err
}

// InternalHandler serves the endpoints other instances forward agent work to.
// It belongs on the internal listener, never on the public one.
func (h *Handler) InternalHandler() http.Handler {
	return h.agentDispatcher.InternalHandler()
}

// ApplyAgentRunningTasks syncs a set of agent tasks to the database and broadcasts updates.
// ApplyAgentRunningTasks records runs an agent reports, without an owner: for
// the startup pull from the connection file, whose user is not known here.
func (h *Handler) ApplyAgentRunningTasks(tasks []agentprotocol.RunningTask) {
	h.ApplyAgentRunningTasksFor("", tasks)
}

// ApplyAgentRunningTasksFor records the runs a connected agent reports as its
// user's: a run the agent has is a run that user started.
func (h *Handler) ApplyAgentRunningTasksFor(ownerID string, tasks []agentprotocol.RunningTask) {
	for _, t := range tasks {
		if t.Status != "queued" && t.Status != "running" {
			continue
		}
		summary := fmt.Sprintf("Execution %s on agent", t.Status)
		var startedAt *time.Time
		if !t.StartedAt.IsZero() {
			startedAt = &t.StartedAt
		}
		act, err := h.db.SyncRemoteRunStatusFor(ownerID, t.ID, t.TaskID, t.ProjectID, t.TaskKey, t.Skill, t.Status, summary, startedAt)
		if err != nil {
			log.Printf("[AgentTasks] Failed to sync run %s (%s): %v", t.ID, t.TaskKey, err)
			continue
		}
		if act != nil {
			h.BroadcastEvent(Event{
				Type: "agent_step_status",
			})
		}
	}
}

// TryPullLocalAgentTasks checks for a local agent connection file on startup
// and pulls any active tasks directly from its HTTP runs endpoint.
func (h *Handler) TryPullLocalAgentTasks() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	candidatePaths := []string{
		filepath.Join(home, ".taskflow", "agent-connection.json"),
	}
	if h.dataDir != "" {
		candidatePaths = append([]string{filepath.Join(h.dataDir, "agent-connection.json")}, candidatePaths...)
	}
	for _, infoPath := range candidatePaths {
		data, err := os.ReadFile(infoPath)
		if err != nil {
			continue
		}
		var info struct {
			URL   string `json:"url"`
			Token string `json:"token"`
		}
		if err := json.Unmarshal(data, &info); err != nil || info.URL == "" {
			continue
		}
		client := &http.Client{Timeout: 3 * time.Second}
		req, err := http.NewRequest(http.MethodGet, strings.TrimRight(info.URL, "/")+"/desktop/runs", nil)
		if err != nil {
			continue
		}
		if info.Token != "" {
			req.Header.Set("Authorization", "Bearer "+info.Token)
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		var runs []agentprotocol.RunningTask
		if err := json.NewDecoder(resp.Body).Decode(&runs); err != nil {
			continue
		}
		log.Printf("[Startup] Pulled %d tasks from local agent (%s)", len(runs), info.URL)
		h.ApplyAgentRunningTasks(runs)
		break
	}
}

// HandleAgentPull allows triggering a pull of running tasks from all connected agents
// and any available local agent.
func (h *Handler) HandleAgentPull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	h.TryPullLocalAgentTasks()

	activeConns := h.agentDispatcher.ActiveConnections()
	for _, ac := range activeConns {
		go h.pullAndApplyAgentTasks(ac)
	}
	// Agents connected to other instances sharing the database report through
	// the instance holding them.
	remote := h.agentDispatcher.RemoteAgents()
	for _, location := range remote {
		go h.pullAndApplyRemoteAgentTasks(location)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":          "ok",
		"connectedAgents": len(activeConns) + len(remote),
	})
}

// HandleOpenEditor opens a workspace, worktree, or path in the user's code editor (default: code)
func (h *Handler) HandleOpenEditor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req struct {
		TaskID        string `json:"taskId"`
		ProjectID     string `json:"projectId"`
		EditorCommand string `json:"editorCommand"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.TaskID != "" {
		task, err := h.db.GetTaskByID(req.TaskID)
		if err != nil || task == nil {
			writeError(w, http.StatusNotFound, "Task not found")
			return
		}
		req.ProjectID = task.ProjectID
	}
	if req.EditorCommand == "" {
		// The editor is a personal command too, so it is the caller's own,
		// falling back to the deployment default through UserSettings.
		settings, _ := h.db.UserSettings(h.webSessionUser(r))
		if settings != nil {
			req.EditorCommand = settings.EditorCommand
		}
	}
	if err := h.db.AgentOperation(agentprotocol.Operation{ProjectID: req.ProjectID, TaskID: req.TaskID, Action: "open_editor", Editor: req.EditorCommand}, nil); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handler) HandleOpenExternalTerminal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req struct {
		TaskID          string `json:"taskId"`
		SkillID         string `json:"skillId"`
		Command         string `json:"command"`
		TerminalCommand string `json:"terminalCommand"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.TaskID == "" {
		writeError(w, http.StatusBadRequest, "Select a task to open an agent console")
		return
	}
	result, err := h.launchTaskExternalTerminal(r.Context(), h.webSessionUser(r), req.TaskID, req.Command, req.SkillID, req.TerminalCommand)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// LaunchTaskExternalTerminal serves callers with no HTTP request of their own,
// which is why it names the implicit user rather than resolving one.
func (h *Handler) LaunchTaskExternalTerminal(taskID, command, skillID, customTermCmd string) (map[string]interface{}, error) {
	return h.launchTaskExternalTerminal(context.Background(), ImplicitUser, taskID, command, skillID, customTermCmd)
}

func (h *Handler) launchTaskExternalTerminal(ctx context.Context, userID, taskID, command, skillID, customTermCmd string) (map[string]interface{}, error) {
	task, err := h.db.GetTaskByID(taskID)
	if err != nil || task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	// The workstation commands are personal (ADR 0015): the terminal that
	// opens is the one of whoever owns this execution, not the deployment's.
	// The project's own override still comes first, the deployment default last.
	settings, _ := h.db.UserSettings(userID)
	var proj *models.Project
	if task.ProjectID != "" {
		proj, _ = h.db.GetProjectByID(task.ProjectID)
	}

	// Precedence: what the call asked for, then the project's own terminal,
	// then the owner's. The resolved value is what travels to the agent, so a
	// personal terminal command is honoured on a launch, not only in the
	// profile screen.
	customTermCmd = strings.TrimSpace(customTermCmd)
	if customTermCmd == "" && proj != nil && proj.ExternalTerminalCommand != "" {
		customTermCmd = proj.ExternalTerminalCommand
	}
	if customTermCmd == "" && settings != nil && settings.ExternalTerminalCommand != "" {
		customTermCmd = settings.ExternalTerminalCommand
	}
	terminalOverride := customTermCmd

	projectID := "default"
	if task.ProjectID != "" {
		projectID = task.ProjectID
	}
	if userID == "" {
		userID = ImplicitUser
	}
	if ac := h.agentDispatcher.Route(userID, projectID); ac != nil {
		log.Printf("🚀 [LaunchTaskExternalTerminal] Delegating external terminal launch to connected agent (%s)", ac.DeviceID)
		launchCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		err := h.agentDispatcher.DispatchAndWait(launchCtx, userID, projectID, task.ID, agentconfig.Dispatch{
			SchemaVersion: agentconfig.Version, TaskKey: task.Key, TaskID: task.ID, ProjectID: projectID,
			SkillID: skillID, Action: "open_terminal", Command: command, Prompt: command, TerminalOverride: terminalOverride,
		})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"success": true,
			"taskId":  task.ID,
			"message": fmt.Sprintf("External terminal opened by local agent (%s)", ac.DeviceID),
		}, nil
	}

	return nil, fmt.Errorf("connect a local agent before opening a terminal")
}

func (h *Handler) HandleTaskPins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	tasks, err := h.db.PinnedTasks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

// HandleMacroRoute handles direct macro API requests like POST /api/macros/{key}/refine.
func (h *Handler) HandleMacroRoute(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/macros/")
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "Clé de macro obligatoire")
		return
	}
	key, err := url.PathUnescape(parts[0])
	if err != nil || key == "" {
		key = parts[0]
	}

	if len(parts) >= 2 && parts[1] == "refine" && r.Method == http.MethodPost {
		projectID := r.URL.Query().Get("projectId")
		todos, proposed, framework, err := h.db.RefineMacro(projectID, key)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"key":           key,
			"todos":         todos,
			"proposedTasks": proposed,
			"specFramework": framework,
		})
		return
	}

	writeError(w, http.StatusNotFound, "Route non trouvée")
}

// HandleTaskPostBack receives post-back task updates resulting from local actions or external tracker operations.
// POST /api/tasks/postback
// POST /api/tasks/{id}/postback
func (h *Handler) HandleTaskPostBack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var payload models.TaskPostBackPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}

	rawPath := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	rawPath = strings.TrimSuffix(rawPath, "/postback")
	rawPath = strings.Trim(rawPath, "/")
	if payload.TaskID == "" && payload.TaskKey == "" && rawPath != "" && rawPath != "postback" {
		payload.TaskID = rawPath
	}

	task, act, err := h.db.PostBackTask(payload)
	if err != nil && task == nil {
		writeJSON(w, http.StatusBadRequest, models.TaskPostBackResult{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	writeJSON(w, http.StatusOK, models.TaskPostBackResult{
		Success:  err == nil,
		Task:     task,
		Activity: act,
		Error:    errStr,
	})
}

// HandleEventsSSE handles real-time SSE stream connections.
// GET /api/events
// GET /api/events/sse
func (h *Handler) HandleEventsSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "Streaming unsupported")
		return
	}

	ch := h.SubscribeEvents()
	defer h.UnsubscribeEvents(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case event, open := <-ch:
			if !open {
				return
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		}
	}
}
