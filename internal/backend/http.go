package backend

import (
	"net/http"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/httpgzip"
	"tasks/internal/metrics"
	"tasks/internal/version"
)

// WrapHTTP applies the backend's authorization, compression, and metrics to
// the complete mux. The caller may register website assets before this call.
func WrapHTTP(mux *http.ServeMux, h *handlers.Handler, database *db.DB) http.Handler {
	build := version.Current()
	serverMetrics := metrics.New(database, db.ActiveUserWindow, metrics.Build{Version: build.Version, Commit: build.Commit})
	mux.Handle(metrics.Path, serverMetrics.Handler())
	return serverMetrics.Instrument(httpgzip.Handler(h.EnableCORS(h.RequireSession(mux))), routeOf(mux))
}

// routeOf reports the bounded route pattern rather than a path with user IDs.
func routeOf(mux *http.ServeMux) func(*http.Request) string {
	return func(r *http.Request) string {
		_, pattern := mux.Handler(r)
		return pattern
	}
}
