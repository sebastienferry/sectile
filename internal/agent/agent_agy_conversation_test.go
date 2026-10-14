package agent

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"tasks/internal/testhome"
	"testing"
	"time"
)

func TestAgyConversationCommandFlagsAndModes(t *testing.T) {
	session := "11111111-1111-4111-8111-111111111111"
	cmd := agyConversationCommand(t.TempDir(), "gemini-3.8-pro", "high", "accept-edits", session, "", []string{"/repo/a", "/repo/b c"}, map[string]string{"SECTILE_PROJECT_ID": "p"})
	got := strings.Join(cmd.Args, " ")
	for _, want := range []string{
		"--input-format stream-json",
		"--output-format stream-json",
		"-p=",
		"--conversation=" + session,
		"--model=gemini-3.8-pro",
		"--effort=high",
		"--mode=accept-edits",
		"--add-dir=/repo/a",
		"--add-dir=/repo/b c",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	if strings.Contains(got, "dangerously-skip-permissions") {
		t.Fatalf("unexpected skip-permissions in accept-edits mode: %s", got)
	}

	autoCmd := agyConversationCommand(t.TempDir(), "", "", "auto", "", "", nil, nil)
	if !strings.Contains(strings.Join(autoCmd.Args, " "), "--dangerously-skip-permissions") {
		t.Fatalf("auto mode missing --dangerously-skip-permissions: %v", autoCmd.Args)
	}

	planCmd := agyConversationCommand(t.TempDir(), "", "", "plan", "", "", nil, nil)
	if !strings.Contains(strings.Join(planCmd.Args, " "), "--mode=plan") {
		t.Fatalf("plan mode missing --mode=plan: %v", planCmd.Args)
	}

	defaultCmd := agyConversationCommand(t.TempDir(), "", "", "default", "", "", nil, nil)
	defaultArgs := strings.Join(defaultCmd.Args, " ")
	if strings.Contains(defaultArgs, "--mode=") || strings.Contains(defaultArgs, "--dangerously-skip-permissions") {
		t.Fatalf("default mode should omit --mode and --dangerously-skip-permissions: %s", defaultArgs)
	}

	slashCmd := agyConversationCommand(t.TempDir(), "gemini-3.8-pro", "", "accept-edits", session, "/help", nil, nil)
	slashArgs := strings.Join(slashCmd.Args, " ")
	if strings.Contains(slashArgs, "--input-format") || !strings.Contains(slashArgs, "-p=/help") {
		t.Fatalf("local slash command must use -p=/help without --input-format: %s", slashArgs)
	}
}

func TestAgyConversationStreamParsingAndToolMapping(t *testing.T) {
	trace := newRunTrace()
	state := newAgyStreamState(false)
	var partials []string
	frames := []string{
		`{"event":"init","conversation_id":"22222222-2222-4222-8222-222222222222","init":{"cwd":"/tmp","permission_mode":"accept-edits"}}`,
		`{"event":"step_update","step_update":{"conversation_id":"22222222-2222-4222-8222-222222222222","step_index":0,"state":"DONE","step_type":"user_input"}}`,
		`{"event":"step_update","step_update":{"step_index":1,"state":"ACTIVE","step_type":"agent_response","thinking_delta":"Checking README"}}`,
		`{"event":"step_update","step_update":{"step_index":2,"state":"ACTIVE","step_type":"tool","tool_name":"view_file","tool_info":{"name":"view_file","parameters":{"AbsolutePath":"/tmp/README.md","StartLine":1,"EndLine":20}}}}`,
		`{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"tool","tool_name":"view_file","tool_info":{"name":"view_file","parameters":{"AbsolutePath":"/tmp/README.md"},"output":"20 lines, 400 bytes"}}}`,
		`{"event":"step_update","step_update":{"step_index":3,"state":"ACTIVE","step_type":"tool","tool_name":"replace_file_content","tool_info":{"name":"replace_file_content","parameters":{"TargetFile":"/tmp/README.md","TargetContent":"old","ReplacementContent":"new","AllowMultiple":false}}}}`,
		`{"event":"step_update","step_update":{"step_index":3,"state":"DONE","step_type":"tool","tool_name":"replace_file_content","tool_info":{"name":"replace_file_content","output":"Replaced 1 occurrence"}}}`,
		`{"event":"step_update","step_update":{"step_index":4,"state":"ACTIVE","step_type":"tool","tool_name":"run_command","tool_info":{"name":"run_command","parameters":{"CommandLine":"go test ./...","toolSummary":"Run tests"}}}}`,
		`{"event":"step_update","step_update":{"step_index":4,"state":"ERROR","step_type":"tool","tool_name":"run_command","tool_info":{"name":"run_command","error":{"type":"exit_error","message":"exit status 1"}}}}`,
		`{"event":"step_update","step_update":{"step_index":5,"state":"ACTIVE","step_type":"agent_response","text_delta":"Updated "}}`,
		`{"event":"step_update","step_update":{"step_index":5,"state":"DONE","step_type":"agent_response","text_delta":"README.","usage":{"input_tokens":1000,"output_tokens":50,"cache_read_tokens":500}}}`,
		`{"event":"result","result":{"conversation_id":"22222222-2222-4222-8222-222222222222","status":"SUCCESS","response":"Updated README.","denied_actions":[{"tool":"run_command","scope":"git push","rule":"headless"}]}}`,
	}
	for _, frame := range frames {
		state.handleLine(frame, trace, func(delta string, clear bool) {
			if !clear && delta != "" {
				partials = append(partials, delta)
			}
		})
	}
	if state.sessionID != "22222222-2222-4222-8222-222222222222" {
		t.Fatalf("sessionID = %q", state.sessionID)
	}
	if state.contextUsed != 1550 {
		t.Fatalf("contextUsed = %d, want 1550", state.contextUsed)
	}
	if strings.Join(partials, "") != "Updated README." {
		t.Fatalf("partials = %q", partials)
	}
	lines, _ := trace.snapshot()
	var events []conversationEvent
	for _, line := range lines {
		var ev conversationEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatal(err)
		}
		events = append(events, ev)
	}
	wantKinds := []string{"thinking", "tool", "tool_result", "tool", "tool_result", "tool", "tool_result", "assistant", "notice"}
	if len(events) != len(wantKinds) {
		t.Fatalf("events = %+v, want kinds %v", events, wantKinds)
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("events[%d].Kind = %q, want %q (%+v)", i, events[i].Kind, want, events[i])
		}
	}
	if events[1].Tool != "Read" || events[3].Tool != "Edit" || events[5].Tool != "Bash" {
		t.Fatalf("mapped tools = %q, %q, %q", events[1].Tool, events[3].Tool, events[5].Tool)
	}
	if !events[6].Error || !strings.Contains(events[6].Text, "exit status 1") {
		t.Fatalf("failed tool_result = %+v", events[6])
	}
	if !strings.Contains(events[8].Detail, "run_command (git push, headless)") {
		t.Fatalf("denied actions notice = %+v", events[8])
	}
}

