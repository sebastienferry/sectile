package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"tasks/internal/agentconfig"
	"testing"
)

func TestConversationProfilesPersistProjectDefaultsAndProtectUsedProfiles(t *testing.T) {
	d, config := disconnectFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/config" {
			_ = json.NewEncoder(w).Encode(config)
			return
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	d.link.serverURL = server.URL
	route := "/desktop/conversation-profiles?projectId=p"
	if got := conversationRequest(d, "GET", route, "", "wrong"); got.Code != 401 {
		t.Fatal("unauthenticated profile access")
	}
	for _, body := range []string{`{"action":"save","name":"Bad","provider":"custom"}`, `{"action":"save","name":"Bad","provider":"claude","mode":"bypassPermissions"}`, `{"action":"save","name":"Bad","provider":"claude","command":"shell"}`} {
		if got := disconnectRequest(d, "POST", route, body); got.Code != 400 {
			t.Fatalf("unsafe profile accepted: %d", got.Code)
		}
	}
	got := disconnectRequest(d, "POST", route, `{"action":"save","name":"Review with Codex","provider":"codex","model":"review-model","mode":"plan","makeDefault":true}`)
	if got.Code != 200 {
		t.Fatalf("save: %d %s", got.Code, got.Body.String())
	}
	var view struct {
		Profiles  []conversationProfile `json:"profiles"`
		DefaultID string                `json:"defaultId"`
		Inherited bool                  `json:"inherited"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		t.Fatal(err)
	}
	selected := settings.ProjectEngine("p")
	if selected.ID != view.DefaultID || selected.Provider != "codex" || selected.Model != "review-model" || selected.ConversationMode != "plan" || view.Inherited {
		t.Fatalf("profile defaults lost: %+v", selected)
	}
	settings.SetProjectEngine("other", selected.ID)
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"action": "delete", "id": selected.ID})
	if got := disconnectRequest(d, "POST", route, string(body)); got.Code != 400 {
		t.Fatal("profile used by another project was deleted")
	}
	if got := disconnectRequest(d, "POST", route, `{"action":"select","id":""}`); got.Code != 200 {
		t.Fatal(got.Body.String())
	}
	settings, err = agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil || settings.Engines.Projects["p"] != "" || settings.ProjectEngine("other").ID != selected.ID {
		t.Fatal("reset changed another project")
	}
}

func TestConversationProfileViewExcludesCommandTemplates(t *testing.T) {
	settings := agentconfig.Settings{Engines: agentconfig.Engines{Catalogue: []agentconfig.Engine{{ID: "safe", Provider: "claude"}, {ID: "template", Provider: "claude", Command: "claude {prompt}"}, {ID: "custom", Provider: "custom", Command: "run {prompt}"}}, Default: "template"}}
	view := conversationProfilesView(settings, "p")
	profiles := view["profiles"].([]conversationProfile)
	if len(profiles) != 1 || profiles[0].ID != "safe" || view["defaultId"] != "" {
		t.Fatal("command template exposed as simple profile")
	}
}

func TestConversationProfileLaunchKeepsModelAndModeIndependentOfLaterEdits(t *testing.T) {
	d, config := disconnectFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/config" {
			_ = json.NewEncoder(w).Encode(config)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	d.link.serverURL = server.URL
	t.Cleanup(d.stopConversations)
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		t.Fatal(err)
	}
	settings.Engines.Catalogue = append(settings.Engines.Catalogue, agentconfig.Engine{ID: "review-profile", Name: "Review", Provider: "codex", Model: "chosen-model", ConversationMode: "plan"})
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	got := disconnectRequest(d, "POST", "/desktop/consoles", `{"projectId":"p","provider":"claude","engineId":"review-profile","view":"conversation"}`)
	var entry desktopRun
	if got.Code != 202 || json.Unmarshal(got.Body.Bytes(), &entry) != nil {
		t.Fatalf("launch: %d %s", got.Code, got.Body.String())
	}
	run := d.queue.runs[entry.ID]
	if run.desktop.Provider != "codex" || run.desktop.Model != "chosen-model" || run.desktop.EngineID != "review-profile" || run.conversation.mode != "plan" {
		t.Fatal("profile not applied on launch")
	}
	got = disconnectRequest(d, "POST", "/desktop/conversation-profiles?projectId=p", `{"action":"save","id":"review-profile","name":"Implementation","provider":"claude","model":"different-model","mode":"execute"}`)
	if got.Code != 200 {
		t.Fatal(got.Body.String())
	}
	if run.desktop.Provider != "codex" || run.desktop.Model != "chosen-model" || run.conversation.mode != "plan" {
		t.Fatal("profile edit changed an open conversation")
	}

	settings, err = agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		t.Fatal(err)
	}
	settings.SetProjectEngine("p", "review-profile")
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	got = disconnectRequest(d, "POST", "/desktop/consoles", `{"projectId":"p","provider":"codex","view":"conversation"}`)
	if got.Code != 202 || json.Unmarshal(got.Body.Bytes(), &entry) != nil {
		t.Fatal(got.Body.String())
	}
	manual := d.queue.runs[entry.ID]
	if manual.desktop.Provider != "codex" || manual.desktop.Model != "" {
		t.Fatal("manual provider switch inherited another provider's model")
	}
}

func TestFirstConversationProfileKeepsImplicitWorkstationDefault(t *testing.T) {
	d, config := disconnectFixture(t)
	path, err := agentconfig.SettingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := agentconfig.ReadSettings(d.localSettingsRoot())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/config" {
			_ = json.NewEncoder(w).Encode(config)
			return
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	d.link.serverURL = server.URL
	got := disconnectRequest(d, "POST", "/desktop/conversation-profiles?projectId=p", `{"action":"save","name":"First project profile","provider":"codex","mode":"plan","makeDefault":true}`)
	if got.Code != 200 {
		t.Fatal(got.Body.String())
	}
	after, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		t.Fatal(err)
	}
	if after.DefaultEngine().Provider != before.DefaultEngine().Provider || after.ProjectEngine("p").Provider != "codex" {
		t.Fatal("first project profile replaced the workstation default")
	}
}
