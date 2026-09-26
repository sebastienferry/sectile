package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// ReadyPath is the server's readiness route. /api/health says the process
// serves HTTP, which is what a liveness probe needs to know; this one says
// whether a load balancer should send it traffic: the database answers, and,
// when other instances share it, this one is registered where they find it and
// its internal listener serves. A stopping instance answers not ready at once,
// so the balancer drains it before it goes (#410).
const ReadyPath = "/api/ready"

// readyProbeTimeout bounds each database check of the probe, so a database
// that stopped answering makes the probe fail rather than hang.
const readyProbeTimeout = 2 * time.Second

// SetInternalServing records whether the internal listener the other instances
// reach this one on is serving.
func (h *Handler) SetInternalServing(serving bool) { h.internalServing.Store(serving) }

// BeginDrain makes the readiness probe answer not ready from now on, for an
// instance that is stopping.
func (h *Handler) BeginDrain() { h.draining.Store(true) }

// HandleReady answers the readiness probe.
func (h *Handler) HandleReady(w http.ResponseWriter, r *http.Request) {
	if reason := h.notReadyReason(r.Context()); reason != "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": reason})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "instance": h.db.InstanceID()})
}

func (h *Handler) notReadyReason(ctx context.Context) string {
	if h.draining.Load() {
		return "this server instance is stopping"
	}
	ctx, cancel := context.WithTimeout(ctx, readyProbeTimeout)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		return "the database does not answer: " + err.Error()
	}
	if !h.db.Shared() {
		return ""
	}
	registered, err := h.db.InstanceRegistered(ctx)
	if err != nil {
		return "cannot read this instance's registration: " + err.Error()
	}
	if !registered {
		return "this instance is not registered among the server instances yet"
	}
	if !h.internalServing.Load() {
		return "the internal listener the other instances reach this one on is not serving"
	}
	return ""
}

// CloseAgentConnections closes every local agent connection with "going away",
// so the agents reconnect, through the load balancer, to an instance that
// stays. Each connection's handler then unregisters it, which records the
// departure for the other instances.
func (h *Handler) CloseAgentConnections() int {
	d := h.agentDispatcher
	d.mu.RLock()
	conns := make([]*AgentConn, 0, len(d.agents))
	for _, ac := range d.agents {
		conns = append(conns, ac)
	}
	d.mu.RUnlock()
	for _, ac := range conns {
		ac.Close(websocket.CloseGoingAway, "Server Stopping")
	}
	if len(conns) > 0 {
		log.Printf("[AgentDispatcher] %d connexion(s) d'agent fermée(s) avant l'arrêt", len(conns))
	}
	return len(conns)
}

// InstanceID names the server instance this handler serves.
func (h *Handler) InstanceID() string { return h.db.InstanceID() }
