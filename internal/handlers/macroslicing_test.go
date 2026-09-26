package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/handlers"
)

// The slicing import reads the specifications on the user's workstation: a
// missing or outdated desktop app is said as such, an agent's refusal is
// shown without the relay's prefix, and the stories source never asks the
// agent.
func TestSlicingImportExplainsAgentFailures(t *testing.T) {
	database, _, server, project := macroRunFixture(t)
	cookie := defaultSession(t, database)
	post := func(source string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/projects/"+project.ID+"/macros/M-7/slicing", strings.NewReader(`{"source":"`+source+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(raw)
	}

	for _, tc := range []struct {
		name    string
		failure error
		want    string
	}{
		{"outdated agent", &agentprotocol.UnsupportedOperationError{Device: "laptop", Build: "v0.3.0", Operation: "macro_spec_file"}, "trop ancienne"},
		{"legacy agent", &agentprotocol.UnsupportedOperationError{Device: "laptop", Operation: "macro_spec_file"}, "trop ancienne"},
		// The type decides, not the text: an agent's reply quoting the words is
		// a refusal of its own.
		{"refusal quoting the text", errors.New(`local agent: unknown local operation "macro_spec_file"`), "unknown local operation"},
		{"no agent", fmt.Errorf("%w for project %s", handlers.ErrNoAgentConnected, project.ID), "connectez l'app desktop"},
		{"agent refusal", errors.New("local agent: aucun dossier de spécification pour M-7 dans /x"), `"aucun dossier de spécification pour M-7 dans /x"`},
	} {
		database.SetAgentOperations(func(context.Context, agentprotocol.Operation) (json.RawMessage, error) { return nil, tc.failure })
		status, body := post("tasks")
		if status != http.StatusBadRequest || !strings.Contains(body, tc.want) {
			t.Errorf("%s: expected 400 with %q, got %d %s", tc.name, tc.want, status, body)
		}
	}

	database.SetAgentOperations(func(context.Context, agentprotocol.Operation) (json.RawMessage, error) {
		t.Error("the stories source must not ask the agent")
		return nil, errors.New("unexpected")
	})
	if status, body := post("stories"); strings.Contains(body, "desktop") {
		t.Errorf("the stories source must not depend on the desktop app: %d %s", status, body)
	}
}

// An older client still sends the server specifications path: it is ignored,
// the update succeeds, and nothing echoes it back.
func TestProjectUpdateIgnoresTheRetiredSpecificationsPath(t *testing.T) {
	database, _, server, project := macroRunFixture(t)
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/api/projects/"+project.ID, strings.NewReader(`{"name":"Renamed","specRepoPath":"/server/wiki"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(defaultSession(t, database))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(raw), "Renamed") || strings.Contains(string(raw), "specRepoPath") {
		t.Fatalf("expected the update to succeed without the path, got %d %s", resp.StatusCode, raw)
	}
}
