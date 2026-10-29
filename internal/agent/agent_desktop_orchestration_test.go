package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"testing"
)

func TestDesktopOrchestrationSettingsAndEvaluationRelay(t *testing.T) {
	var mu sync.Mutex
	var writes []map[string]any
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer paired" {
			t.Error("paired credential was lost")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/projects/p" && r.URL.Path != "/api/projects/p/orchestrate" {
			t.Errorf("unexpected route: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		if r.Method != "GET" {
			var input map[string]any
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
			}
			mu.Lock()
			writes = append(writes, input)
			methods = append(methods, r.Method)
			mu.Unlock()
		}
		if r.URL.Path == "/api/projects/p/orchestrate" {
			w.WriteHeader(409)
			_, _ = w.Write([]byte(`{"error":"Project is disabled"}`))
			return
		}
		_, _ = w.Write([]byte(`{"orchestratorEnabled":true,"orchestratorMaxConcurrent":3,"orchestratorPrompt":"Only small fixes","repoPath":"/server/private"}`))
	}))
	defer server.Close()
	d := taskPageDaemon(t, server)
	d.link.projectID = "p"
	if _, err := gitLocal(context.Background(), d.repoRoot, "init"); err != nil {
		t.Fatal(err)
	}
	if err := agentconfig.WriteSettings(agentconfig.Settings{Seeded: agentconfig.Seeded{Defaults: server.URL, Projects: map[string]string{"p": "now"}}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d.repoRoot, "AGENTS.md"), []byte("Only local small fixes"), 0600); err != nil {
		t.Fatal(err)
	}
	get := taskPageRequest(d, "GET", "/desktop/orchestration?projectId=p", "")
	if get.Code != 200 {
		t.Fatal(get.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	views := result["agents"].([]any)
	first := views[0].(map[string]any)
	if len(views) != 1 || first["guidelines"] != "Only local small fixes" || first["file"] != "AGENTS.md" || first["id"] != "default" || result["repoPath"] != nil || result["orchestratorPrompt"] != nil {
		t.Fatalf("unexpected settings: %v", result)
	}
	save := taskPageRequest(d, "PUT", "/desktop/orchestration?projectId=p", `{"orchestratorEnabled":true,"orchestratorMaxConcurrent":3,"agents":[{"id":"default","name":"Default agent","file":"AGENTS.md"}]}`)
	if save.Code != 200 {
		t.Fatal(save.Body.String())
	}
	if err := os.WriteFile(filepath.Join(d.repoRoot, "AGENTS.md"), []byte("Fresh local guidelines"), 0600); err != nil {
		t.Fatal(err)
	}
	evaluate := taskPageRequest(d, "POST", "/desktop/orchestration?projectId=p", `{}`)
	if evaluate.Code != 409 {
		t.Fatalf("server refusal lost: %d %s", evaluate.Code, evaluate.Body.String())
	}
	for _, body := range []string{`{"force":true}`, `{"maxConcurrent":10}`, `bad`} {
		if response := taskPageRequest(d, "POST", "/desktop/orchestration?projectId=p", body); response.Code != 400 {
			t.Fatalf("accepted override: %s", body)
		}
	}
	for _, body := range []string{`{}`, `{"orchestratorEnabled":true,"orchestratorMaxConcurrent":11,"orchestrationFile":""}`, `{"orchestratorEnabled":true,"orchestratorMaxConcurrent":1,"orchestrationFile":"","ownerUserId":"other"}`} {
		if response := taskPageRequest(d, "PUT", "/desktop/orchestration?projectId=p", body); response.Code != 400 {
			t.Fatalf("accepted invalid settings: %s", body)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(writes) != 2 || methods[0] != "PUT" || methods[1] != "POST" || len(writes[0]) != 2 || writes[0]["orchestratorPrompt"] != nil || len(writes[1]) != 2 || writes[1]["profileId"] != "default" || writes[1]["guidelines"] != nil {
		t.Fatalf("unexpected upstream writes: %v %v", methods, writes)
	}
}

func TestDesktopOrchestrationReportsUnsupportedServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"id":"p"}`)) }))
	defer server.Close()
	d := taskPageDaemon(t, server)
	if response := taskPageRequest(d, "GET", "/desktop/orchestration?projectId=p", ""); response.Code != 501 {
		t.Fatalf("old server treated as supported: %d", response.Code)
	}
	if response := taskPageRequest(d, "GET", "/desktop/orchestration", ""); response.Code != 400 {
		t.Fatal("missing project accepted")
	}
	if response := taskPageRequest(d, "PATCH", "/desktop/orchestration?projectId=p", ""); response.Code != 405 {
		t.Fatal("unsupported method accepted")
	}
}

func TestOrchestrationGuidelinesFileBoundaries(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.md")); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{
		"AGENTS.md": []byte("# Local policy\nOnly small fixes."),
		"empty.md":  []byte(" "), "invalid.md": {0xff},
		"large.md": []byte(strings.Repeat("x", models.OrchestratorGuidelinesMaxBytes+1)),
	} {
		if err := os.WriteFile(filepath.Join(root, name), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := readOrchestrationGuidelines(root, "AGENTS.md"); err != nil || !strings.Contains(got, "Only small fixes") {
		t.Fatalf("valid policy: %q, %v", got, err)
	}
	for _, file := range []string{"../outside.md", outside, "escape.md", "missing.md", "empty.md", "invalid.md", "large.md", "README.txt"} {
		if _, err := readOrchestrationGuidelines(root, file); err == nil {
			t.Errorf("accepted invalid policy %s", file)
		}
	}
}

func TestDesktopOrchestrationCustomFileAndMissingFileRefusal(t *testing.T) {
	// The policy text never travels: the server learns which agent to run,
	// and the workstation reads its file at every session launch.
	var started models.OrchestrateProjectRequest
	var stopped bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/projects/p/orchestrate" && r.Method == http.MethodPost:
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&started); err != nil {
				t.Errorf("start request: %v", err)
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"session":{"id":"run-1"}}`))
			return
		case r.URL.Path == "/api/projects/p/orchestrate/stop":
			stopped = true
			_, _ = w.Write([]byte(`{"projectId":"p"}`))
			return
		case r.URL.Path == "/api/projects/p/orchestrate":
			_, _ = w.Write([]byte(`{"projectId":"p","maxConcurrent":2,"decisions":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"p","orchestratorEnabled":true,"orchestratorMaxConcurrent":2}`))
	}))
	defer server.Close()
	d := taskPageDaemon(t, server)
	d.link.projectID = "p"
	if _, err := gitLocal(context.Background(), d.repoRoot, "init"); err != nil {
		t.Fatal(err)
	}
	settings := agentconfig.Settings{Seeded: agentconfig.Seeded{Defaults: server.URL, Projects: map[string]string{"p": "now"}}}
	settings.SetProject("p", agentconfig.ProjectSettings{OrchestrationFile: "policy.md"})
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	get := taskPageRequest(d, "GET", "/desktop/orchestration?projectId=p", "")
	if get.Code != 200 || !strings.Contains(get.Body.String(), "guidelinesError") {
		t.Fatal(get.Body.String())
	}
	for _, method := range []string{"PUT", "POST"} {
		body := `{}`
		if method == "PUT" {
			body = `{"orchestratorEnabled":true,"orchestratorMaxConcurrent":2,"agents":[{"id":"default","name":"Default agent","file":"policy.md"}]}`
		}
		if got := taskPageRequest(d, method, "/desktop/orchestration?projectId=p", body); got.Code != 400 {
			t.Fatalf("%s accepted missing guidelines: %d", method, got.Code)
		}
	}
	if err := os.WriteFile(filepath.Join(d.repoRoot, "policy.md"), []byte("Custom local policy"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := taskPageRequest(d, "POST", "/desktop/orchestration?projectId=p", `{}`); got.Code != http.StatusAccepted || started.ProfileID != "default" || started.ProfileName != "Default agent" {
		t.Fatalf("start did not name the agent: %d %+v", got.Code, started)
	}
	if got := taskPageRequest(d, "DELETE", "/desktop/orchestration?projectId=p", ""); got.Code != 200 || !stopped {
		t.Fatalf("stop was not relayed: %d", got.Code)
	}
	if err := os.Remove(filepath.Join(d.repoRoot, "policy.md")); err != nil {
		t.Fatal(err)
	}
	// Disabling remains possible when the previously selected file disappeared.
	if got := taskPageRequest(d, "PUT", "/desktop/orchestration?projectId=p", `{"orchestratorEnabled":false,"orchestratorMaxConcurrent":2,"agents":[{"id":"default","name":"Default agent","file":"policy.md"}]}`); got.Code != 200 {
		t.Fatal(got.Body.String())
	}
}

func TestDesktopNamedOrchestrationAgentsSelectTheirOwnPolicy(t *testing.T) {
	var started []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer paired" {
			t.Error("paired identity lost")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/projects/p/orchestrate" && r.Method == http.MethodPost {
			var input map[string]any
			_ = json.NewDecoder(r.Body).Decode(&input)
			if len(input) != 2 || input["guidelines"] != nil {
				t.Errorf("agent selection sent more than the agent's identity: %v", input)
			}
			started = append(started, fmt.Sprint(input["profileId"]))
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"session":{"id":"run"}}`))
			return
		}
		if r.Method == "PUT" {
			var input map[string]any
			_ = json.NewDecoder(r.Body).Decode(&input)
			if len(input) != 2 || input["agents"] != nil {
				t.Error("local definitions leaked into shared settings")
			}
		}
		_, _ = w.Write([]byte(`{"id":"p","orchestratorEnabled":true,"orchestratorMaxConcurrent":2}`))
	}))
	defer server.Close()
	d := taskPageDaemon(t, server)
	d.link.projectID = "p"
	if _, err := gitLocal(context.Background(), d.repoRoot, "init"); err != nil {
		t.Fatal(err)
	}
	if err := agentconfig.WriteSettings(agentconfig.Settings{Seeded: agentconfig.Seeded{Defaults: server.URL, Projects: map[string]string{"p": "now"}}}); err != nil {
		t.Fatal(err)
	}
	for file, text := range map[string]string{"docs.md": "Documentation policy only", "tests.md": "Regression test policy only"} {
		if err := os.WriteFile(filepath.Join(d.repoRoot, file), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	route := "/desktop/orchestration?projectId=p"
	body := `{"orchestratorEnabled":true,"orchestratorMaxConcurrent":2,"agents":[{"id":"docs","name":"Documentation","file":"docs.md"},{"id":"tests","name":"Tests","file":"tests.md"}]}`
	if got := taskPageRequest(d, "PUT", route, body); got.Code != 200 {
		t.Fatal(got.Body.String())
	}
	for _, id := range []string{"docs", "tests"} {
		if got := taskPageRequest(d, "POST", route, `{"agentId":"`+id+`"}`); got.Code != http.StatusAccepted {
			t.Fatal(got.Body.String())
		}
	}
	if len(started) != 2 || started[0] != "docs" || started[1] != "tests" {
		t.Fatalf("wrong agent started: %v", started)
	}
	// Each session reads its own agent's file when the workstation launches it.
	docs, err := d.orchestrationProfilePolicy("p", d.repoRoot, "docs")
	if err != nil || !strings.Contains(docs, "Documentation [docs]") || !strings.Contains(docs, "Documentation policy only") || strings.Contains(docs, "Regression test policy") {
		t.Fatalf("docs policy: %q %v", docs, err)
	}
	tests, err := d.orchestrationProfilePolicy("p", d.repoRoot, "tests")
	if err != nil || !strings.Contains(tests, "Tests [tests]") || !strings.Contains(tests, "Regression test policy only") {
		t.Fatalf("tests policy: %q %v", tests, err)
	}
	if _, err := d.orchestrationProfilePolicy("p", d.repoRoot, "removed"); err == nil {
		t.Fatal("a removed agent's session ran without a policy")
	}
	for _, body := range []string{`{}`, `{"agentId":"missing"}`, `{"agentId":"docs","force":true}`, `{"agentId":"docs","maxConcurrent":10}`} {
		if got := taskPageRequest(d, "POST", route, body); got.Code != 400 {
			t.Fatalf("accepted invalid selection %s: %d", body, got.Code)
		}
	}
	if err := os.WriteFile(filepath.Join(d.repoRoot, "docs.md"), []byte("Updated documentation policy"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(d.repoRoot, "tests.md")); err != nil {
		t.Fatal(err)
	}
	if got := taskPageRequest(d, "POST", route, `{"agentId":"docs"}`); got.Code != http.StatusAccepted {
		t.Fatal(got.Body.String())
	}
	if policy, err := d.orchestrationProfilePolicy("p", d.repoRoot, "docs"); err != nil || !strings.Contains(policy, "Updated documentation policy") {
		t.Fatalf("selected file was not read afresh: %q %v", policy, err)
	}
	if got := taskPageRequest(d, "POST", route, `{"agentId":"tests"}`); got.Code != 400 {
		t.Fatal("missing selected file launched")
	}
	// An unrelated missing policy must not block the valid saved agent.
	if got := taskPageRequest(d, "PUT", route, `{"orchestratorEnabled":true,"orchestratorMaxConcurrent":2,"agents":[]}`); got.Code != 400 {
		t.Fatal("enabled project accepted no agents")
	}
	if got := taskPageRequest(d, "PUT", route, `{"orchestratorEnabled":false,"orchestratorMaxConcurrent":2,"agents":[]}`); got.Code != 200 {
		t.Fatal(got.Body.String())
	}
	if got := taskPageRequest(d, "GET", route, ""); !strings.Contains(got.Body.String(), `"agents":[]`) {
		t.Fatal("empty list restored the default agent")
	}
	if got := taskPageRequest(d, "POST", route, `{"agentId":"docs"}`); got.Code != 400 {
		t.Fatal("removed agent launched")
	}
}

func TestNamedOrchestrationGuidelinesBoundTheIdentityAndFileTogether(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(strings.Repeat("x", models.OrchestratorGuidelinesMaxBytes)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readOrchestrationAgentGuidelines(root, agentconfig.OrchestrationAgent{ID: "docs", Name: "Docs", File: "AGENTS.md"}); err == nil {
		t.Fatal("agent identity bypassed the snapshot bound")
	}
}

// Removing the agent this workstation started stops orchestration; an agent
// started from another workstation is not this one's to stop.
func TestRemovingTheStartedAgentStopsOrchestration(t *testing.T) {
	var armedDevice string
	var stops int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/projects/p/orchestrate/stop":
			stops++
			_, _ = w.Write([]byte(`{"projectId":"p"}`))
		case r.URL.Path == "/api/projects/p/orchestrate":
			_, _ = w.Write([]byte(`{"projectId":"p","armed":{"projectId":"p","profileId":"docs","deviceId":"` + armedDevice + `"},"decisions":[]}`))
		default:
			_, _ = w.Write([]byte(`{"id":"p","orchestratorEnabled":true,"orchestratorMaxConcurrent":2}`))
		}
	}))
	defer server.Close()
	d := taskPageDaemon(t, server)
	d.link.projectID = "p"
	d.link.deviceID = "this-workstation"
	if _, err := gitLocal(context.Background(), d.repoRoot, "init"); err != nil {
		t.Fatal(err)
	}
	if err := agentconfig.WriteSettings(agentconfig.Settings{Seeded: agentconfig.Seeded{Defaults: server.URL, Projects: map[string]string{"p": "now"}}}); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"docs.md", "tests.md"} {
		if err := os.WriteFile(filepath.Join(d.repoRoot, file), []byte("Policy"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	route := "/desktop/orchestration?projectId=p"
	both := `{"orchestratorEnabled":true,"orchestratorMaxConcurrent":2,"agents":[{"id":"docs","name":"Documentation","file":"docs.md"},{"id":"tests","name":"Tests","file":"tests.md"}]}`
	onlyTests := `{"orchestratorEnabled":true,"orchestratorMaxConcurrent":2,"agents":[{"id":"tests","name":"Tests","file":"tests.md"}]}`

	armedDevice = "this-workstation"
	if got := taskPageRequest(d, "PUT", route, both); got.Code != 200 || stops != 0 {
		t.Fatalf("keeping the started agent stopped it: %d, %d stops", got.Code, stops)
	}
	armedDevice = "another-workstation"
	if got := taskPageRequest(d, "PUT", route, onlyTests); got.Code != 200 || stops != 0 {
		t.Fatalf("another workstation's agent was stopped: %d, %d stops", got.Code, stops)
	}
	armedDevice = "this-workstation"
	if got := taskPageRequest(d, "PUT", route, onlyTests); got.Code != 200 || stops != 1 {
		t.Fatalf("removing the started agent did not stop it: %d, %d stops", got.Code, stops)
	}
}
