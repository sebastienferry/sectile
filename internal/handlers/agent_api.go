package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/db"
	"tasks/internal/taskmcp"
)

// defaultMCPSilenceNotice is how long a client may say nothing before Sectile
// remarks on it. Silence is not proof of death: a stage that compiles, tests or
// waits for its owner is quiet for a long while and its run must survive it, so
// crossing this bound only appends a sentence to the runs the session owns.
const defaultMCPSilenceNotice = 4 * time.Hour

// mcpSilenceNotice reads the deployment's override. An unparseable or
// negative value keeps the default rather than disabling the bound silently.
func mcpSilenceNotice() time.Duration {
	raw := strings.TrimSpace(os.Getenv("SECTILE_MCP_SESSION_TIMEOUT"))
	if raw == "" {
		return defaultMCPSilenceNotice
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil || timeout <= 0 {
		return defaultMCPSilenceNotice
	}
	return timeout
}

// defaultMCPAbandonAfter is how long a client may say nothing before Sectile
// gives up on it (#319): past it the session is closed and its runs are canceled
// with the disconnect note, which their owner may still correct. Eight hours
// covers a run waiting on its owner through a working day, while a client that
// died no longer holds the board and its chain until the server restarts.
const defaultMCPAbandonAfter = 8 * time.Hour

// mcpAbandonAfter reads the deployment's override under the same rules as the
// silence bound, and never returns less than that bound: a session is always
// remarked upon before it is closed.
func mcpAbandonAfter(silence time.Duration) time.Duration {
	abandon := defaultMCPAbandonAfter
	if raw := strings.TrimSpace(os.Getenv("SECTILE_MCP_SESSION_ABANDON_AFTER")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			abandon = parsed
		}
	}
	if abandon < silence {
		return silence
	}
	return abandon
}

