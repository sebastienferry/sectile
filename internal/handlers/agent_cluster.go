package handlers

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/db"
)

// A local agent keeps one WebSocket to whichever server instance the load
// balancer gave it. When several instances share a database, the one a
// request reaches may not be that one. The cluster side of the dispatcher
// records in the database which instance holds each agent, and forwards the
// work to that instance's internal endpoints; the instance holding the
// connection runs it exactly as if the request had reached it, correlation of
// the agent's answers included. A request is forwarded at most once: the
// internal endpoints only ever act on a local connection.
//
// See docs/clarifications/406.md.

// AgentDirectory is where instances record the agents they hold. *db.DB
// implements it.
type AgentDirectory interface {
	InstanceID() string
	AgentConnected(userID, projectID, deviceID string) (db.AgentLocation, bool, error)
	AgentDisconnected(userID, projectID, deviceID string)
	AgentOwner(userID, projectID string) (db.AgentLocation, bool)
	AgentRecentlyConnected(userID, projectID string, since time.Time) bool
	ConnectedAgentLocations() []db.AgentLocation
}

// AgentRoute is where an agent slot can be reached from here: on a local
// connection, or through another instance.
type AgentRoute struct {
	UserID    string
	ProjectID string
	DeviceID  string
	local     *AgentConn
	remote    *db.AgentLocation
}

// agentCluster is nil on a server that shares its store with nobody, and every
// method is then a no-op, so the single-instance behaviour is unchanged.
type agentCluster struct {
	directory AgentDirectory
	token     string
	tokenErr  error
	client    *http.Client
}

// internalAgentPath prefixes the internal endpoints.
const internalAgentPath = "/internal/agent/"

// Error codes carried across instances, so a forwarded failure keeps the
// meaning callers test for.
const (
	codeNoAgent           = "no_agent"
	codeRunNotOwned       = "run_not_owned"
	codeLaunchUnconfirmed = "launch_unconfirmed"
	codeAgentOutdated     = "agent_outdated"
)

// outdatedRelay is an UnsupportedOperationError that crossed instances: the
// holding instance's message verbatim, still matching ErrUnsupportedOperation.
type outdatedRelay struct{ message string }

func (e outdatedRelay) Error() string { return e.message }
func (e outdatedRelay) Unwrap() error { return agentprotocol.ErrUnsupportedOperation }

// SetCluster makes the dispatcher record its agents in the directory and
// forward work for agents held by other instances. tokenErr, when set, says
// why no bearer is available: forwarding and the internal endpoints then
// refuse with that reason.
func (d *AgentDispatcher) SetCluster(directory AgentDirectory, token string, tokenErr error) {
	if directory == nil {
		d.cluster = nil
		return
	}
	d.cluster = &agentCluster{directory: directory, token: token, tokenErr: tokenErr, client: &http.Client{}}
}

// Route returns where an agent slot can be reached, without waiting, or nil.
func (d *AgentDispatcher) Route(userID, projectID string) *AgentRoute {
	if ac := d.Lookup(userID, projectID); ac != nil {
		return &AgentRoute{UserID: ac.UserID, ProjectID: ac.ProjectID, DeviceID: ac.DeviceID, local: ac}
	}
	if location, ok := d.cluster.owner(userID, projectID); ok {
		return &AgentRoute{UserID: location.UserID, ProjectID: location.ProjectID, DeviceID: location.DeviceID, remote: &location}
	}
	return nil
}

// waitForRoute is Route with the reconnection grace WaitForAgent gives a local
// agent, extended to an agent reconnecting through another instance.
func (d *AgentDispatcher) waitForRoute(ctx context.Context, userID, projectID string, grace time.Duration) *AgentRoute {
	if route := d.Route(userID, projectID); route != nil {
		return route
	}
	if !d.agentWasRecentlyConnected(userID, projectID) {
		return nil
	}
	deadline := time.NewTimer(grace)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-deadline.C:
			return nil
		case <-ticker.C:
			if route := d.Route(userID, projectID); route != nil {
				return route
			}
		}
	}
}

// RemoteAgents lists the agents other instances hold.
func (d *AgentDispatcher) RemoteAgents() []db.AgentLocation {
	if d.cluster == nil {
		return nil
	}
	self := d.cluster.directory.InstanceID()
	var out []db.AgentLocation
	for _, location := range d.cluster.directory.ConnectedAgentLocations() {
		if location.InstanceID != self {
			out = append(out, location)
		}
	}
	return out
}

