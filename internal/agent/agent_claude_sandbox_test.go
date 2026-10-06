package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/testhome"
)

// The capture of a real "can_use_tool" request of Claude Code 2.1.286, made
// for #700: Claude proposes to write its rule into localSettings.
func alwaysAllowFixture(t *testing.T) conversationApproval {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "conversation_always_allow.json"))
	if err != nil {
		t.Fatal(err)
	}
	var control conversationControlRequest
	if err := json.Unmarshal(raw, &control); err != nil {
		t.Fatal(err)
	}
	return conversationApproval{ID: control.RequestID, Tool: control.Request.ToolName, Input: control.Request.Input, Suggestions: control.Request.Suggestions}
}

// updatedPermissions reads back what a decision hands Claude.
func updatedPermissions(t *testing.T, response map[string]any) []map[string]any {
	t.Helper()
	data, _ := json.Marshal(response)
	var frame struct {
		Response struct {
			Response struct {
				UpdatedPermissions []map[string]any `json:"updatedPermissions"`
			} `json:"response"`
		} `json:"response"`
	}
	if err := json.Unmarshal(data, &frame); err != nil {
		t.Fatal(err)
	}
	return frame.Response.Response.UpdatedPermissions
}

func TestAlwaysAllowKeepsTheRuleForTheSessionOnly(t *testing.T) {
	response, rules, unread := approvalDecision(alwaysAllowFixture(t), "always", nil)
	updates := updatedPermissions(t, response)
	if len(updates) != 1 || updates[0]["destination"] != "session" || updates[0]["type"] != "addRules" || updates[0]["behavior"] != "allow" {
		t.Fatalf("updates = %v", updates)
	}
	if data, _ := json.Marshal(response); strings.Contains(string(data), "localSettings") {
		t.Fatalf("a persistent destination survived: %s", data)
	}
	if !reflect.DeepEqual(rules, []string{"Bash(make fixture-seven-hundred *)"}) || unread != nil {
		t.Fatalf("rules = %v, unread = %v", rules, unread)
	}
}

func TestAllowAndDenyCarryNoRule(t *testing.T) {
	for _, decision := range []string{"allow", "deny"} {
		response, rules, unread := approvalDecision(alwaysAllowFixture(t), decision, nil)
		if data, _ := json.Marshal(response); strings.Contains(string(data), "updatedPermissions") || rules != nil || unread != nil {
			t.Fatalf("%s: %s %v %v", decision, data, rules, unread)
		}
	}
}

// A mode change or a directory is applied to the conversation and adds no rule
// to the project.
func TestAlwaysAllowWithoutARulePersistsNothing(t *testing.T) {
	approval := conversationApproval{ID: "r", Suggestions: json.RawMessage(`[{"type":"setMode","mode":"acceptEdits","destination":"session"},{"type":"addDirectories","directories":["/x"],"destination":"localSettings"},{"type":"addRules","rules":[{"toolName":"Bash","ruleContent":"rm *"}],"behavior":"deny","destination":"projectSettings"}]`)}
	response, rules, unread := approvalDecision(approval, "always", nil)
	updates := updatedPermissions(t, response)
	if len(updates) != 3 || rules != nil || unread != nil {
		t.Fatalf("updates = %v, rules = %v, unread = %v", updates, rules, unread)
	}
	for _, update := range updates {
		if update["destination"] != "session" {
			t.Fatalf("destination kept: %v", update)
		}
	}
	if updates[0]["mode"] != "acceptEdits" {
		t.Fatalf("an update lost a field: %v", updates[0])
	}
}

