package agent

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func journalFixture(t *testing.T) (*agentDaemon, string, *controlledRun) {
	t.Helper()
	d, id := conversationFixture(t)
	d.store = testRunStore(t)
	run := d.queue.runs[id]
	run.conversation.session = "journal-session"
	d.queue.mu.Lock()
	err := d.enableConversationJournalLocked(run)
	d.queue.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	return d, id, run
}
func journalRestart(t *testing.T, d *agentDaemon) *agentDaemon {
	t.Helper()
	fresh := &agentDaemon{store: openRunStore(d.store.dir), loopback: loopbackServer{desktopToken: "private"}}
	fresh.restoreRuns()
	t.Cleanup(fresh.stopConversations)
	return fresh
}

func TestConversationJournalRestoresEveryEventAndDraftWithoutFlush(t *testing.T) {
	d, id, run := journalFixture(t)
	for _, event := range []conversationEvent{
		{Kind: "user", Text: "latest prompt"},
		{Kind: "tool", Tool: "Read", ToolID: "read-1", Text: "Read", Input: json.RawMessage(`{"file_path":"test.txt"}`)},
		{Kind: "tool_result", ToolID: "read-1", Text: "latest result"},
		{Kind: "assistant", Text: "completed answer"},
	} {
		conversationWriteEvent(run.trace, event)
	}
	d.queue.mu.Lock()
	run.conversation.setPartial("unfinished ")
	run.conversation.setPartial("unfinished Markdown **reply")
	d.queue.mu.Unlock()
	before, version := run.trace.snapshot()
	// No persistRuns or periodic loop: restore only from the initial snapshot + WAL.
	fresh := journalRestart(t, d)
	after, newVersion := fresh.queue.runs[id].trace.snapshot()
	if !reflect.DeepEqual(before, after[:len(before)]) || len(after) != len(before)+1 || newVersion != version+1 {
		t.Fatal("journal lost or duplicated normalized events")
	}
	var fragment conversationEvent
	_ = json.Unmarshal([]byte(after[len(before)]), &fragment)
	if fragment.Text != "unfinished Markdown **reply" || !fragment.Interrupted {
		t.Fatal("draft was not preserved as interrupted history")
	}
	again := journalRestart(t, fresh)
	repeated, _ := again.queue.runs[id].trace.snapshot()
	if !reflect.DeepEqual(after, repeated) {
		t.Fatal("draft was duplicated during another restart")
	}
}

func TestConversationJournalCompleteReplyAtomicallyReplacesDraft(t *testing.T) {
	d, id, run := journalFixture(t)
	run.conversation.setPartial("partial")
	conversationWrite(run.trace, "assistant", "complete", "")
	fresh := journalRestart(t, d)
	lines, _ := fresh.queue.runs[id].trace.snapshot()
	text := strings.Join(lines, "\n")
	if strings.Contains(text, "partial") || strings.Count(text, `"kind":"assistant"`) != 1 || strings.Contains(text, `"interrupted":true`) {
		t.Fatal("completed reply left a duplicate recovered draft")
	}
}

func TestConversationJournalRepairsTornTailBeforeAppending(t *testing.T) {
	d, id, run := journalFixture(t)
	conversationWrite(run.trace, "user", "durable", "")
	f, err := os.OpenFile(d.store.journalPath(id), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`{"seq":3,"kind":"delta","text":"torn`)
	_ = f.Close()
	fresh := journalRestart(t, d)
	conversationWrite(fresh.queue.runs[id].trace, "assistant", "after repair", "")
	again := journalRestart(t, fresh)
	lines, _ := again.queue.runs[id].trace.snapshot()
	text := strings.Join(lines, "\n")
	if !strings.Contains(text, "durable") || !strings.Contains(text, "after repair") || strings.Contains(text, "torn") {
		t.Fatal("torn WAL tail swallowed subsequent events")
	}
}

func TestConversationJournalCheckpointRenameDoesNotReplayOldEntries(t *testing.T) {
	d, id, run := journalFixture(t)
	conversationWrite(run.trace, "user", "once", "")
	j := run.journal
	j.mu.Lock()
	// Simulate a crash after snapshot replacement but before journal truncation.
	err := d.store.save(j.record)
	j.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	conversationWrite(run.trace, "assistant", "newer", "")
	fresh := journalRestart(t, d)
	lines, _ := fresh.queue.runs[id].trace.snapshot()
	text := strings.Join(lines, "\n")
	if strings.Count(text, "once") != 1 || strings.Count(text, "newer") != 1 {
		t.Fatal("checkpoint recovery duplicated or lost events")
	}
}

func TestConversationJournalFailureRejectsUnpersistedPrompt(t *testing.T) {
	d, id, run := journalFixture(t)
	path := d.store.journalPath(id)
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	w := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"must not launch"}`, "private")
	if w.Code != 503 || run.conversation.busy {
		t.Fatal("failed persistence dispatched a prompt or left it busy")
	}
	w = conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private")
	if !strings.Contains(w.Body.String(), "persistenceError") {
		t.Fatal("persistence failure was hidden")
	}
}

func TestConversationJournalClearCannotResurrectHistory(t *testing.T) {
	d, id, run := journalFixture(t)
	conversationWrite(run.trace, "user", "before clear", "")
	d.store.remove(id)
	conversationWrite(run.trace, "assistant", "after clear", "")
	run.journal.checkpoint()
	for _, p := range []string{d.store.path(id), d.store.journalPath(id)} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatal("cleared conversation reappeared")
		}
	}
}

func TestConversationJournalKeepsDraftWithoutProviderIdentity(t *testing.T) {
	d, id, run := journalFixture(t)
	d.queue.mu.Lock()
	run.conversation.session = ""
	d.updateConversationJournalLocked(run)
	run.conversation.setPartial("reply without a provider identity")
	d.queue.mu.Unlock()
	fresh := journalRestart(t, d)
	got := fresh.queue.runs[id]
	lines, _ := got.trace.snapshot()
	if !got.restored || !strings.Contains(strings.Join(lines, "\n"), "reply without a provider identity") {
		t.Fatal("read-only recovery lost an unfinished reply")
	}
	again := journalRestart(t, fresh)
	twice, _ := again.queue.runs[id].trace.snapshot()
	if !reflect.DeepEqual(lines, twice) {
		t.Fatal("read-only draft was promoted twice")
	}
}

func BenchmarkConversationJournalDelta(b *testing.B) {
	store := openRunStore(b.TempDir())
	record := storedRun{Run: desktopRun{ID: "benchmark", Status: "running", Conversation: true}}
	if err := store.save(record); err != nil {
		b.Fatal(err)
	}
	journal := &conversationJournal{store: store, record: record}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		journal.partial(strings.Repeat("x", i%512+1))
	}
	b.StopTimer()
	if err := journal.err(); err != nil {
		b.Fatal(err)
	}
}
