package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/db"
	"tasks/internal/taskmcp"
)

// AgentAPIAuth shares the agent handshake's identity policy. Deployments can pin
// a bearer credential with TASKFLOW_SERVER_TOKEN; without it this is local mode.
func (h *Handler) AgentAPIAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			// MCP and agent configuration are machine APIs, not cross-origin browser APIs.
			writeError(w, http.StatusForbidden, "Browser origins are not allowed on agent APIs")
			return
		}
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if !strings.HasPrefix(auth, "Bearer ") || h.resolveAgentUser(token) == "" {
			writeError(w, http.StatusUnauthorized, "Valid agent bearer token required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func validAgentToken(token string) bool {
	if strings.TrimSpace(token) == "" {
		return false
	}
	expected := os.Getenv("TASKFLOW_SERVER_TOKEN")
	return expected == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

func (h *Handler) MCPHandler() http.Handler {
	server := taskmcp.NewServer(h.db)
	return h.AgentAPIAuth(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}))
}

func (h *Handler) HandleAgentConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	config, err := h.db.AgentConfig(r.URL.Query().Get("projectId"), r.URL.Query().Get("taskKey"), r.URL.Query().Get("framework"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (h *Handler) HandleAgentProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	projects, err := h.db.AgentProjects()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Cannot list projects")
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

// HandleAgentRunOutput records what an autonomous run printed. An interactive
// run shows its output in a terminal the user is looking at; a headless one has
// nowhere else to put it, so the agent posts it here as it goes. The body is
// capped at the record limit so a runaway CLI cannot push an unbounded request.
func (h *Handler) HandleAgentRunOutput(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req struct {
		TaskID string `json:"taskId"`
		RunID  string `json:"runId"`
		Output string `json:"output"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, db.RemoteRunOutputLimit)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid run output payload: "+err.Error())
		return
	}
	if strings.TrimSpace(req.TaskID) == "" || strings.TrimSpace(req.RunID) == "" {
		writeError(w, http.StatusBadRequest, "taskId and runId are required")
		return
	}
	if err := h.db.AppendRemoteRunOutput(req.TaskID, req.RunID, req.Output); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
