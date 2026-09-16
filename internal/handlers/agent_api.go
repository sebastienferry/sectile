package handlers

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/taskmcp"
)

// defaultMCPSessionTimeout bounds a session whose client never says goodbye: a
// process killed outright sends no termination, so only silence reveals it.
// Sectile's own bridge pings well inside this window, which keeps a live but
// idle conversation connected while still closing an abandoned one.
const defaultMCPSessionTimeout = 15 * time.Minute

// mcpSessionTimeout reads the deployment's override. An unparseable or
// negative value keeps the default rather than disabling the bound silently.
func mcpSessionTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("TASKFLOW_MCP_SESSION_TIMEOUT"))
	if raw == "" {
		return defaultMCPSessionTimeout
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil || timeout <= 0 {
		return defaultMCPSessionTimeout
	}
	return timeout
}

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

// MCPHandler serves the tool catalog over a stateful Streamable HTTP session.
// Statefulness is what makes a connection observable: a stateless endpoint
// builds a throwaway session per request, so it can neither tell two clients
// apart nor notice that one went away.
func (h *Handler) MCPHandler() http.Handler {
	server := taskmcp.NewServer(h.db, h.mcpSessions)
	return h.AgentAPIAuth(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{JSONResponse: true, SessionTimeout: mcpSessionTimeout()},
	))
}

// HandleMCPSessions reports the clients currently connected to the MCP
// endpoint. It is a browser-facing status view, so it stays outside the
// machine-API authentication that guards the endpoint itself.
func (h *Handler) HandleMCPSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": h.mcpSessions.Snapshot()})
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
