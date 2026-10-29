package backend

import (
	"net/http"

	"tasks/internal/handlers"
)

// gateWork keeps the Local process alive for probes, profile pairing and agent
// bootstrap while execution APIs wait for an authenticated self-connection.
// The narrow bootstrap list prevents a new API route from silently bypassing
// the gate. Fleet never uses this wrapper.
func (r *Runtime) gateWork(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if r.readyForWork() || localBootstrapPath(request.URL.Path) {
			next.ServeHTTP(w, request)
			return
		}
		w.Header().Set("Retry-After", "1")
		http.Error(w, "Le service local démarre.", http.StatusServiceUnavailable)
	})
}

func localBootstrapPath(path string) bool {
	switch path {
	case handlers.HealthPath,
		"/auth/local",
		"/api/me",
		"/api/pairing-codes",
		"/api/v1/agent/pair",
		"/ws/agent-connect",
		"/api/v1/agent/identity",
		"/api/v1/agent/config",
		"/api/v1/agent/projects",
		"/api/v1/agent/execution-seed",
		"/api/v1/agent/capabilities",
		"/api/v1/agent/run-output":
		return true
	default:
		return false
	}
}