// A suggestion Sectile does not recognise still reaches Claude, for the
// conversation only, and is reported rather than dropped.
func TestAnUnreadableSuggestionIsPassedOnForTheSession(t *testing.T) {
	approval := conversationApproval{ID: "r", Suggestions: json.RawMessage(`[{"type":"addRules","rules":[{"ruleContent":"x"}],"behavior":"allow","destination":"localSettings"},"odd"]`)}
	response, rules, unread := approvalDecision(approval, "always", nil)
	data, _ := json.Marshal(response)
	if rules != nil || len(unread) != 2 || strings.Contains(string(data), "localSettings") || !strings.Contains(string(data), `"odd"`) {
		t.Fatalf("response = %s, rules = %v, unread = %v", data, rules, unread)
	}
	approval.Suggestions = json.RawMessage(`{"type":"addRules"}`)
	response, rules, unread = approvalDecision(approval, "always", nil)
	data, _ = json.Marshal(response)
	if rules != nil || len(unread) != 1 || !strings.Contains(string(data), `"updatedPermissions":{"type":"addRules"}`) {
		t.Fatalf("a non-list suggestion was not passed on verbatim: %s", data)
	}
}

func TestPermissionRulesRenderClaudesShape(t *testing.T) {
	rules, ok := permissionRules([]any{map[string]any{"toolName": "Read"}, map[string]any{"toolName": "WebFetch", "ruleContent": "domain:example.com"}})
	if !ok || !reflect.DeepEqual(rules, []string{"Read", "WebFetch(domain:example.com)"}) {
		t.Fatalf("rules = %v %v", rules, ok)
	}
	if _, ok := permissionRules([]any{map[string]any{"toolName": "Bash(x)"}}); ok {
		t.Fatal("a tool name with parentheses was read")
	}
}

func TestProjectAllowRulesGainTheRuleOnce(t *testing.T) {
	testhome.Temp(t)
	d := &agentDaemon{repoRoot: t.TempDir()}
	if err := agentconfig.WriteSettings(agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{
		"project": {Path: "/checkout", ClaudeSandbox: &agentconfig.ClaudeSandbox{Deny: []string{"Bash(git push:*)"}}},
	}}); err != nil {
		t.Fatal(err)
	}
	added, err := d.addProjectAllowRules("project", []string{"Bash(npm test:*)", "Read"})
	if err != nil || !reflect.DeepEqual(added, []string{"Bash(npm test:*)", "Read"}) {
		t.Fatalf("added = %v %v", added, err)
	}
	if added, err := d.addProjectAllowRules("project", []string{"Read", " Bash(npm test:*) "}); err != nil || added != nil {
		t.Fatalf("a known rule was added again: %v %v", added, err)
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		t.Fatal(err)
	}
	project := settings.Project("project")
	if project.Path != "/checkout" || !reflect.DeepEqual(project.ClaudeSandbox.Allow, []string{"Bash(npm test:*)", "Read"}) || !reflect.DeepEqual(project.ClaudeSandbox.Deny, []string{"Bash(git push:*)"}) {
		t.Fatalf("project = %+v %+v", project, project.ClaudeSandbox)
	}
	path, err := d.projectClaudeSettings("project")
	if err != nil || path == "" {
		t.Fatalf("the next launch gets no settings: %q %v", path, err)
	}
	if raw, _ := os.ReadFile(path); !strings.Contains(string(raw), `"Bash(npm test:*)"`) {
		t.Fatalf("the generated settings lack the rule: %s", raw)
	}
}

// A project with no values of its own launches with the workstation ones it
// is covered by, and with nothing once the whitelist leaves it out (#730).
func TestALaunchAppliesTheWorkstationSandbox(t *testing.T) {
	testhome.Temp(t)
	d := &agentDaemon{repoRoot: t.TempDir()}
	settings := agentconfig.Settings{
		Defaults: agentconfig.Defaults{ClaudeSandbox: &agentconfig.ClaudeSandbox{Deny: []string{"Bash(git push:*)"}}},
		ProjectSettings: map[string]agentconfig.ProjectSettings{
			"project": {Path: "/checkout", ClaudeSandbox: &agentconfig.ClaudeSandbox{Allow: []string{"Read"}}},
			"other":   {Path: "/other"},
		},
	}
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"project", "other"} {
		path, err := d.projectClaudeSettings(id)
		if err != nil || path == "" {
			t.Fatalf("%s gets no settings: %q %v", id, path, err)
		}
		if raw, _ := os.ReadFile(path); !strings.Contains(string(raw), `"Bash(git push:*)"`) {
			t.Fatalf("%s lacks the workstation rule: %s", id, raw)
		}
	}
	settings.Defaults.ClaudeSandboxProjects = []string{"project"}
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	if path, err := d.projectClaudeSettings("other"); err != nil || path != "" {
		t.Fatalf("a project left out of the whitelist still gets settings: %q %v", path, err)
	}
	path, err := d.projectClaudeSettings("project")
	if raw, _ := os.ReadFile(path); err != nil || !strings.Contains(string(raw), `"Bash(git push:*)"`) || !strings.Contains(string(raw), `"Read"`) {
		t.Fatalf("a covered project lacks a level: %s %v", raw, err)
	}
}

