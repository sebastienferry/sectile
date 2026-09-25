package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"tasks/internal/db"
	"tasks/internal/taskmcp"
)

// An MCP session lives in the memory of the server instance that created it
// (ADR 0007), and its id names that instance. When several instances share a
// database behind a load balancer, a request for a session may reach another
// one: that instance forwards it, unchanged, to the owner's internal listener,
// and relays the answer as it comes, streamed events included. The owner serves
// a forwarded request as if it had reached it directly, and never forwards it
// again. A session whose owner is gone is not found anywhere, and its client
// initializes a new one. See docs/clarifications/408.md.

// MCPDirectory is where an instance finds the others. *db.DB implements it.
type MCPDirectory interface {
	InstanceID() string
	LiveInstance(id string) (db.InstanceLocation, bool)
	LiveInstances() []db.InstanceLocation
}

const (
	// mcpSessionHeader carries the session id on every request after
	// initialization.
	mcpSessionHeader = "Mcp-Session-Id"
	// internalAuthHeader carries the internal credential on a forwarded MCP
	// request, whose Authorization header is the client's own.
	internalAuthHeader = "X-Sectile-Internal-Authorization"
	// internalMCPPath serves the MCP requests other instances forward.
	internalMCPPath = "/internal/mcp"
	// internalMCPSessionsPath serves this instance's sessions to the others.
	internalMCPSessionsPath = "/internal/mcp/sessions"
	// mcpForwardDialTimeout bounds reaching the owner, and nothing else: the
	// GET event stream of a session is open for as long as its client is.
	mcpForwardDialTimeout = 5 * time.Second
)

// mcpPeerSessionsTimeout bounds how long the sessions view waits for each
// other instance. A variable so tests can shorten it.
var mcpPeerSessionsTimeout = 2 * time.Second

// mcpCluster is nil on a server that shares its store with nobody, and every
// method is then a no-op, so the single-instance behaviour is unchanged.
type mcpCluster struct {
	directory MCPDirectory
	token     string
	tokenErr  error
	transport http.RoundTripper
}

func newMCPCluster(directory MCPDirectory, token string, tokenErr error) *mcpCluster {
	if directory == nil {
		return nil
	}
	return &mcpCluster{directory: directory, token: token, tokenErr: tokenErr, transport: &http.Transport{
		DialContext:         (&net.Dialer{Timeout: mcpForwardDialTimeout, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConnsPerHost: 32,
		IdleConnTimeout:     90 * time.Second,
	}}
}

// setMCPCluster makes this instance forward MCP requests for sessions other
// instances hold. tokenErr, when set, says why no internal credential is
// available: nothing is then forwarded and the internal endpoints refuse.
func (h *Handler) setMCPCluster(directory MCPDirectory, token string, tokenErr error) {
	h.mcpCluster = newMCPCluster(directory, token, tokenErr)
}

// mcpRouter serves a request here unless its session id names another live
// instance, which it is then forwarded to. A request without a session id is
// an initialization, and the instance receiving it holds the new session.
func (h *Handler) mcpRouter(local http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only an instance presents the internal credential, and only on the
		// internal listener: a client that sends it is not believed.
		r.Header.Del(internalAuthHeader)
		if owner, ok := h.mcpCluster.ownerOf(r.Header.Get(mcpSessionHeader)); ok {
			h.mcpCluster.forward(w, r, owner)
			return
		}
		local.ServeHTTP(w, r)
	})
}

// ownerOf returns the live instance, other than this one, that a session id
// names. An id naming this instance, no instance, or one that is gone is
// served here, where the transport answers 404 for a session it does not hold.
func (c *mcpCluster) ownerOf(sessionID string) (db.InstanceLocation, bool) {
	if c == nil || c.tokenErr != nil {
		return db.InstanceLocation{}, false
	}
	owner := taskmcp.SessionOwner(sessionID)
	if owner == "" || owner == c.directory.InstanceID() {
		return db.InstanceLocation{}, false
	}
	return c.directory.LiveInstance(owner)
}

