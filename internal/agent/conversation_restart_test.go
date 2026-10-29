package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestConversationRestartPreservesStateWithoutReplaying(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			d, id := conversationFixture(t)
			d.store = testRunStore(t)
			run := d.queue.runs[id]
			run.desktop.Provider, run.desktop.Model = provider, "test-model"
			run.conversation.session, run.conversation.mode, run.conversation.effort = "saved-session", "plan", "high"
			run.conversation.busy = true
			run.conversation.next = []string{"must never replay"}
			run.conversation.partial = "unfinished answer"
			conversationWrite(run.trace, "assistant", "Saved context", "")
			before, _ := run.trace.snapshot()
			d.persistRuns()
			// A crash or orderly shutdown must both retain identity. Shutdown cancels
			// processes, but is not an explicit request to end a conversation.
			d.stopConversations()
			d.persistRuns()
			fresh := &agentDaemon{store: openRunStore(d.store.dir), loopback: loopbackServer{desktopToken: "private"}}
			fresh.restoreRuns()
			t.Cleanup(fresh.stopConversations)
			got := fresh.queue.runs[id]
			c := got.conversation
			if c == nil || got.restored || got.canceled || c.busy || c.input != nil || c.codex != nil || len(c.next) > 0 || c.partial != "" {
				t.Fatal("restart did not restore an idle writable conversation")
			}
			if c.session != "saved-session" || c.mode != "plan" || c.effort != "high" || got.desktop.Model != "test-model" {
				t.Fatal("provider identity or turn settings were lost")
			}
			after, _ := got.trace.snapshot()
			if len(after) != len(before)+1 || !reflect.DeepEqual(before, after[:len(before)]) || !strings.Contains(after[len(before)], `"interrupted":true`) || !strings.Contains(after[len(before)], "unfinished answer") {
				t.Fatal("restart changed the saved transcript")
			}
			// Repeated restarts must save the resumed run too, including changes that
			// occur while the transcript is unchanged.
			got.desktop.Model, c.mode = "new-model", providerConversationMode(provider, "")
			fresh.persistRuns()
			fresh.stopConversations()
			fresh.persistRuns()
			again := &agentDaemon{store: openRunStore(d.store.dir)}
			again.restoreRuns()
			t.Cleanup(again.stopConversations)
			if r := again.queue.runs[id]; r.conversation == nil || r.desktop.Model != "new-model" || r.conversation.mode != c.mode {
				t.Fatal("second restart lost changed settings")
			}
		})
	}
}

func TestConversationExplicitStopDoesNotBecomeResumable(t *testing.T) {
	d, id := conversationFixture(t)
	d.store = testRunStore(t)
	run := d.queue.runs[id]
	run.conversation.session = "ended-session"
	run.canceled = true
	d.stopConversations()
	d.persistRuns()
	fresh := &agentDaemon{store: openRunStore(d.store.dir)}
	fresh.restoreRuns()
	if got := fresh.queue.runs[id]; got == nil || !got.restored || got.conversation != nil {
		t.Fatal("explicit stop was undone by restart")
	}
}

func TestCodexRestartUsesStoredThreadAndSettings(t *testing.T) {
	d, id, log := codexConversationFixture(t)
	d.store = testRunStore(t)
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"first","mode":"plan","model":"chosen-model"}`, "private"); w.Code != 202 {
		t.Fatal(w.Code)
	}
	waitCodex(t, d, id, func(c *providerConversation) bool { return !c.busy })
	d.stopConversations()
	if lines, _ := d.queue.runs[id].trace.snapshot(); strings.Contains(strings.Join(lines, "\n"), "Conversation paused for agent restart") {
		t.Fatal("idle Codex conversation was reported as interrupted during shutdown")
	}
	d.persistRuns()
	fresh := &agentDaemon{store: openRunStore(d.store.dir), loopback: loopbackServer{desktopToken: "private"}}
	fresh.restoreRuns()
	t.Cleanup(fresh.stopConversations)
	before, _ := os.ReadFile(log)
	// Reading the desktop snapshot must not start a provider or replay a turn.
	w := conversationRequest(fresh, "GET", "/desktop/conversation?id="+id, "", "private")
	var snapshot map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &snapshot)
	if snapshot["readOnly"] != false || snapshot["model"] != "chosen-model" || snapshot["mode"] != "plan" {
		t.Fatal("desktop snapshot lost state")
	}
	after, _ := os.ReadFile(log)
	if string(before) != string(after) {
		t.Fatal("opening the restored history started Codex")
	}
	if w := conversationRequest(fresh, "POST", "/desktop/conversation?id="+id, `{"message":"second"}`, "private"); w.Code != 202 {
		t.Fatal(w.Code)
	}
	waitCodex(t, fresh, id, func(c *providerConversation) bool { return !c.busy && c.codex != nil })
	data, _ := os.ReadFile(log)
	tail := string(data[len(before):])
	if !strings.Contains(tail, `"method":"thread/resume"`) || !strings.Contains(tail, `"threadId":"thread-test"`) || strings.Contains(tail, `"method":"thread/start"`) || !strings.Contains(tail, `"model":"chosen-model"`) || !strings.Contains(tail, `"mode":"plan"`) || strings.Count(tail, `"method":"turn/start"`) != 1 {
		t.Fatalf("provider was not resumed with one new turn: %s", tail)
	}
}

func TestClaudeRestartUsesStoredSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a POSIX script")
	}
	d, id := conversationFixture(t)
	d.store = testRunStore(t)
	run := d.queue.runs[id]
	run.conversation.session, run.conversation.mode = "saved-claude-session", "plan"
	run.desktop.Model = "haiku"
	d.stopConversations()
	d.persistRuns()
	fresh := &agentDaemon{store: openRunStore(d.store.dir), loopback: loopbackServer{desktopToken: "private"}}
	fresh.restoreRuns()
	t.Cleanup(fresh.stopConversations)
	bin := t.TempDir()
	script := `#!/bin/sh
printf '%s\n' "$@" > args.txt
read -r init
read -r message
printf '%s\n' '{"type":"result","result":"Resumed","session_id":"saved-claude-session"}'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if w := conversationRequest(fresh, "POST", "/desktop/conversation?id="+id, `{"message":"continue"}`, "private"); w.Code != 202 {
		t.Fatal(w.Code)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		fresh.queue.mu.Lock()
		busy := fresh.queue.runs[id].conversation.busy
		fresh.queue.mu.Unlock()
		if !busy {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("resumed turn did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	args, _ := os.ReadFile(filepath.Join(run.root, "args.txt"))
	for _, want := range []string{"--resume\nsaved-claude-session", "--model\nhaiku", "--permission-mode\nplan"} {
		if !strings.Contains(string(args), want) {
			t.Fatalf("missing resumed option %s", want)
		}
	}
}

func TestConversationCrashRestoresOnlyAnIdleSession(t *testing.T) {
	d, id := conversationFixture(t)
	d.store = testRunStore(t)
	run := d.queue.runs[id]
	run.conversation.session = "crashed-session"
	run.conversation.busy = true
	run.conversation.next = []string{"unsent"}
	d.persistRuns()
	fresh := &agentDaemon{store: openRunStore(d.store.dir)}
	fresh.restoreRuns()
	t.Cleanup(fresh.stopConversations)
	c := fresh.queue.runs[id].conversation
	if c == nil || c.session != "crashed-session" || c.busy || len(c.next) > 0 || c.input != nil {
		t.Fatal("crash restored pending execution instead of an idle session")
	}
	// Let fixture cleanup finish immediately; no process was launched here.
	run.conversation.busy = false
}
