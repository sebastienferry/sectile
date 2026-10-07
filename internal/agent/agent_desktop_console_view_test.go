package agent

import (
	"strings"
	"testing"

	"tasks/internal/agentconfig"
)

// The workstation console view (#711) is read and replaced through Desktop's
// endpoint, refuses a value it does not know, and is announced in the status.
func TestDesktopConsoleViewEndpoint(t *testing.T) {
	d, _ := disconnectFixture(t)
	view := func() string {
		t.Helper()
		rec := disconnectRequest(d, "GET", "/desktop/console-view", "")
		if rec.Code != 200 {
			t.Fatalf("GET = %d %s", rec.Code, rec.Body.String())
		}
		return strings.TrimSpace(rec.Body.String())
	}
	if got := view(); got != `{"view":"terminal"}` || d.workstationConsoleView() != agentconfig.ConsoleViewTerminal {
		t.Fatalf("an agent that never received a view must hold the terminal: %s", got)
	}
	if rec := disconnectRequest(d, "PUT", "/desktop/console-view", `{"view":"conversation"}`); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"view":"conversation"}` {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body.String())
	}
	if got := view(); got != `{"view":"conversation"}` || d.workstationConsoleView() != agentconfig.ConsoleViewConversation {
		t.Fatalf("view not kept: %s", got)
	}
	for _, body := range []string{`{"view":"tabs"}`, `{"view":""}`, `not json`} {
		if rec := disconnectRequest(d, "PUT", "/desktop/console-view", body); rec.Code != 400 {
			t.Errorf("%s accepted: %d", body, rec.Code)
		}
	}
	if got := view(); got != `{"view":"conversation"}` {
		t.Fatalf("a refused value replaced the view: %s", got)
	}
	if rec := disconnectRequest(d, "DELETE", "/desktop/console-view", ""); rec.Code != 405 {
		t.Fatalf("DELETE = %d", rec.Code)
	}
	if rec := disconnectRequest(d, "GET", "/desktop/status", ""); !strings.Contains(rec.Body.String(), `"`+consoleViewCapability+`"`) {
		t.Fatalf("status = %s", rec.Body.String())
	}

	// Saving the workstation form does not carry the view and must not drop it.
	if rec := disconnectRequest(d, "PUT", "/desktop/workstation", `{"editorCommand":"vim"}`); rec.Code != 204 {
		t.Fatalf("workstation PUT = %d %s", rec.Code, rec.Body.String())
	}
	if got := view(); got != `{"view":"conversation"}` {
		t.Fatalf("the workstation form erased the view: %s", got)
	}

	if rec := disconnectRequest(d, "PUT", "/desktop/console-view", `{"view":"terminal"}`); rec.Code != 200 || view() != `{"view":"terminal"}` {
		t.Fatalf("back to the terminal: %d %s", rec.Code, rec.Body.String())
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil || settings.Defaults.ConsoleView != "" {
		t.Fatalf("the terminal is stored as an absent key: %+v %v", settings.Defaults, err)
	}
}

func TestOpensConversation(t *testing.T) {
	claude, codex := agentconfig.Config{AIProvider: "claude"}, agentconfig.Config{AIProvider: "codex"}
	const terminal, conversation = agentconfig.ConsoleViewTerminal, agentconfig.ConsoleViewConversation
	for _, tc := range []struct {
		name       string
		autonomous bool
		action     string
		marked     bool
		view       string
		config     agentconfig.Config
		want       bool
	}{
		{"web launch, conversation view", false, "clarify", false, conversation, claude, true},
		{"web discussion, conversation view", false, "discuss", false, conversation, claude, true},
		{"web launch, terminal view", false, "clarify", false, terminal, claude, false},
		{"web launch, no view", false, "clarify", false, "", claude, false},
		{"Desktop mark, terminal view", false, "clarify", true, terminal, claude, true},
		{"Desktop mark, conversation view", false, "clarify", true, conversation, claude, true},
		{"autonomous, conversation view", true, "clarify", false, conversation, claude, false},
		{"autonomous, marked", true, "clarify", true, conversation, claude, false},
		{"open_terminal, conversation view", false, "open_terminal", false, conversation, claude, false},
		{"open_terminal, marked", false, "open_terminal", true, terminal, claude, false},
		{"codex, conversation view", false, "clarify", false, conversation, codex, true},
		{"codex, marked", false, "clarify", true, conversation, codex, true},
	} {
		if got := opensConversation(tc.autonomous, tc.action, tc.marked, tc.view, tc.config); got != tc.want {
			t.Errorf("%s: opensConversation = %v, want %v", tc.name, got, tc.want)
		}
	}
}