// forward relays one request to the owner's internal listener and its answer
// back, as it comes. A failure to reach the owner is answered 503 and never
// retried: the request may have reached it, and the body is not kept to be
// replayed anyway.
func (c *mcpCluster) forward(w http.ResponseWriter, r *http.Request, owner db.InstanceLocation) {
	target, err := url.Parse(strings.TrimRight(owner.Address, "/"))
	if strings.TrimSpace(owner.Address) == "" || err != nil {
		writeError(w, http.StatusServiceUnavailable,
			fmt.Sprintf("the server instance holding this MCP session (%s) advertises no usable internal address", owner.ID))
		return
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.URL.Path = target.Path + internalMCPPath
			pr.Out.URL.RawPath = ""
			pr.SetXForwarded()
			pr.Out.Header.Set(internalAuthHeader, "Bearer "+c.token)
		},
		Transport: c.transport,
		// Every event of the stream, and every answer, reaches the client as
		// soon as the owner writes it.
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("[MCP] Relais vers l'instance %s impossible : %v", owner.ID, err)
			writeError(w, http.StatusServiceUnavailable,
				fmt.Sprintf("the server instance holding this MCP session (%s) did not answer: %v", owner.ID, err))
		},
	}
	proxy.ServeHTTP(w, r)
}

// authorized reports whether a request carries the deployment's internal
// credential in the header forwarded MCP requests use.
func (c *mcpCluster) authorized(r *http.Request) bool {
	return c != nil && c.tokenErr == nil && internalBearerMatches(r, internalAuthHeader, c.token)
}

// internalMCPAuth admits only the other instances of the deployment.
func (h *Handler) internalMCPAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.mcpCluster.authorized(r) {
			writeError(w, http.StatusUnauthorized, "internal call not authorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// InternalMCPHandler serves the MCP requests other instances forward. It
// checks the internal credential, then the client's own exactly as the public
// endpoint does, so a forwarded call acts for the same user. It never
// forwards, which is what keeps a request from travelling twice. It belongs on
// the internal listener, never on the public one.
func (h *Handler) InternalMCPHandler() http.Handler {
	return h.internalMCPAuth(h.AgentAPIAuth(h.mcpStreamableHandler()))
}

// InternalMCPSessionsHandler serves this instance's sessions to the sessions
// view of the others. It belongs on the internal listener.
func (h *Handler) InternalMCPSessionsHandler() http.Handler {
	return h.internalMCPAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": h.mcpSessions.Snapshot()})
	}))
}

// peers lists the other live instances whose sessions the view includes.
func (c *mcpCluster) peers() []db.InstanceLocation {
	if c == nil || c.tokenErr != nil {
		return nil
	}
	self := c.directory.InstanceID()
	var out []db.InstanceLocation
	for _, instance := range c.directory.LiveInstances() {
		if instance.ID != self {
			out = append(out, instance)
		}
	}
	return out
}

// peerSessions asks every peer for its sessions at once, each within the
// bound, and names the peers that did not answer in time instead of failing.
func (c *mcpCluster) peerSessions(ctx context.Context, peers []db.InstanceLocation) ([]taskmcp.SessionView, []string) {
	answers := make([][]taskmcp.SessionView, len(peers))
	failed := make([]bool, len(peers))
	var wg sync.WaitGroup
	for i, peer := range peers {
		wg.Add(1)
		go func(i int, peer db.InstanceLocation) {
			defer wg.Done()
			sessions, err := c.sessionsOf(ctx, peer)
			if err != nil {
				log.Printf("[MCP] Sessions de l'instance %s indisponibles : %v", peer.ID, err)
				failed[i] = true
				return
			}
			answers[i] = sessions
		}(i, peer)
	}
	wg.Wait()
	var sessions []taskmcp.SessionView
	unreachable := []string{}
	for i, peer := range peers {
		if failed[i] {
			unreachable = append(unreachable, peer.ID)
			continue
		}
		sessions = append(sessions, answers[i]...)
	}
	return sessions, unreachable
}

func (c *mcpCluster) sessionsOf(ctx context.Context, peer db.InstanceLocation) ([]taskmcp.SessionView, error) {
	if strings.TrimSpace(peer.Address) == "" {
		return nil, fmt.Errorf("no internal address advertised")
	}
	ctx, cancel := context.WithTimeout(ctx, mcpPeerSessionsTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(peer.Address, "/")+internalMCPSessionsPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(internalAuthHeader, "Bearer "+c.token)
	resp, err := (&http.Client{Transport: c.transport}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("answered %s", resp.Status)
	}
	var body struct {
		Sessions []taskmcp.SessionView `json:"sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.Sessions, nil
}
