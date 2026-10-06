package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/testhome"
)

// The workstation default permission mode of Claude conversations is read and
// replaced through Desktop's endpoint, refuses anything the composer does not
// offer (bypassPermissions included), and is announced in the status.
func TestDesktopConversationModeEndpoint(t *testing.T) {
	d, _ := disconnectFixture(t)
	mode := func() string {
		t.Helper()
		rec := disconnectRequest(d, "GET", "/desktop/conversation-mode", "")
		if rec.Code != 200 {
			t.Fatalf("GET = %d %s", rec.Code, rec.Body.String())
		}
		return strings.TrimSpace(rec.Body.String())
	}
	if got := mode(); got != `{"mode":"acceptEdits"}` || d.workstationConversationMode() != "acceptEdits" {
		t.Fatalf("an agent that never received a mode must hold acceptEdits: %s", got)
	}
	for _, value := range []string{"default", "plan", "auto"} {
		if rec := disconnectRequest(d, "PUT", "/desktop/conversation-mode", `{"mode":"`+value+`"}`); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"mode":"`+value+`"}` {
			t.Fatalf("PUT %s = %d %s", value, rec.Code, rec.Body.String())
		}
		if got := mode(); got != `{"mode":"`+value+`"}` || d.workstationConversationMode() != value {
			t.Fatalf("mode %s not kept: %s", value, got)
		}
	}
	for _, body := range []string{`{"mode":"bypassPermissions"}`, `{"mode":"dontAsk"}`, `{"mode":""}`, `not json`} {
		if rec := disconnectRequest(d, "PUT", "/desktop/conversation-mode", body); rec.Code != 400 {
			t.Errorf("%s accepted: %d", body, rec.Code)
		}
	}
	if got := mode(); got != `{"mode":"auto"}` {
		t.Fatalf("a refused value replaced the mode: %s", got)
	}
	if rec := disconnectRequest(d, "DELETE", "/desktop/conversation-mode", ""); rec.Code != 405 {
		t.Fatalf("DELETE = %d", rec.Code)
	}
	if rec := disconnectRequest(d, "GET", "/desktop/status", ""); !strings.Contains(rec.Body.String(), `"`+conversationModeCapability+`"`) {
		t.Fatalf("status = %s", rec.Body.String())
	}

	// Saving the workstation form does not carry the mode and must not drop it.
	if rec := disconnectRequest(d, "PUT", "/desktop/workstation", `{"editorCommand":"vim"}`); rec.Code != 204 {
		t.Fatalf("workstation PUT = %d %s", rec.Code, rec.Body.String())
	}
	if got := mode(); got != `{"mode":"auto"}` {
		t.Fatalf("the workstation form erased the mode: %s", got)
	}

	if rec := disconnectRequest(d, "PUT", "/desktop/conversation-mode", `{"mode":"acceptEdits"}`); rec.Code != 200 || mode() != `{"mode":"acceptEdits"}` {
		t.Fatalf("back to acceptEdits: %d %s", rec.Code, rec.Body.String())
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil || settings.Defaults.ConversationMode != "" {
		t.Fatalf("acceptEdits is stored as an absent key: %+v %v", settings.Defaults, err)
	}
}

// A value written by hand that the composer does not offer, bypassPermissions
// first, reads as acceptEdits: the setting can never widen what Claude may do
// beyond the modes the owner can pick.
func TestWorkstationConversationModeFallsBackToAcceptEdits(t *testing.T) {
	d, _ := disconnectFixture(t)
	for stored, want := range map[string]string{"": "acceptEdits", "bypassPermissions": "acceptEdits", "dontAsk": "acceptEdits", "Auto": "acceptEdits", "plan": "plan", "default": "default"} {
		setConversationMode(t, d.localSettingsRoot(), stored)
		if got := d.workstationConversationMode(); got != want {
			t.Errorf("stored %q: workstationConversationMode = %q, want %q", stored, got, want)
		}
	}
}

func setConversationMode(t *testing.T, root, mode string) {
	t.Helper()
	if _, err := agentconfig.UpdateSettings(root, func(settings *agentconfig.Settings) error {
		settings.Defaults.ConversationMode = mode
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// fakeConversationClaude puts on PATH a claude that records its arguments in
// args.txt, in the directory it runs in, and ends its turn at once.
func fakeConversationClaude(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	bin := t.TempDir()
	script := `#!/bin/sh
read -r init
read -r message
printf '%s\n' "$@" > args.txt
printf '%s\n' '{"type":"result","is_error":false,"result":"ok","session_id":"11111111-1111-4111-8111-111111111111"}'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func conversationReportedMode(t *testing.T, d *agentDaemon, id string) string {
	t.Helper()
	w := conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private")
	var body struct {
		Mode string `json:"mode"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &body) != nil {
		t.Fatalf("GET conversation = %d %s", w.Code, w.Body.String())
	}
	return body.Mode
}

func turnPermissionMode(t *testing.T, directory string) string {
	t.Helper()
	args, err := os.ReadFile(filepath.Join(directory, "args.txt"))
	if err != nil {
		t.Fatal(err)
	}
	_, after, found := strings.Cut(string(args), "--permission-mode\n")
	if !found {
		t.Fatalf("no permission mode on the turn: %s", args)
	}
	mode, _, _ := strings.Cut(after, "\n")
	return mode
}

// A conversation started from a selected execution starts in the workstation
// default mode: Desktop's composer adopts the mode the agent reports, and a
// first message that names none runs in it. The composer's pick still
// replaces it from the next message.
func TestANewConversationStartsInTheWorkstationMode(t *testing.T) {
	testhome.Temp(t)
	fakeConversationClaude(t)
	setConversationMode(t, t.TempDir(), "auto")
	d, id := conversationFixture(t)
	run := d.queue.runs[id]
	if got := conversationReportedMode(t, d, id); got != "auto" {
		t.Fatalf("a new conversation reports %q, want the workstation mode auto", got)
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"first"}`, "private"); w.Code != 202 {
		t.Fatalf("send: %d %s", w.Code, w.Body.String())
	}
	waitConversationIdle(t, d, run)
	if got := turnPermissionMode(t, run.root); got != "auto" {
		t.Fatalf("the first turn ran in %q, want auto", got)
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"next","mode":"plan"}`, "private"); w.Code != 202 {
		t.Fatalf("send: %d %s", w.Code, w.Body.String())
	}
	waitConversationIdle(t, d, run)
	if got := turnPermissionMode(t, run.root); got != "plan" || conversationReportedMode(t, d, id) != "plan" {
		t.Fatalf("the composer's pick did not replace the default: %q", got)
	}

	// A changed default applies to the next conversation only.
	setConversationMode(t, t.TempDir(), "default")
	if got := conversationReportedMode(t, d, id); got != "plan" {
		t.Fatalf("an open conversation changed mode with the setting: %q", got)
	}
	w := conversationRequest(d, "POST", "/desktop/conversation", `{"sourceRunId":"source"}`, "private")
	var entry desktopRun
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &entry) != nil {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if got := conversationReportedMode(t, d, entry.ID); got != "default" {
		t.Fatalf("the next conversation reports %q, want default", got)
	}
}

// Without a setting, or with one the agent does not accept, a conversation
// starts in acceptEdits, as it always did.
func TestANewConversationWithoutAValidSettingStartsInAcceptEdits(t *testing.T) {
	for _, stored := range []string{"", "bypassPermissions"} {
		t.Run("stored "+stored, func(t *testing.T) {
			testhome.Temp(t)
			fakeConversationClaude(t)
			setConversationMode(t, t.TempDir(), stored)
			d, id := conversationFixture(t)
			run := d.queue.runs[id]
			if got := conversationReportedMode(t, d, id); got != "acceptEdits" {
				t.Fatalf("reported %q, want acceptEdits", got)
			}
			if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"first"}`, "private"); w.Code != 202 {
				t.Fatalf("send: %d %s", w.Code, w.Body.String())
			}
			waitConversationIdle(t, d, run)
			if got := turnPermissionMode(t, run.root); got != "acceptEdits" {
				t.Fatalf("the first turn ran in %q, want acceptEdits", got)
			}
		})
	}
}

