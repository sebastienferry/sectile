package agentconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runHook executes the managed hook script the way Claude Code does: payload on
// stdin, environment supplied, output read back. It returns stdout, which must
// always be empty, and the exit code, which must always be 0.
func runHook(t *testing.T, payload string, env map[string]string, home string) (string, int) {
	t.Helper()
	raw, err := hookScripts.ReadFile(claudeHookSource)
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), claudeHookFile)
	if err := os.WriteFile(script, raw, 0700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/bin/sh", script)
	command.Stdin = strings.NewReader(payload)
	// A clean environment, not the test process's: these tests would otherwise
	// inherit the SECTILE_* variables of whatever session runs them, and a hook
	// reads exactly those to decide which report it makes.
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home}
	for key, value := range env {
		command.Env = append(command.Env, key+"="+value)
	}
	var stdout strings.Builder
	command.Stdout = &stdout
	_ = command.Run()
	return stdout.String(), command.ProcessState.ExitCode()
}

// runEnv is the environment of a session Sectile launched, pointed at a server.
func runEnv(loopback string) map[string]string {
	return map[string]string{"SECTILE_RUN_ID": "run-1", "SECTILE_LOOPBACK_URL": loopback, "SECTILE_AGENT_TOKEN": "key"}
}

// pairWorkstation publishes the connection file a session without a run reads.
func pairWorkstation(t *testing.T, server string) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".taskflow"), 0700); err != nil {
		t.Fatal(err)
	}
	connection := `{"url":"` + server + `","token":"companion-token"}`
	if err := os.WriteFile(filepath.Join(home, ".taskflow/agent-connection.json"), []byte(connection), 0600); err != nil {
		t.Fatal(err)
	}
	return home
}

// oneReport returns the single report a hook made, and fails when it made none
// or more than one.
func oneReport(t *testing.T, received chan string) string {
	t.Helper()
	var body string
	select {
	case body = <-received:
	default:
		t.Fatal("no report reached the agent")
	}
	select {
	case extra := <-received:
		t.Fatalf("a second report was made: %s", extra)
	default:
	}
	return body
}

func noReport(t *testing.T, received chan string) {
	t.Helper()
	select {
	case body := <-received:
		t.Fatalf("a report was made where none was due: %s", body)
	default:
	}
}

// Every path through the hook is checked for the same two things, because they
// are what separates a report from an interrupted session: the agent reads the
// exit code, and it reads standard output.
func TestHookAlwaysSucceedsAndStaysSilent(t *testing.T) {
	// Port 1 refuses immediately, which is the stopped-agent case.
	dead := runEnv("http://127.0.0.1:1")
	for _, test := range []struct {
		name    string
		payload string
		env     map[string]string
	}{
		{name: "unparseable payload", payload: "this is not JSON at all"},
		{name: "empty payload", payload: ""},
		{name: "no connection and no run", payload: `{"hook_event_name":"Notification","cwd":"/tmp/work"}`},
		{name: "dead loopback on a wait", payload: `{"hook_event_name":"Notification","cwd":"/tmp/work"}`, env: dead},
		{name: "dead loopback on a resume", payload: `{"hook_event_name":"PreToolUse","cwd":"/tmp/work"}`, env: dead},
		{name: "dead loopback on a stop", payload: `{"hook_event_name":"Stop"}`, env: dead},
		{name: "payload with no event", payload: `{"session_id":"abc","cwd":"/tmp/work"}`, env: dead},
		{name: "event the hook does not answer", payload: `{"hook_event_name":"SessionStart"}`, env: dead},
	} {
		t.Run(test.name, func(t *testing.T) {
			stdout, code := runHook(t, test.payload, test.env, t.TempDir())
			if code != 0 {
				t.Fatalf("exit code %d would be read back by the agent", code)
			}
			if stdout != "" {
				t.Fatalf("the hook wrote %q on stdout, which the agent reads back", stdout)
			}
		})
	}
}

// The state a launched session reports is decided by the event alone. A wait
// opens on a prompt and on the end of a turn — the agent then awaits the next
// prompt — and closes on anything that only happens while the agent works.
func TestHookReportsTheRunStateForEachEvent(t *testing.T) {
	for _, test := range []struct {
		event   string
		waiting string
	}{
		{event: "Notification", waiting: "true"},
		{event: "Stop", waiting: "true"},
		{event: "UserPromptSubmit", waiting: "false"},
		{event: "PreToolUse", waiting: "false"},
		{event: "PostToolUse", waiting: "false"},
	} {
		t.Run(test.event, func(t *testing.T) {
			received := make(chan string, 4)
			server := startJSONRecorder(t, received)
			payload := `{"hook_event_name":"` + test.event + `","cwd":"/tmp/w","tool_name":"Bash","tool_input":{"command":"ls"}}`
			if _, code := runHook(t, payload, runEnv(server), t.TempDir()); code != 0 {
				t.Fatalf("exit %d", code)
			}
			if body := oneReport(t, received); !strings.Contains(body, `"waiting":`+test.waiting) {
				t.Fatalf("%s reported %s, expected waiting=%s", test.event, body, test.waiting)
			}
		})
	}
}