func TestAProjectWithoutValuesGetsNoSettings(t *testing.T) {
	testhome.Temp(t)
	d := &agentDaemon{repoRoot: t.TempDir()}
	if path, err := d.projectClaudeSettings("project"); err != nil || path != "" {
		t.Fatalf("path = %q %v", path, err)
	}
	if path, err := d.launchClaudeSettings(agentconfig.Config{ProjectID: "project", AIProvider: "codex"}); err != nil || path != "" {
		t.Fatalf("codex = %q %v", path, err)
	}
}

// Only the built-in Claude lines take the settings; codex, agy and a
// configured template are left as they are.
func TestSettingsReachOnlyTheBuiltInClaudeLines(t *testing.T) {
	launch := agentCommandContext{ClaudeSettings: "/home/me/.config/sectile/claude/p.json"}
	want := "--settings='/home/me/.config/sectile/claude/p.json'"
	for _, mode := range []string{models.SkillModeAutonomous, models.SkillModeInteractive} {
		line, err := modeCommandLine("claude", "", "", "go", mode, launch)
		if err != nil || !strings.HasSuffix(line, want) {
			t.Fatalf("claude %s = %q %v", mode, line, err)
		}
		if bare, _ := modeCommandLine("claude", "", "", "go", mode); strings.Contains(bare, "--settings") || bare+" "+want != line {
			t.Fatalf("claude %s without values changed: %q", mode, bare)
		}
		for _, provider := range []string{"codex", "agy"} {
			if line, _ := modeCommandLine(provider, "", "", "go", mode, launch); strings.Contains(line, "--settings") {
				t.Fatalf("%s %s got the settings: %q", provider, mode, line)
			}
		}
	}
	if line, _ := modeCommandLine("claude", "claude {mode:-p|} '{prompt}'", "", "go", models.SkillModeAutonomous, launch); strings.Contains(line, "--settings") {
		t.Fatalf("a configured template was rewritten: %q", line)
	}
	if got := settingsArg("Claude", " /p.json "); got != "--settings='/p.json'" {
		t.Fatalf("settingsArg = %q", got)
	}
}

func TestConversationCommandCarriesTheSettings(t *testing.T) {
	plain := claudeConversationCommand(t.TempDir(), "", "", "", "", "", nil, nil)
	cmd := claudeConversationCommand(t.TempDir(), "", "", "", "", "/p q.json", []string{"/a"}, nil)
	if got := cmd.Args[len(plain.Args):]; len(got) != 2 || got[0] != "--add-dir=/a" || got[1] != "--settings=/p q.json" {
		t.Fatalf("arguments = %q", got)
	}
}

// A headless run names what Claude Code refused in its activity, in French,
// and where it can be allowed (#700).
func TestAHeadlessRunNamesWhatClaudeRefused(t *testing.T) {
	record := func(result string) string {
		var recorded strings.Builder
		readTracedOutput(newRunTrace(), strings.NewReader(result+"\n"), func(text string) { recorded.WriteString(text) })
		return recorded.String()
	}
	got := record(`{"type":"result","is_error":false,"result":"Done.","permission_denials":[{"tool_name":"Bash","tool_use_id":"t1","tool_input":{"command":"git push origin main"}},{"tool_name":"WebFetch","tool_use_id":"t2","tool_input":{"url":"https://blocked.example/x"}}]}`)
	want := "Done.\n" +
		"Refusé par Claude Code : Bash(git push origin main) · autorisez-le dans les réglages Claude du projet\n" +
		"Refusé par Claude Code : WebFetch(https://blocked.example/x) · autorisez-le dans les réglages Claude du projet\n"
	if got != want {
		t.Fatalf("activity = %q", got)
	}
	if got := record(`{"type":"result","is_error":false,"result":"Done.","permission_denials":[]}`); got != "Done.\n" {
		t.Fatalf("a run with no refusal changed its activity: %q", got)
	}
}

