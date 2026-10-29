package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalRuntimeProbeRequiresOwnKeyAndRejectsBrowserOrigins(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	handler := localRuntimeProbe(next, "private-key")
	for _, tc := range []struct {
		path, token, origin string
		status              int
	}{
		{"/api/local-runtime", "private-key", "", 200},
		{"/api/local-runtime", "wrong", "", 404},
		{"/api/local-runtime", "private-key", "http://example.com", 404},
		{"/api/version", "", "", 204},
	} {
		req := httptest.NewRequest("GET", tc.path, nil)
		req.Header.Set("Authorization", "Bearer "+tc.token)
		req.Header.Set("Origin", tc.origin)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != tc.status {
			t.Fatalf("%s: got %d, want %d", tc.path, res.Code, tc.status)
		}
	}
}