func TestParseAgyMCPList(t *testing.T) {
	connected := "NAME     TYPE  STATUS   COMMAND/URL\nsectile  http  enabled  https://sectile.example.test/mcp\n"
	if got := parseAgyMCPList(connected); got.Status != "connected" {
		t.Fatalf("parseAgyMCPList(connected) = %+v", got)
	}
	disabled := "NAME     TYPE  STATUS    COMMAND/URL\nsectile  http  disabled  https://sectile.example.test/mcp\n"
	if got := parseAgyMCPList(disabled); got.Status != "failed" {
		t.Fatalf("parseAgyMCPList(disabled) = %+v", got)
	}
	missing := "NAME     TYPE  STATUS   COMMAND/URL\nother    http  enabled  https://other.example.test/mcp\n"
	if got := parseAgyMCPList(missing); got.Status != "missing" {
		t.Fatalf("parseAgyMCPList(missing) = %+v", got)
	}
}

func TestAgyConversationRunsAndResumesTurn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script; Windows launch flags have a separate test")
	}
	testhome.Temp(t)
	bin := t.TempDir()
	script := `#!/bin/sh
printf '%s\n' "$@" >> args.txt
case "$*" in
  *"-p=/help"*)
    printf '%s\n' '{"event":"command_result","command":{"name":"help","data":{"commands":[{"name":"help","description":"Show help"}]}}}'
    printf '%s\n' '{"event":"result","result":{"conversation_id":"","status":"SUCCESS","response":"/help\tShow help"}}'
    exit 0
    ;;
esac
read -r message
printf '%s' "$message" > prompt.txt
printf '%s\n' '{"event":"init","conversation_id":"11111111-1111-4111-8111-111111111111"}'
printf '%s\n' '{"event":"step_update","step_update":{"conversation_id":"11111111-1111-4111-8111-111111111111","step_index":1,"state":"DONE","step_type":"agent_response","text_delta":"Hello from Antigravity"}}'
printf '%s\n' '{"event":"result","result":{"conversation_id":"11111111-1111-4111-8111-111111111111","status":"SUCCESS","response":"Hello from Antigravity"}}'
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}
	d.probeCommandsFn = func(*exec.Cmd) []conversationSlash {
		return []conversationSlash{{Name: "clarify-issue", Description: "Clarify a ticket"}}
	}
	d.checkMCPFn = func(string, map[string]string) conversationMCP {
		return conversationMCP{Status: "connected"}
	}
	dir := t.TempDir()
	d.queue.runs = map[string]*controlledRun{
		"source": {desktop: desktopRun{ProjectID: "project", Provider: "agy", Model: "gemini-3.8-pro", Directory: dir, Branch: "feat/agy"}},
	}
	w := conversationRequest(d, "POST", "/desktop/conversation", `{"sourceRunId":"source"}`, "private")
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var entry desktopRun
	if err := json.Unmarshal(w.Body.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.stopConversations)
	if entry.Provider != "agy" || entry.Model != "gemini-3.8-pro" {
		t.Fatalf("created entry = %+v", entry)
	}
	run := d.queue.runs[entry.ID]

	sendAndWait := func(payload map[string]string) {
		t.Helper()
		body, _ := json.Marshal(payload)
		resp := conversationRequest(d, "POST", "/desktop/conversation?id="+entry.ID, string(body), "private")
		if resp.Code != http.StatusAccepted {
			t.Fatalf("send: %d %s", resp.Code, resp.Body.String())
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			d.queue.mu.Lock()
			busy := run.conversation.busy
			d.queue.mu.Unlock()
			if !busy {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("Antigravity turn did not finish")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	sendAndWait(map[string]string{"message": "first turn", "effort": "high", "mode": "accept-edits"})
	sendAndWait(map[string]string{"message": "second turn", "mode": "auto"})
	sendAndWait(map[string]string{"message": "/help"})

	rawArgs, err := os.ReadFile(filepath.Join(dir, "args.txt"))
	if err != nil {
		t.Fatal(err)
	}
	argsText := string(rawArgs)
	for _, want := range []string{
		"--mode=accept-edits",
		"--effort=high",
		"--conversation=11111111-1111-4111-8111-111111111111",
		"--dangerously-skip-permissions",
		"-p=/help",
	} {
		if !strings.Contains(argsText, want) {
			t.Fatalf("missing %q in args:\n%s", want, argsText)
		}
	}
	rawPrompt, err := os.ReadFile(filepath.Join(dir, "prompt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawPrompt), `"event":"user"`) || !strings.Contains(string(rawPrompt), `"second turn"`) {
		t.Fatalf("unexpected stdin frame: %s", string(rawPrompt))
	}
	lines, _ := run.trace.snapshot()
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Hello from Antigravity") || !strings.Contains(joined, "/help\\tShow help") {
		t.Fatalf("trace missing expected assistant or command output:\n%s", joined)
	}
}
