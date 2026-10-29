package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"tasks/internal/localruntime"
)

// localRuntimeProbe authenticates discovery of a companion-owned server. The
// random key is installation-local and is never accepted as a user credential.
func localRuntimeProbe(next http.Handler, key string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/local-runtime" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if key == "" || r.Method != http.MethodGet || r.Header.Get("Origin") != "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+key)) != 1 {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			localruntime.Metadata
			Ready bool `json:"ready"`
		}{localruntime.Info(), true})
	})
}