// A skill launched in the conversation view sends its command as the first
// turn at once, before the owner could pick anything: that turn runs in the
// workstation default mode.
func TestASkillLaunchedAsAConversationRunsItsFirstTurnInTheWorkstationMode(t *testing.T) {
	testhome.Temp(t)
	fakeConversationClaude(t)
	setConversationMode(t, t.TempDir(), "auto")
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}
	d.probeCommandsFn = func(*exec.Cmd) []conversationSlash { return nil }
	d.checkMCPFn = func(string, map[string]string) conversationMCP { return conversationMCP{Status: "unknown"} }
	t.Cleanup(d.stopConversations)
	directory := t.TempDir()
	run, err := d.enqueueRun("task", agentconfig.Dispatch{RunID: "skill-run"}, "p", directory, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	d.startLaunchConversation(run, desktopRun{ID: "skill-run", TaskID: "task", ProjectID: "p", Skill: "clarify", Directory: directory}, "", "It runs the clarify skill.", "/clarify T-1", nil, nil)
	waitConversationIdle(t, d, run)
	if got := turnPermissionMode(t, directory); got != "auto" {
		t.Fatalf("the skill's first turn ran in %q, want auto", got)
	}
	if got := conversationReportedMode(t, d, "skill-run"); got != "auto" {
		t.Fatalf("the skill conversation reports %q, want auto", got)
	}
}

// A Claude chat opened from a project in the conversation view starts in the
// workstation default mode too.
func TestAClaudeChatStartsInTheWorkstationMode(t *testing.T) {
	d, config := disconnectFixture(t)
	fakeConversationClaude(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/api/v1/agent/config" {
			_ = json.NewEncoder(w).Encode(config)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	d.link.serverURL = server.URL
	// Only the turns run the fake Claude: no command list, no MCP check.
	d.probeCommandsFn = func(*exec.Cmd) []conversationSlash { return nil }
	d.checkMCPFn = func(string, map[string]string) conversationMCP { return conversationMCP{Status: "unknown"} }
	t.Cleanup(d.stopConversations)
	setConversationMode(t, d.localSettingsRoot(), "plan")
	rec := disconnectRequest(d, "POST", "/desktop/consoles", `{"projectId":"p","provider":"claude","view":"conversation"}`)
	var entry desktopRun
	if rec.Code != 202 || json.Unmarshal(rec.Body.Bytes(), &entry) != nil || !entry.Conversation {
		t.Fatalf("chat admission: %d %s", rec.Code, rec.Body.String())
	}
	if got := conversationReportedMode(t, d, entry.ID); got != "plan" {
		t.Fatalf("the chat reports %q, want plan", got)
	}
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+entry.ID, `{"message":"hello"}`, "private"); w.Code != 202 {
		t.Fatalf("send: %d %s", w.Code, w.Body.String())
	}
	d.queue.mu.Lock()
	run := d.queue.runs[entry.ID]
	d.queue.mu.Unlock()
	waitConversationIdle(t, d, run)
	if got := turnPermissionMode(t, d.repoRoot); got != "plan" {
		t.Fatalf("the chat's first turn ran in %q, want plan", got)
	}
}
