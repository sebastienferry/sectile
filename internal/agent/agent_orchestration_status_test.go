package agent

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFailedProviderAuthenticationIsReportedWithoutRawConsoleOutput(t *testing.T) {
	trace := newRunTrace()
	trace.write(traceLine("Failed to authenticate: OAuth session expired and cannot be refreshed. private diagnostic"))
	trace.close()
	for _, test := range []struct {
		status, provider string
		want             bool
	}{
		{"failed", "claude", true}, {"failed", "codex", true}, {"running", "claude", false}, {"completed", "claude", false}, {"canceled", "claude", false}, {"failed", "custom", false},
	} {
		if got := runNeedsAuthentication(desktopRun{Status: test.status, Provider: test.provider}, trace); got != test.want {
			t.Errorf("%s/%s authentication required = %v, want %v", test.status, test.provider, got, test.want)
		}
	}
	if runNeedsAuthentication(desktopRun{Status: "failed", Provider: "claude"}, nil) {
		t.Fatal("missing trace invented an authentication failure")
	}
	d := restoredDaemon(t)
	d.queue.runs["headless"].desktop.Provider = "claude"
	d.queue.runs["headless"].trace = trace
	request := httptest.NewRequest("GET", "/desktop/runs", nil)
	request.Header.Set("Authorization", "Bearer private")
	response := httptest.NewRecorder()
	d.desktopHandler(response, request)
	var runs []desktopRun
	if err := json.Unmarshal(response.Body.Bytes(), &runs); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, run := range runs {
		if run.ID == "headless" {
			found = true
			if !run.AuthenticationRequired || !run.Restored {
				t.Fatalf("restored failure lost its recovery state: %+v", run)
			}
		}
	}
	if !found || strings.Contains(response.Body.String(), "private diagnostic") {
		t.Fatal("discovery omitted the failed run or exposed raw output")
	}
}

func TestOrdinaryProviderFailureDoesNotAskForSignIn(t *testing.T) {
	trace := newRunTrace()
	trace.write(traceLine("Build failed: missing package"))
	if runNeedsAuthentication(desktopRun{Status: "failed", Provider: "claude"}, trace) {
		t.Fatal("build failure was classified as expired authentication")
	}
}