// PullRemoteTasks asks the instance holding an agent for its running tasks.
func (d *AgentDispatcher) PullRemoteTasks(ctx context.Context, location db.AgentLocation) ([]agentprotocol.RunningTask, error) {
	raw, err := d.cluster.forward(ctx, location, "pull", internalRequest{UserID: location.UserID, ProjectID: location.ProjectID})
	if err != nil {
		return nil, err
	}
	var tasks []agentprotocol.RunningTask
	if err := json.Unmarshal(raw, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (c *agentCluster) owner(userID, projectID string) (db.AgentLocation, bool) {
	if c == nil {
		return db.AgentLocation{}, false
	}
	location, ok := c.directory.AgentOwner(userID, projectID)
	if !ok || location.InstanceID == c.directory.InstanceID() {
		// This instance's own record without a local connection is a
		// departure in progress, not somewhere to forward to.
		return db.AgentLocation{}, false
	}
	return location, true
}

func (c *agentCluster) recentlyConnected(userID, projectID string) bool {
	return c != nil && c.directory.AgentRecentlyConnected(userID, projectID, time.Now().UTC().Add(-agentAbsenceIsRecent))
}

// agentConnected records a new local connection, and asks the instance that
// held the slot before, if another, to close its connection as a single
// instance does on a rebind.
func (c *agentCluster) agentConnected(ac *AgentConn) {
	if c == nil {
		return
	}
	previous, had, err := c.directory.AgentConnected(ac.UserID, ac.ProjectID, ac.DeviceID)
	if err != nil {
		log.Printf("[AgentDispatcher] %v", err)
		return
	}
	if !had {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := c.forward(ctx, previous, "close", internalRequest{UserID: ac.UserID, ProjectID: ac.ProjectID}); err != nil && !errors.Is(err, ErrNoAgentConnected) {
			log.Printf("[AgentDispatcher] Could not close the previous connection of user=%s project=%s on instance %s: %v",
				ac.UserID, ac.ProjectID, previous.InstanceID, err)
		}
	}()
}

func (c *agentCluster) agentDisconnected(userID, projectID, deviceID string) {
	if c != nil {
		c.directory.AgentDisconnected(userID, projectID, deviceID)
	}
}

// internalRequest is the body of every internal call.
type internalRequest struct {
	UserID    string                   `json:"userId,omitempty"`
	ProjectID string                   `json:"projectId,omitempty"`
	TaskID    string                   `json:"taskId,omitempty"`
	Type      string                   `json:"type,omitempty"`
	Payload   json.RawMessage          `json:"payload,omitempty"`
	Operation *agentprotocol.Operation `json:"operation,omitempty"`
}

// internalResponse is the answer of every internal call.
type internalResponse struct {
	Value json.RawMessage `json:"value,omitempty"`
	Error string          `json:"error,omitempty"`
	Code  string          `json:"code,omitempty"`
}

// forward runs one verb on the instance holding the agent. A failure to get an
// answer is reported as such and never retried: the work may have reached the
// agent, and running it twice is worse than asking the user to retry.
func (c *agentCluster) forward(ctx context.Context, location db.AgentLocation, verb string, body internalRequest) (json.RawMessage, error) {
	if c == nil {
		return nil, ErrNoAgentConnected
	}
	if c.tokenErr != nil {
		return nil, fmt.Errorf("the local agent is connected to server instance %s, but this instance cannot reach it: %w", location.InstanceID, c.tokenErr)
	}
	if strings.TrimSpace(location.Address) == "" {
		return nil, fmt.Errorf("the local agent is connected to server instance %s, which advertises no internal address", location.InstanceID)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(location.Address, "/")+internalAgentPath+verb, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("the server instance holding the local agent (%s) did not answer: %w", location.InstanceID, err)
	}
	defer resp.Body.Close()
	var answer internalResponse
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		return nil, fmt.Errorf("the server instance holding the local agent (%s) answered %s without a readable body: %w", location.InstanceID, resp.Status, err)
	}
	if answer.Error == "" {
		return answer.Value, nil
	}
	switch answer.Code {
	case codeNoAgent:
		return nil, fmt.Errorf("%w: %s", ErrNoAgentConnected, answer.Error)
	case codeRunNotOwned:
		return nil, fmt.Errorf("%w: %s", ErrRunNotOwned, answer.Error)
	case codeLaunchUnconfirmed:
		return nil, fmt.Errorf("%w: %s", ErrLaunchUnconfirmed, answer.Error)
	case codeAgentOutdated:
		return nil, outdatedRelay{message: answer.Error}
	}
	return nil, errors.New(answer.Error)
}

func (c *agentCluster) operation(ctx context.Context, location db.AgentLocation, op agentprotocol.Operation) (json.RawMessage, error) {
	return c.forward(ctx, location, "operation", internalRequest{Operation: &op})
}

func (c *agentCluster) dispatch(location db.AgentLocation, userID, projectID, msgType, taskID string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal dispatch payload: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err = c.forward(ctx, location, "dispatch", internalRequest{UserID: userID, ProjectID: projectID, Type: msgType, TaskID: taskID, Payload: raw})
	return err
}

func (c *agentCluster) dispatchAndWait(ctx context.Context, location db.AgentLocation, userID, projectID, taskID string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = c.forward(ctx, location, "dispatch-and-wait", internalRequest{UserID: userID, ProjectID: projectID, TaskID: taskID, Payload: raw})
	return err
}

// InternalHandler serves the internal endpoints other instances forward agent
// work to. It only ever acts on a connection held here.
func (d *AgentDispatcher) InternalHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		if !d.cluster.authorized(r) {
			writeError(w, http.StatusUnauthorized, "internal call not authorized")
			return
		}
		var req internalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, internalResponse{Error: "invalid internal request"})
			return
		}
		value, err := d.serveInternal(r.Context(), strings.TrimPrefix(r.URL.Path, internalAgentPath), req)
		writeJSON(w, http.StatusOK, internalAnswer(value, err))
	})
}

