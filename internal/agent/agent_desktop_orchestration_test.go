package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
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
	get := taskPageRequest(d, "GET", "/desktop/orchestration?projectId=p", "")
	if get.Code != 200 {
		t.Fatal(get.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["orchestratorPrompt"] != "Only small fixes" || result["repoPath"] != nil || len(result) != 3 {
		t.Fatalf("unexpected settings: %v", result)
	}
	save := taskPageRequest(d, "PUT", "/desktop/orchestration?projectId=p", `{"orchestratorEnabled":true,"orchestratorMaxConcurrent":3,"orchestratorPrompt":"Only small fixes"}`)
	if save.Code != 200 {
		t.Fatal(save.Body.String())
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
	for _, body := range []string{`{}`, `{"orchestratorEnabled":true,"orchestratorMaxConcurrent":11,"orchestratorPrompt":""}`, `{"orchestratorEnabled":true,"orchestratorMaxConcurrent":1,"orchestratorPrompt":"","ownerUserId":"other"}`} {
		if response := taskPageRequest(d, "PUT", "/desktop/orchestration?projectId=p", body); response.Code != 400 {
			t.Fatalf("accepted invalid settings: %s", body)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(writes) != 2 || methods[0] != "PUT" || methods[1] != "POST" || len(writes[0]) != 3 || len(writes[1]) != 0 {
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
	if response := taskPageRequest(d, "DELETE", "/desktop/orchestration?projectId=p", ""); response.Code != 405 {
		t.Fatal("unsupported method accepted")
	}
}
