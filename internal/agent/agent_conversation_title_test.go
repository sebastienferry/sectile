package agent

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestConversationTitlePersistsWithoutAddingProviderMessages(t *testing.T) {
	d, id, run := journalFixture(t)
	conversationWrite(run.trace, "user", "First prompt", " ")
	before, _ := run.trace.snapshot()
	body, _ := json.Marshal(map[string]string{"id": id, "title": "  Review the project  "})
	got := conversationRequest(d, "POST", "/desktop/conversation/title", string(body), "private")
	if got.Code != 200 {
		t.Fatalf("rename: %d %s", got.Code, got.Body.String())
	}
	fresh := journalRestart(t, d)
	restored := fresh.queue.runs[id]
	after, _ := restored.trace.snapshot()
	if restored.desktop.Title != "Review the project" || strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatal("rename lost persistence or changed history")
	}
	if restored.desktop.Model != run.desktop.Model || restored.conversation.mode != run.conversation.mode {
		t.Fatal("rename changed conversation settings")
	}
	for _, title := range []string{"", strings.Repeat("é", 81), "two\nlines"} {
		body, _ = json.Marshal(map[string]string{"id": id, "title": title})
		if got := conversationRequest(d, "POST", "/desktop/conversation/title", string(body), "private"); got.Code != 400 {
			t.Fatal("invalid title accepted")
		}
	}
	run.desktop.TaskID = "remote-task"
	if got := conversationRequest(d, "POST", "/desktop/conversation/title", `{"id":"`+id+`","title":"local"}`, "private"); got.Code != 404 {
		t.Fatal("remote task title changed locally")
	}
}
func TestAutomaticConversationTitleUsesFirstLineAndUnicode(t *testing.T) {
	if got := automaticConversationTitle("  Explain   this project\nDetails"); got != "Explain this project" {
		t.Fatal(got)
	}
	got := automaticConversationTitle(strings.Repeat("é", 100))
	if utf8.RuneCountInString(got) != 80 || !strings.HasSuffix(got, "…") {
		t.Fatal("title truncated a Unicode character")
	}
}

func TestReadOnlyConversationTitlePersistsWithoutProviderState(t *testing.T) {
	d := &agentDaemon{store: testRunStore(t), loopback: loopbackServer{desktopToken: "private"}}
	record := storedRun{Version: runStoreVersion, Run: desktopRun{ID: "legacy-history", Status: "completed", Conversation: true, Provider: "claude", Directory: t.TempDir()}, Trace: []string{`{"kind":"user","text":"Legacy prompt"}`}}
	if err := d.store.save(record); err != nil {
		t.Fatal(err)
	}
	d.restoreRuns()
	if !d.queue.runs[record.Run.ID].restored {
		t.Fatal("legacy history unexpectedly became writable")
	}
	got := conversationRequest(d, "POST", "/desktop/conversation/title", `{"id":"legacy-history","title":"Saved history"}`, "private")
	if got.Code != 200 {
		t.Fatalf("read-only rename: %d %s", got.Code, got.Body.String())
	}
	again := journalRestart(t, d)
	if again.queue.runs[record.Run.ID].desktop.Title != "Saved history" {
		t.Fatal("read-only title did not survive restart")
	}
	lines, _ := again.queue.runs[record.Run.ID].trace.snapshot()
	if !strings.Contains(strings.Join(lines, "\n"), "Legacy prompt") {
		t.Fatal("read-only rename lost history")
	}
}