func (c *agentCluster) authorized(r *http.Request) bool {
	return c != nil && c.tokenErr == nil && internalBearerMatches(r, "Authorization", c.token)
}

// internalBearerMatches compares, in constant time, the bearer a header
// carries with the deployment's internal credential. No credential matches
// nothing.
func internalBearerMatches(r *http.Request, header, token string) bool {
	if token == "" {
		return false
	}
	presented := strings.TrimPrefix(r.Header.Get(header), "Bearer ")
	return subtle.ConstantTimeCompare([]byte(presented), []byte(token)) == 1
}

func (d *AgentDispatcher) serveInternal(ctx context.Context, verb string, req internalRequest) (json.RawMessage, error) {
	userID, projectID := req.UserID, req.ProjectID
	if req.Operation != nil {
		userID, projectID = req.Operation.UserID, req.Operation.ProjectID
		if userID == "" {
			userID = ImplicitUser
		}
	}
	if verb == "close" {
		// A rebind names the exact slot it took over. The project fallbacks
		// would close an agent registered for every project instead.
		d.mu.RLock()
		ac := d.agents[agentKey{UserID: userID, ProjectID: projectID}]
		d.mu.RUnlock()
		if ac != nil {
			ac.Close(4001, "Session Rebound")
		}
		return nil, nil
	}
	ac := d.Lookup(userID, projectID)
	if ac == nil {
		return nil, fmt.Errorf("%w for user %s on project %s on this instance", ErrNoAgentConnected, userID, projectID)
	}
	switch verb {
	case "operation":
		if req.Operation == nil {
			return nil, errors.New("operation missing")
		}
		return d.callOperationLocal(ctx, ac, *req.Operation)
	case "dispatch":
		return nil, d.dispatchLocal(ac, req.UserID, req.Type, req.TaskID, req.Payload)
	case "dispatch-and-wait":
		var payload any = req.Payload
		var dispatch agentconfig.Dispatch
		if json.Unmarshal(req.Payload, &dispatch) == nil {
			payload = dispatch
		}
		return nil, d.dispatchAndWaitLocal(ctx, ac, req.UserID, req.TaskID, payload)
	case "pull":
		tasks, err := d.PullTasks(ctx, ac)
		if err != nil {
			return nil, err
		}
		return json.Marshal(tasks)
	}
	return nil, fmt.Errorf("unknown internal verb %q", verb)
}

func internalAnswer(value json.RawMessage, err error) internalResponse {
	if err == nil {
		return internalResponse{Value: value}
	}
	answer := internalResponse{Error: err.Error()}
	switch {
	case errors.Is(err, ErrNoAgentConnected):
		answer.Code = codeNoAgent
	case errors.Is(err, ErrRunNotOwned):
		answer.Code = codeRunNotOwned
	case errors.Is(err, ErrLaunchUnconfirmed):
		answer.Code = codeLaunchUnconfirmed
	case errors.Is(err, agentprotocol.ErrUnsupportedOperation):
		answer.Code = codeAgentOutdated
	}
	return answer
}