func TestHeadlessRefusalsAreBounded(t *testing.T) {
	denials := make([]string, 12)
	for i := range denials {
		denials[i] = `{"tool_name":"Bash","tool_input":{"command":"step ` + string(rune('a'+i)) + `"}}`
	}
	lines := permissionDenialLines(`{"type":"result","permission_denials":[` + strings.Join(denials, ",") + `]}`)
	if len(lines) != 11 || !strings.Contains(lines[9], "step j") || lines[10] != "… et 2 autres refus de Claude Code" {
		t.Fatalf("lines = %q", lines)
	}
	if lines := permissionDenialLines(`{"type":"result","permission_denials":[{"tool_name":"Read","tool_input":{}}]}`); len(lines) != 1 || !strings.HasPrefix(lines[0], "Refusé par Claude Code : Read ·") {
		t.Fatalf("a refusal without a target = %q", lines)
	}
}

// The desktop reads and saves a project's sandbox values; an empty entry is
// refused and saves nothing, an emptied value clears them (#700).
func TestDesktopProjectSandboxSettings(t *testing.T) {
	root := t.TempDir()
	testhome.Set(t, root)
	for _, args := range [][]string{{"init"}, {"remote", "add", "origin", "https://example.test/project.git"}} {
		if _, err := gitLocal(context.Background(), root, args...); err != nil {
			t.Fatal(err)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/agent/config") {
			_ = json.NewEncoder(w).Encode(agentconfig.Config{SchemaVersion: agentconfig.Version, ProjectID: "p", GitRemoteURL: "https://example.test/project.git"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	d := &agentDaemon{repoRoot: root, loopback: loopbackServer{desktopToken: "private"}, link: serverLink{serverURL: srv.URL, projectID: "p"}}
	doReq := func(method, path string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Authorization", "Bearer private")
		w := httptest.NewRecorder()
		d.desktopHandler(w, r)
		return w
	}
	read := func() map[string]any {
		t.Helper()
		w := doReq("GET", "/desktop/project?id=p", nil)
		var out map[string]any
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil {
			t.Fatalf("GET: %d %s", w.Code, w.Body.String())
		}
		return out
	}
	initial := read()
	if initial["platformSandbox"] != (runtime.GOOS != "windows") {
		t.Fatalf("platformSandbox = %v", initial["platformSandbox"])
	}
	if sandbox, _ := initial["claudeSandbox"].(map[string]any); sandbox["enabled"] != nil || len(sandbox["allow"].([]any)) != 0 {
		t.Fatalf("a project without values reads %v", initial["claudeSandbox"])
	}
	save := map[string]any{"projectId": "p", "path": root, "claudeSandbox": map[string]any{
		"enabled": true, "allowedDomains": []string{" registry.npmjs.org ", "registry.npmjs.org"}, "allowWrite": []string{"~/.cache/go-build"},
		"allow": []string{"Bash(make test:*)"}, "deny": []string{"Bash(git push:*)"},
	}}
	if w := doReq("POST", "/desktop/projects", save); w.Code != 204 {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	sandbox := read()["claudeSandbox"].(map[string]any)
	if sandbox["enabled"] != true || !reflect.DeepEqual(sandbox["allowedDomains"], []any{"registry.npmjs.org"}) || !reflect.DeepEqual(sandbox["deny"], []any{"Bash(git push:*)"}) {
		t.Fatalf("saved values read back as %v", sandbox)
	}
	if w := doReq("POST", "/desktop/projects", map[string]any{"projectId": "p", "path": root, "claudeSandbox": map[string]any{"allow": []string{"  "}}}); w.Code != 400 {
		t.Fatalf("an empty rule was accepted: %d", w.Code)
	}
	if sandbox := read()["claudeSandbox"].(map[string]any); !reflect.DeepEqual(sandbox["allow"], []any{"Bash(make test:*)"}) {
		t.Fatalf("a refused save changed the values: %v", sandbox)
	}
	if w := doReq("POST", "/desktop/projects", map[string]any{"projectId": "p", "path": root, "terminal": "ghostty"}); w.Code != 204 {
		t.Fatalf("save without sandbox: %d", w.Code)
	}
	if sandbox := read()["claudeSandbox"].(map[string]any); sandbox["enabled"] != true {
		t.Fatalf("a save without sandbox values dropped them: %v", sandbox)
	}
	if w := doReq("POST", "/desktop/projects", map[string]any{"projectId": "p", "path": root, "claudeSandbox": map[string]any{}}); w.Code != 204 {
		t.Fatalf("clear: %d", w.Code)
	}
	settings, _ := agentconfig.ReadSettings(root)
	if settings.Project("p").ClaudeSandbox != nil || settings.Project("p").Terminal != "ghostty" {
		t.Fatalf("clearing left %+v", settings.Project("p"))
	}
}

// A rule "Always allow" adds while the project dialog is open survives the
// dialog's save, which carries the values it read as its base (#700).
func TestDesktopSaveKeepsARuleAddedWhileTheDialogWasOpen(t *testing.T) {
	root := t.TempDir()
	testhome.Set(t, root)
	for _, args := range [][]string{{"init"}, {"remote", "add", "origin", "https://example.test/project.git"}} {
		if _, err := gitLocal(context.Background(), root, args...); err != nil {
			t.Fatal(err)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/agent/config") {
			_ = json.NewEncoder(w).Encode(agentconfig.Config{SchemaVersion: agentconfig.Version, ProjectID: "p", GitRemoteURL: "https://example.test/project.git"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	d := &agentDaemon{repoRoot: root, loopback: loopbackServer{desktopToken: "private"}, link: serverLink{serverURL: srv.URL, projectID: "p"}}
	save := func(body any) {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", "/desktop/projects", bytes.NewReader(raw))
		r.Header.Set("Authorization", "Bearer private")
		w := httptest.NewRecorder()
		d.desktopHandler(w, r)
		if w.Code != 204 {
			t.Fatalf("save: %d %s", w.Code, w.Body.String())
		}
	}
	opened := map[string]any{"allow": []string{"Read", "Bash(ls:*)"}, "deny": []string{}}
	save(map[string]any{"projectId": "p", "path": root, "claudeSandbox": opened})
	if _, err := d.addProjectAllowRules("p", []string{"Bash(npm test:*)"}); err != nil {
		t.Fatal(err)
	}
	// The owner removed Bash(ls:*) and added a deny rule in the open dialog.
	save(map[string]any{"projectId": "p", "path": root,
		"claudeSandbox":     map[string]any{"allow": []string{"Read"}, "deny": []string{"Bash(rm:*)"}},
		"claudeSandboxBase": opened})
	settings, _ := agentconfig.ReadSettings(root)
	got := settings.Project("p").ClaudeSandbox
	if got == nil || !reflect.DeepEqual(got.Allow, []string{"Read", "Bash(npm test:*)"}) || !reflect.DeepEqual(got.Deny, []string{"Bash(rm:*)"}) {
		t.Fatalf("the save lost a concurrent rule or kept a removed one: %+v", got)
	}
}

func TestAlwaysAllowDirectorySuggestionsPersistProjectAccess(t *testing.T) {
	testhome.Set(t, t.TempDir())
	d := &agentDaemon{}
	raw := json.RawMessage(`[{"type":"addDirectories","directories":["/shared"," /shared "],"destination":"localSettings"},{"type":"removeDirectories","directories":["/removed"]}]`)
	directories := approvedDirectories(raw)
	if err := d.addProjectDirectories("p", directories); err != nil {
		t.Fatal(err)
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		t.Fatal(err)
	}
	if got := settings.Project("p").ClaudeSandbox.AdditionalDirectories; !reflect.DeepEqual(got, []string{"/shared"}) {
		t.Fatalf("directories = %v", got)
	}
	if got := approvedDirectories(json.RawMessage(`{"type":"addDirectories"}`)); got != nil {
		t.Fatalf("malformed update persisted: %v", got)
	}
}
