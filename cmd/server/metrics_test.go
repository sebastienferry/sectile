package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func serveMetrics(t *testing.T, token, authorization string) int {
	t.Helper()
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	recorder := httptest.NewRecorder()
	metricsHandler(ok, token).ServeHTTP(recorder, request)
	return recorder.Code
}

func TestMetricsAreOpenWithoutAToken(t *testing.T) {
	if code := serveMetrics(t, "", ""); code != http.StatusOK {
		t.Fatalf("no token configured: %d, want 200", code)
	}
}

func TestMetricsTokenIsRequiredOnceSet(t *testing.T) {
	cases := map[string]int{
		"":                 http.StatusUnauthorized,
		"Bearer wrong":     http.StatusUnauthorized,
		"Basic s3cret":     http.StatusUnauthorized,
		"Bearer s3cret":    http.StatusOK,
		"Bearer  s3cret  ": http.StatusOK,
	}
	for authorization, want := range cases {
		if code := serveMetrics(t, "s3cret", authorization); code != want {
			t.Errorf("Authorization %q: %d, want %d", authorization, code, want)
		}
	}
}
