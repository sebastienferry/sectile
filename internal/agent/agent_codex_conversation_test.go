package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// This subprocess speaks the app-server protocol over real pipes, without
// authentication, network access or model inference.
func TestCodexAppServerHelper(t *testing.T) {
	if os.Getenv("SECTILE_CODEX_HELPER") != "1" {
		return
	}
	log, _ := os.OpenFile(os.Getenv("SECTILE_CODEX_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	defer log.Close()
	send := func(value any) { data, _ := json.Marshal(value); fmt.Fprintln(os.Stdout, string(data)) }
	notify := func(method string, params any) { send(map[string]any{"method": method, "params": params}) }
	reply := func(id json.RawMessage, result any) { send(map[string]any{"id": id, "result": result}) }
	thread, turn, number := "thread-test", "", 0
	finish := func(status string) {
		notify("turn/completed", map[string]any{"threadId": thread, "turn": map[string]string{"id": turn, "status": status}})
		turn = ""
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(log, line)
		var frame codexFrame
		if json.Unmarshal([]byte(line), &frame) != nil {
			os.Exit(2)
		}
		var params map[string]any
		_ = json.Unmarshal(frame.Params, &params)
		switch frame.Method {
		case "initialize":
			reply(frame.ID, map[string]string{"userAgent": "codex-test"})
		case "initialized":
		case "config/read":
			reply(frame.ID, map[string]any{"config": map[string]any{"developer_instructions": "native instructions"}})
		case "thread/start", "thread/resume":
			reply(frame.ID, map[string]any{"thread": map[string]string{"id": thread}, "model": "codex-test", "reasoningEffort": "medium"})
		case "model/list":
			reply(frame.ID, map[string]any{"data": []any{map[string]any{"model": "codex-test", "supportedReasoningEfforts": []any{map[string]string{"reasoningEffort": "low"}, map[string]string{"reasoningEffort": "high"}}}}})
		case "skills/list":
			reply(frame.ID, map[string]any{"data": []any{map[string]any{"skills": []any{map[string]any{"name": "code-issue", "path": "/skills/code-issue/SKILL.md", "enabled": true, "description": "Implement a ticket"}}}}})
		case "mcpServerStatus/list":
			reply(frame.ID, map[string]any{"data": []any{map[string]any{"name": "sectile", "runtimeStatus": "connected", "tools": map[string]any{"get_task": map[string]any{}}}}})
		case "turn/start":
			number++
			turn = fmt.Sprintf("turn-%d", number)
			input := params["input"].([]any)
			message := input[0].(map[string]any)["text"].(string)
			if strings.Contains(message, "crash") {
				os.Exit(3)
			}
			notify("turn/started", map[string]any{"threadId": thread, "turn": map[string]string{"id": turn, "status": "inProgress"}})
			reply(frame.ID, map[string]any{"turn": map[string]string{"id": turn, "status": "inProgress"}})
			switch {
			case strings.Contains(message, "approval"):
				notify("item/started", map[string]any{"threadId": thread, "turnId": turn, "item": map[string]any{"id": "command-1", "type": "commandExecution", "command": "git status", "cwd": "/repo", "status": "inProgress"}})
				send(map[string]any{"id": 7, "method": "item/commandExecution/requestApproval", "params": map[string]any{"threadId": thread, "turnId": turn, "itemId": "command-1", "command": "git status", "availableDecisions": []string{"accept", "acceptForSession", "decline"}}})
			case strings.Contains(message, "companion-mcp"):
				send(map[string]any{"id": "mcp-call-1", "method": "mcpServer/elicitation/request", "params": map[string]any{"serverName": "sectile_companion", "mode": "form", "message": "Allow this call?", "requestedSchema": map[string]any{"type": "object", "properties": map[string]any{}}}})
			case strings.Contains(message, "question"):
				send(map[string]any{"id": "question-1", "method": "item/tool/requestUserInput", "params": map[string]any{"threadId": thread, "turnId": turn, "itemId": "question-tool", "questions": []any{map[string]any{"id": "q_1", "header": "Scope", "question": "Which scope?", "options": []any{map[string]string{"label": "Small", "description": "One file"}}}}}})
			case message == "hold", message == "race":
			default:
				notify("item/agentMessage/delta", map[string]string{"threadId": thread, "turnId": turn, "itemId": "answer", "delta": "Hello"})
				notify("item/completed", map[string]any{"threadId": thread, "turnId": turn, "item": map[string]string{"id": "answer", "type": "agentMessage", "text": "Hello"}})
				notify("thread/tokenUsage/updated", map[string]any{"threadId": thread, "tokenUsage": map[string]any{"last": map[string]int{"inputTokens": 20, "outputTokens": 5}, "modelContextWindow": 200000}})
				finish("completed")
			}
		case "turn/steer":
			input := params["input"].([]any)
			message := input[0].(map[string]any)["text"].(string)
			if message == "race-message" {
				finish("completed")
				send(map[string]any{"id": frame.ID, "error": map[string]any{"code": -32000, "message": "No active turn"}})
			} else {
				reply(frame.ID, map[string]string{"turnId": turn})
				finish("completed")
			}
		case "turn/interrupt":
			reply(frame.ID, map[string]any{})
			finish("interrupted")
		default:
			if frame.Method != "" {
				os.Exit(4)
			}
			if string(frame.ID) == "7" {
				notify("item/completed", map[string]any{"threadId": thread, "turnId": turn, "item": map[string]any{"id": "command-1", "type": "commandExecution", "command": "git status", "status": "completed", "aggregatedOutput": "clean", "exitCode": 0}})
				finish("completed")
			} else if string(frame.ID) == `"question-1"` {
				finish("completed")
			} else if string(frame.ID) == `"mcp-call-1"` {
				notify("item/agentMessage/delta", map[string]string{"threadId": thread, "turnId": turn, "itemId": "answer", "delta": "MCP allowed"})
				notify("item/completed", map[string]any{"threadId": thread, "turnId": turn, "item": map[string]string{"id": "answer", "type": "agentMessage", "text": "MCP allowed"}})
				finish("completed")
			}
		}
	}
	os.Exit(0)
}

func codexConversationFixture(t *testing.T, profiles ...*profileSnapshot) (*agentDaemon, string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the helper launcher is a POSIX script; Windows command flags have separate tests")
	}
	root := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SECTILE_CODEX_HELPER", "1")
	t.Setenv("SECTILE_CODEX_BINARY", binary)
	log := filepath.Join(root, "protocol.log")
	t.Setenv("SECTILE_CODEX_LOG", log)
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.WriteFile(filepath.Join(root, "codex"), []byte("#!/bin/sh\nexec \"$SECTILE_CODEX_BINARY\" -test.run '^TestCodexAppServerHelper$'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}
	d.queue.runs = map[string]*controlledRun{"source": {desktop: desktopRun{Provider: "codex", Directory: root, Model: "codex-test", ProjectID: "p"}}}
	w := conversationRequest(d, "POST", "/desktop/conversation", `{"sourceRunId":"source"}`, "private")
	var entry desktopRun
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &entry) != nil {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if entry.Provider != "codex" {
		t.Fatalf("provider lost: %+v", entry)
	}
	t.Cleanup(d.stopConversations)
	if len(profiles) > 0 {
		applyProfile(d.queue.runs[entry.ID], profiles[0])
	}
	// Opening the view initializes the CLI and discovers skills/models without inference.
	conversationRequest(d, "GET", "/desktop/conversation?id="+entry.ID, "", "private")
	waitCodex(t, d, entry.ID, func(c *providerConversation) bool {
		return c.input != nil && len(c.models) > 0 && len(c.commands) > 0 && c.sectileMCP != nil
	})
	return d, entry.ID, log
}

func waitCodex(t *testing.T, d *agentDaemon, id string, ready func(*providerConversation) bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		d.queue.mu.Lock()
		ok := ready(d.queue.runs[id].conversation)
		d.queue.mu.Unlock()
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	w := conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private")
	t.Fatalf("Codex state did not settle: %s", w.Body.String())
}

func codexMessage(t *testing.T, d *agentDaemon, id, body string) {
	t.Helper()
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, body, "private"); w.Code != 202 {
		t.Fatalf("message: %d %s", w.Code, w.Body.String())
	}
}

func codexLog(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCodexConversationKeepsOneProcessAndSendsNativeSkills(t *testing.T) {
	d, id, log := codexConversationFixture(t)
	codexMessage(t, d, id, `{"message":"$code-issue #42","mode":"plan","effort":"high"}`)
	waitCodex(t, d, id, func(c *providerConversation) bool { return !c.busy && c.contextWindow == 200000 })
	codexMessage(t, d, id, `{"message":"next","mode":"workspace-write"}`)
	waitCodex(t, d, id, func(c *providerConversation) bool { return !c.busy })
	text := codexLog(t, log)
	if strings.Count(text, `"method":"initialize"`) != 1 || strings.Count(text, `"method":"turn/start"`) != 2 {
		t.Fatalf("process was not reused: %s", text)
	}
	for _, part := range []string{`"type":"skill"`, `"path":"/skills/code-issue/SKILL.md"`, `"mode":"plan"`, `"type":"readOnly"`, `"type":"workspaceWrite"`} {
		if !strings.Contains(text, part) {
			t.Errorf("missing %s: %s", part, text)
		}
	}
	w := conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private")
	if !strings.Contains(w.Body.String(), `"used":25`) || !strings.Contains(w.Body.String(), `"provider":"codex"`) {
		t.Fatal(w.Body.String())
	}
	if w := conversationRequest(d, "POST", "/desktop/stop?id="+id, "", "private"); w.Code != 204 {
		t.Fatalf("stop: %d %s", w.Code, w.Body.String())
	}
	d.queue.mu.Lock()
	state := d.queue.runs[id].conversation.codex
	d.queue.mu.Unlock()
	if state != nil {
		t.Fatal("stop acknowledged before the Codex process exited")
	}
}

func TestCodexConversationApprovalPreservesNumericRequestID(t *testing.T) {
	d, id, log := codexConversationFixture(t)
	codexMessage(t, d, id, `{"message":"approval"}`)
	waitCodex(t, d, id, func(c *providerConversation) bool { return len(c.approvals) == 1 })
	d.queue.mu.Lock()
	waiting := !d.queue.runs[id].desktop.WaitingSince.IsZero()
	d.queue.mu.Unlock()
	if !waiting {
		t.Fatal("approval did not mark the run waiting")
	}
	w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"approval":{"id":"codex-7","decision":"always"}}`, "private")
	if w.Code != 200 {
		t.Fatalf("approval: %d %s", w.Code, w.Body.String())
	}
	waitCodex(t, d, id, func(c *providerConversation) bool { return !c.busy && len(c.approvals) == 0 })
	if !strings.Contains(codexLog(t, log), `{"id":7,"result":{"decision":"acceptForSession"}}`) {
		t.Fatal("native session-scoped approval was not sent")
	}
}

func TestCodexConversationQuestionUsesNativeQuestionIDs(t *testing.T) {
	d, id, log := codexConversationFixture(t)
	codexMessage(t, d, id, `{"message":"question"}`)
	waitCodex(t, d, id, func(c *providerConversation) bool { return len(c.approvals) == 1 })
	w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"approval":{"id":"codex-\"question-1\"","decision":"answer","answers":{"Which scope?":"Small"}}}`, "private")
	if w.Code != 200 {
		t.Fatalf("question: %d %s", w.Code, w.Body.String())
	}
	waitCodex(t, d, id, func(c *providerConversation) bool { return !c.busy })
	if !strings.Contains(codexLog(t, log), `"answers":{"q_1":{"answers":["Small"]}}`) {
		t.Fatal("answers were not keyed by native question id")
	}
}

func TestCodexConversationSteersAndQueuesAMessageAtTurnEnd(t *testing.T) {
	for _, message := range []string{"steered", "race-message"} {
		t.Run(message, func(t *testing.T) {
			d, id, log := codexConversationFixture(t)
			codexMessage(t, d, id, `{"message":"hold"}`)
			waitCodex(t, d, id, func(c *providerConversation) bool { return c.codex.turnID != "" })
			body, _ := json.Marshal(map[string]string{"message": message})
			codexMessage(t, d, id, string(body))
			waitCodex(t, d, id, func(c *providerConversation) bool { return !c.busy && len(c.next) == 0 })
			text := codexLog(t, log)
			if !strings.Contains(text, `"method":"turn/steer"`) {
				t.Fatal("message did not steer the active turn")
			}
			want := 1
			if message == "race-message" {
				want = 2
			}
			if strings.Count(text, `"method":"turn/start"`) != want {
				t.Fatalf("turn boundary duplicated or lost the message: %s", text)
			}
		})
	}
}

func TestCodexConversationInterruptKeepsTheSession(t *testing.T) {
	d, id, log := codexConversationFixture(t)
	codexMessage(t, d, id, `{"message":"hold"}`)
	waitCodex(t, d, id, func(c *providerConversation) bool { return c.codex.turnID != "" })
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"interrupt":true}`, "private"); w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	waitCodex(t, d, id, func(c *providerConversation) bool { return !c.busy && !c.interrupted })
	codexMessage(t, d, id, `{"message":"continue"}`)
	waitCodex(t, d, id, func(c *providerConversation) bool { return !c.busy })
	text := codexLog(t, log)
	if strings.Count(text, `"method":"thread/start"`) != 1 || !strings.Contains(text, `"method":"thread/resume"`) {
		t.Fatalf("interrupt replaced the session: %s", text)
	}
}

func TestCodexConversationResumesAfterAProcessCrash(t *testing.T) {
	d, id, log := codexConversationFixture(t)
	codexMessage(t, d, id, `{"message":"crash"}`)
	waitCodex(t, d, id, func(c *providerConversation) bool { return c.codex == nil && !c.busy })
	codexMessage(t, d, id, `{"message":"resume"}`)
	waitCodex(t, d, id, func(c *providerConversation) bool { return c.codex != nil && !c.busy })
	if !strings.Contains(codexLog(t, log), `"method":"thread/resume"`) {
		t.Fatal("the next message did not resume the saved thread")
	}
}

func TestCodexEventsBoundToolResultsAndIgnoreEncryptedReasoning(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"id": "c", "type": "commandExecution", "aggregatedOutput": strings.Repeat("x", 20000), "status": "failed"})
	events := codexItemEvents(raw, true)
	if len(events) != 1 || !events[0].Truncated || !events[0].Error {
		t.Fatalf("tool failure or bound was lost: %+v", events)
	}
	if events := codexItemEvents(json.RawMessage(`{"type":"reasoning","encrypted_content":"private"}`), true); len(events) != 0 {
		t.Fatal("encrypted reasoning was displayed")
	}
}

func TestCodexConversationAutoApprovesCompanionMCP(t *testing.T) {
	d, id, log := codexConversationFixture(t)
	codexMessage(t, d, id, `{"message":"companion-mcp"}`)
	waitCodex(t, d, id, func(c *providerConversation) bool { return !c.busy && len(c.approvals) == 0 })
	if !strings.Contains(codexLog(t, log), `{"id":"mcp-call-1","result":{"action":"accept","content":{}}}`) {
		t.Fatalf("companion MCP was not automatically approved: %s", codexLog(t, log))
	}
}

func TestCodexConversationEnablesDefaultModeRequestUserInput(t *testing.T) {
	cmd := codexConversationCommand(t.TempDir(), nil)
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "-c features.default_mode_request_user_input=true") {
		t.Fatalf("codexConversationCommand did not enable default_mode_request_user_input: %s", args)
	}
}

func TestWorktreeGitWritableRootsIncludesWorktreeAndCommonSubdirs(t *testing.T) {
	root := t.TempDir()
	mainRepo := filepath.Join(root, "main")
	worktree := filepath.Join(root, "wt")
	if err := os.MkdirAll(mainRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitTest(t, mainRepo, "init")
	gitTest(t, mainRepo, "config", "user.email", "test@example.com")
	gitTest(t, mainRepo, "config", "user.name", "Test")
	gitTest(t, mainRepo, "commit", "--allow-empty", "-m", "init")
	gitTest(t, mainRepo, "worktree", "add", "-b", "feat/wt", worktree)

	if got := worktreeGitWritableRoots(mainRepo); len(got) != 0 {
		t.Fatalf("main repo should not add extra git roots, got %v", got)
	}
	roots := worktreeGitWritableRoots(worktree)
	joined := strings.Join(roots, "\n")
	if !strings.Contains(joined, filepath.Join(".git", "worktrees")) || !strings.Contains(joined, filepath.Join(".git", "objects")) || !strings.Contains(joined, filepath.Join(".git", "refs")) {
		t.Fatalf("worktreeGitWritableRoots missing expected subdirs: %v", roots)
	}
	for _, r := range roots {
		if strings.HasSuffix(r, string(filepath.Separator)+".git") || strings.Contains(r, "hooks") || strings.Contains(r, "config") {
			t.Fatalf("worktreeGitWritableRoots must not grant write access to %q", r)
		}
	}
}

func TestCodexConversationTurnGrantsWorktreeGitMetadataInSandboxPolicy(t *testing.T) {
	d, id, log := codexConversationFixture(t)
	root := t.TempDir()
	mainRepo := filepath.Join(root, "repo")
	worktree := filepath.Join(root, "worktrees", "issue-904")
	if err := os.MkdirAll(mainRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitTest(t, mainRepo, "init")
	gitTest(t, mainRepo, "config", "user.email", "test@example.com")
	gitTest(t, mainRepo, "config", "user.name", "Test")
	gitTest(t, mainRepo, "commit", "--allow-empty", "-m", "init")
	gitTest(t, mainRepo, "worktree", "add", "-b", "issue-904", worktree)

	d.queue.mu.Lock()
	d.queue.runs[id].desktop.Directory = worktree
	d.queue.mu.Unlock()

	codexMessage(t, d, id, `{"message":"commit","mode":"workspace-write"}`)
	waitCodex(t, d, id, func(c *providerConversation) bool { return !c.busy })
	text := codexLog(t, log)
	if !strings.Contains(text, `"type":"workspaceWrite"`) {
		t.Fatalf("expected workspaceWrite sandbox policy: %s", text)
	}
	for _, want := range []string{
		filepath.Join(".git", "worktrees", "issue-904"),
		filepath.Join(".git", "objects"),
		filepath.Join(".git", "refs"),
		filepath.Join(".git", "logs"),
	} {
		if !strings.Contains(text, want) {
			t.Errorf("turn/start sandboxPolicy missing %s: %s", want, text)
		}
	}
}