// Claude Code notifies for more than prompts. Only a prompt means the session
// waits; a payload naming no type comes from an older Claude Code and is read
// as a prompt, which is what every payload was before the type existed.
func TestHookOnlyTreatsAPromptAsAWait(t *testing.T) {
	for _, test := range []struct {
		kind    string
		waiting bool
	}{
		{kind: "permission_prompt", waiting: true},
		{kind: "idle_prompt", waiting: true},
		{kind: "elicitation_dialog", waiting: true},
		{kind: "agent_needs_input", waiting: true},
		{kind: "", waiting: true},
		{kind: "auth_success", waiting: false},
		{kind: "agent_completed", waiting: false},
		{kind: "quota_auto_resume_fired", waiting: false},
	} {
		name := test.kind
		if name == "" {
			name = "no type"
		}
		t.Run(name, func(t *testing.T) {
			received := make(chan string, 4)
			server := startJSONRecorder(t, received)
			payload := `{"hook_event_name":"Notification","cwd":"/tmp/w","message":"hello"`
			if test.kind != "" {
				payload += `,"notification_type":"` + test.kind + `"`
			}
			payload += "}"
			stdout, code := runHook(t, payload, runEnv(server), t.TempDir())
			if code != 0 || stdout != "" {
				t.Fatalf("exit %d, stdout %q", code, stdout)
			}
			if !test.waiting {
				noReport(t, received)
				return
			}
			if body := oneReport(t, received); !strings.Contains(body, `"waiting":true`) {
				t.Fatalf("a prompt did not open a wait: %s", body)
			}
		})
	}
}

// A session Sectile did not launch reports itself through the connection file
// the agent publishes for its companions, naming itself by its directory. Only
// the two events that deserve a banner are announced: resuming is not one.
func TestHookAnnouncesAForeignSessionByItsDirectory(t *testing.T) {
	for _, test := range []struct {
		event string
		state string
	}{
		{event: "Notification", state: "waiting"},
		{event: "Stop", state: "completed"},
		{event: "UserPromptSubmit"},
		{event: "PreToolUse"},
		{event: "PostToolUse"},
	} {
		t.Run(test.event, func(t *testing.T) {
			received := make(chan string, 4)
			home := pairWorkstation(t, startJSONRecorder(t, received))
			stdout, code := runHook(t, `{"hook_event_name":"`+test.event+`","cwd":"/Users/x/worktrees/#174"}`, nil, home)
			if code != 0 || stdout != "" {
				t.Fatalf("exit %d, stdout %q", code, stdout)
			}
			if test.state == "" {
				noReport(t, received)
				return
			}
			body := oneReport(t, received)
			if !strings.Contains(body, `"session":"#174"`) {
				t.Fatalf("the session is not named by its directory: %s", body)
			}
			if !strings.Contains(body, `"state":"`+test.state+`"`) {
				t.Fatalf("the announced state is wrong: %s", body)
			}
		})
	}
}

// A payload with no working directory still announces the session, under a
// fallback name: the absence of a name is no reason to lose the banner.
func TestHookFallsBackToAGenericSessionName(t *testing.T) {
	received := make(chan string, 4)
	home := pairWorkstation(t, startJSONRecorder(t, received))
	if _, code := runHook(t, `{"hook_event_name":"Notification"}`, nil, home); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if body := oneReport(t, received); !strings.Contains(body, `"session":"Claude Code"`) {
		t.Fatalf("no fallback name: %s", body)
	}
}

// A session Sectile did launch reports against its run, and must not also file
// a session-level alert: that would announce the same session twice.
func TestHookWithARunReportsOnlyTheRun(t *testing.T) {
	received := make(chan string, 4)
	server := startJSONRecorder(t, received)
	home := pairWorkstation(t, server)
	if _, code := runHook(t, `{"hook_event_name":"Notification","cwd":"/tmp/w"}`, runEnv(server), home); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if body := oneReport(t, received); !strings.Contains(body, `"waiting":true`) {
		t.Fatalf("the run report is wrong: %s", body)
	}
}