// AgentAPIAuth shares the agent handshake's identity policy: every machine
// surface takes the workstation API key as a bearer credential. An expired key
// is refused by name so the owner knows to renew it rather than retype it.
func (h *Handler) AgentAPIAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			// MCP and agent configuration are machine APIs, not cross-origin browser APIs.
			writeError(w, http.StatusForbidden, "Browser origins are not allowed on agent APIs")
			return
		}
		if _, err := h.resolveAgentCredential(bearerToken(r)); err != nil {
			writeError(w, http.StatusUnauthorized, agentAuthMessage(err))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// bearerToken extracts the credential of a machine request, empty when the
// header is missing or uses another scheme.
func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

// agentAuthMessage is what a refused machine request is told. Only expiry is
// named: distinguishing an unknown key from a revoked one would tell a caller
// whether a key ever existed.
func agentAuthMessage(err error) string {
	if errors.Is(err, db.ErrAPIKeyExpired) {
		return db.ErrAPIKeyExpired.Error()
	}
	// A blocked account is worth naming too: the key is valid, and its owner
	// would otherwise hunt for a typo in a token that is perfectly good.
	if errors.Is(err, db.ErrAccountBlocked) {
		return msgBlocked
	}
	return "Valid agent bearer token required"
}

// sharedServerTokenConfigured reports whether the deployment still pins the
// deprecated shared credential.
func sharedServerTokenConfigured() bool {
	return strings.TrimSpace(os.Getenv("SECTILE_SERVER_TOKEN")) != ""
}

// validAgentToken accepts the deprecated shared server credential. It stays
// for one release so an upgrade does not cut off an agent started with it, and
// it is now the only credential outside the key store that opens a machine
// surface: the legacy open mode, where an unset SECTILE_SERVER_TOKEN made any
// nonempty token name the implicit user, is gone (ADR 0019).
func validAgentToken(token string) bool {
	if strings.TrimSpace(token) == "" || !sharedServerTokenConfigured() {
		return false
	}
	expected := os.Getenv("SECTILE_SERVER_TOKEN")
	return subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

// sharedTokenDeprecation is logged the first time the shared credential is
// used, once per process: every request would flood the log, and once is
// enough to send the operator to the profile.
var sharedTokenDeprecation sync.Once

// SharedServerTokenWarning is the line the server prints when the deprecated
// credential is configured or used.
const SharedServerTokenWarning = "SECTILE_SERVER_TOKEN is deprecated and will be removed in the next release: create an API key from the web profile and start the agent with it instead"

// agentCredential is who a machine bearer credential names. Device is nil when
// the deprecated shared server token was presented.
type agentCredential struct {
	UserID string
	Device *db.DeviceCredential
}

// resolveAgentCredential maps a bearer credential to the user it is bound to.
// An API key resolves through the database; a deployment that still pins the
// shared server token resolves to the single implicit user, with a deprecation
// notice in the log. Nothing else resolves to anyone.
//
// There is no longer a fallback for a deployment that holds no key at all.
// Signing in is mandatory (ADR 0015) and the machine surfaces are not an
// exception to it: a credential that names no key and matches no configured
// shared token names nobody, on a fresh deployment as on an established one.
// The count of issued keys is deliberately not consulted, because a count can
// fall back to zero — an emptied key store would otherwise re-arm the open mode
// on a deployment that had once left it behind.
func (h *Handler) resolveAgentCredential(token string) (agentCredential, error) {
	if strings.TrimSpace(token) == "" {
		return agentCredential{}, db.ErrAPIKeyUnknown
	}
	device, err := h.db.LookupDeviceToken(token)
	if err == nil {
		// A blocked account's workstation keys stop opening with it. Leaving
		// them valid would make the block a browser-only measure, while the
		// key is the credential that runs the agent and the MCP tools.
		if user, lookupErr := h.db.GetUser(device.UserID); lookupErr == nil && user != nil && user.Blocked {
			return agentCredential{}, db.ErrAccountBlocked
		}
		return agentCredential{UserID: device.UserID, Device: device}, nil
	}
	if !errors.Is(err, db.ErrAPIKeyUnknown) {
		return agentCredential{}, err
	}
	if !validAgentToken(token) {
		return agentCredential{}, db.ErrAPIKeyUnknown
	}
	sharedTokenDeprecation.Do(func() { log.Printf("[Identity] %s", SharedServerTokenWarning) })
	return agentCredential{UserID: ImplicitUser}, nil
}

// HandleAgentIdentity tells a machine caller who its key names and when the
// key runs out, so an agent can warn its owner ahead of the expiry.
func (h *Handler) HandleAgentIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	credential, err := h.resolveAgentCredential(bearerToken(r))
	if err != nil {
		writeError(w, http.StatusUnauthorized, agentAuthMessage(err))
		return
	}
	body := map[string]interface{}{
		"userId":      credential.UserID,
		"role":        h.principalFor(credential.UserID).Role,
		"mode":        h.signInMode(),
		"sharedToken": credential.Device == nil,
		"expiresAt":   nil,
	}
	if credential.Device != nil {
		body["deviceId"] = credential.Device.ID
		body["label"] = credential.Device.Label
		if credential.Device.ExpiresAt != nil {
			body["expiresAt"] = credential.Device.ExpiresAt
		}
	}
	writeJSON(w, http.StatusOK, body)
}

// MCPHandler serves the tool catalog over a stateful Streamable HTTP session.
// Statefulness is what makes a connection observable: a stateless endpoint
// builds a throwaway session per request, so it can neither tell two clients
// apart nor notice that one went away.
//
// A request for a session another instance holds is forwarded to it (see
// mcpRouter).
func (h *Handler) MCPHandler() http.Handler {
	return h.AgentAPIAuth(h.mcpRouter(h.mcpStreamableHandler()))
}

// mcpStreamableHandler is the one transport handler of this instance. It holds
// the sessions in memory, so the public and the internal routes share it, and
// building it twice would split them.
func (h *Handler) mcpStreamableHandler() http.Handler {
	h.mcpOnce.Do(func() {
		server := taskmcp.NewServerWithCallers(h.db, h.mcpSessions, h.mcpCaller)
		h.mcpServer = server
		h.mcpStreamable = mcp.NewStreamableHTTPHandler(
			func(*http.Request) *mcp.Server { return server },
			// The transport is given no bound of its own: its timeout closes the
			// session, which would cancel every run it adopted. Sectile owns the
			// bound instead and only marks the silence (see SessionRegistry).
			&mcp.StreamableHTTPOptions{JSONResponse: true, SessionTimeout: 0},
		)
	})
	return h.mcpStreamable
}

// mcpCaller names the user behind an MCP call from the bearer key of the HTTP
// request that carried it, so a run a client starts has an owner. The header is
// the one AgentAPIAuth already accepted; a call without it names nobody.
func (h *Handler) mcpCaller(header http.Header) (taskmcp.Caller, bool) {
	if header == nil {
		return taskmcp.Caller{}, false
	}
	credential, err := h.resolveAgentCredential(bearerToken(&http.Request{Header: header}))
	if err != nil {
		return taskmcp.Caller{}, false
	}
	p := h.principalFor(credential.UserID)
	// A key paired to no device is the shared server key: it names the
	// implicit account, not a person, so its tracker writes are refused.
	return taskmcp.Caller{UserID: p.UserID, Name: p.Name, Role: p.Role, Anonymous: credential.Device == nil}, true
}

// HandleMCPSessions reports the clients currently connected to the MCP
// endpoint. It is a browser-facing status view, so it stays outside the
// machine-API authentication that guards the endpoint itself.
//
// When several instances serve the deployment, the view lists the sessions of
// every live one, and names in unreachable those that did not answer in time
// rather than failing for them.
func (h *Handler) HandleMCPSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	sessions := h.mcpSessions.Snapshot()
	unreachable := []string{}
	if peers := h.mcpCluster.peers(); len(peers) > 0 {
		var remote []taskmcp.SessionView
		remote, unreachable = h.mcpCluster.peerSessions(r.Context(), peers)
		sessions = append(sessions, remote...)
		taskmcp.SortSessions(sessions)
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions, "unreachable": unreachable})
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
		log.Printf("[AgentAPI] cannot list projects: %v", err)
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
