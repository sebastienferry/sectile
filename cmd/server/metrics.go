package main

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// metricsHandler guards the Prometheus metrics with SECTILE_METRICS_TOKEN when
// it is set: a scraper then sends it as a bearer token. Without it the route is
// open, since it sits outside /api/ and the session guard, and keeping it off
// the public ingress is the deployment's job.
func metricsHandler(next http.Handler, token string) http.Handler {
	token = strings.TrimSpace(token)
	if token == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(sent)), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "metrics token required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// routeOf names the controller a request reaches: the mux pattern, which is a
// bounded set, rather than the path, which carries ids.
func routeOf(mux *http.ServeMux) func(*http.Request) string {
	return func(r *http.Request) string {
		_, pattern := mux.Handler(r)
		return pattern
	}
}
