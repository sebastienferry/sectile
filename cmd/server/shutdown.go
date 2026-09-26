package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"tasks/internal/db"
	"tasks/internal/handlers"
)

// defaultShutdownGrace is how long a stopping instance keeps serving after it
// reports not ready, so the load balancer stops sending it new traffic before
// it goes. The orchestrator's termination grace period must be longer.
const defaultShutdownGrace = 5 * time.Second

// shutdownDeadline bounds the requests still running once the grace is over.
const shutdownDeadline = 5 * time.Second

// shutdownGraceFromEnv reads SECTILE_SHUTDOWN_GRACE, a Go duration. Zero is
// allowed and skips the wait; anything unparsable or negative is an error.
func shutdownGraceFromEnv(getenv func(string) string) (time.Duration, error) {
	raw := strings.TrimSpace(getenv("SECTILE_SHUTDOWN_GRACE"))
	if raw == "" {
		return defaultShutdownGrace, nil
	}
	grace, err := time.ParseDuration(raw)
	if err != nil || grace < 0 {
		return 0, fmt.Errorf("SECTILE_SHUTDOWN_GRACE=%q is not a duration such as 5s", raw)
	}
	return grace, nil
}

// instanceTimingFromEnv reads the liveness bounds a test harness shortens:
// SECTILE_INSTANCE_HEARTBEAT, SECTILE_INSTANCE_DEAD_AFTER and
// SECTILE_INSTANCE_RECLAIM, Go durations. Unset keeps the default; a value
// that is not a positive duration is an error. They are not meant to be tuned
// in production: a dead-after bound shorter than a few heartbeats reclaims the
// work of an instance that is merely slow.
func instanceTimingFromEnv(getenv func(string) string) (heartbeat, deadAfter, reclaim time.Duration, err error) {
	read := func(name string) (time.Duration, error) {
		raw := strings.TrimSpace(getenv(name))
		if raw == "" {
			return 0, nil
		}
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 {
			return 0, fmt.Errorf("%s=%q is not a positive duration such as 10s", name, raw)
		}
		return value, nil
	}
	if heartbeat, err = read("SECTILE_INSTANCE_HEARTBEAT"); err != nil {
		return
	}
	if deadAfter, err = read("SECTILE_INSTANCE_DEAD_AFTER"); err != nil {
		return
	}
	reclaim, err = read("SECTILE_INSTANCE_RECLAIM")
	return
}

// drain takes a stopping instance out of service in the order that loses the
// least: readiness first, so the balancer stops routing to it; then, after the
// grace, its instance row, so the others take over its work at once instead of
// after the dead-after bound; then its agent connections, so the agents
// reconnect to an instance that stays; and last the requests still running.
// MCP sessions held here end with the process, as they do on any stop.
func drain(h *handlers.Handler, stopInstance func(), server *http.Server, grace time.Duration) {
	h.BeginDrain()
	log.Printf("Arrêt demandé : instance retirée du service, %s de grâce", grace)
	time.Sleep(grace)
	stopInstance()
	h.CloseAgentConnections()
	ctx, cancel := context.WithTimeout(context.Background(), shutdownDeadline)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("⚠️  Arrêt du serveur : %v", err)
	}
	log.Printf("Instance %s arrêtée", h.InstanceID())
}

// applyInstanceTiming installs the liveness bounds the environment asks for.
func applyInstanceTiming(getenv func(string) string) error {
	heartbeat, deadAfter, reclaim, err := instanceTimingFromEnv(getenv)
	if err != nil {
		return err
	}
	db.SetInstanceTiming(heartbeat, deadAfter, reclaim)
	return nil
}
