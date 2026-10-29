package agentexec

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
)

func TestRunReadsControlTokenFromEnvironmentWithoutPassingItToChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the supervised command uses bash")
	}
	t.Setenv("SECTILE_RUN_TOKEN", "disposable-run-token")
	var exits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer disposable-run-token" {
			t.Error("run-control authorization was missing")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodPost {
			exits.Add(1)
		}
		_, _ = w.Write([]byte(`{"canceled":false}`))
	}))
	defer server.Close()
	if err := Run([]string{"--url", server.URL, "--command", `test -z "$SECTILE_RUN_TOKEN"`}); err != nil {
		t.Fatal(err)
	}
	if exits.Load() != 1 {
		t.Fatalf("expected one exit report, got %d", exits.Load())
	}
}
