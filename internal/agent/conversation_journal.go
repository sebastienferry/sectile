package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// Conversations commit normalized display events before publication. Provider
// credentials, raw protocol frames and pending approval answers never enter WAL.
// Lock order is queue -> trace -> journal -> store; journal never calls back up.
type conversationJournal struct {
	mu       sync.Mutex
	store    *runStore
	record   storedRun
	metadata string
	saved    uint64
	failure  error
	bytes    int
}

type conversationMetadata struct {
	Run          desktopRun          `json:"run"`
	Conversation *storedConversation `json:"conversation,omitempty"`
	FinishedAt   time.Time           `json:"finishedAt,omitzero"`
}

type conversationJournalEvent struct {
	Sequence uint64                `json:"seq"`
	Kind     string                `json:"kind"`
	Text     string                `json:"text,omitempty"`
	Metadata *conversationMetadata `json:"metadata,omitempty"`
}

const conversationJournalLimit = 4 * 1024 * 1024

func (s *runStore) journalPath(id string) string { return s.path(id) + ".journal" }

func (s *runStore) appendConversation(id string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.removed[id] {
		return nil
	}
	file, err := os.OpenFile(s.journalPath(id), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err = file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}

func (s *runStore) truncateConversation(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.removed[id] {
		return nil
	}
	file, err := os.OpenFile(s.journalPath(id), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func applyConversationJournal(record *storedRun, event conversationJournalEvent) error {
	switch event.Kind {
	case "metadata":
		if event.Metadata == nil || event.Metadata.Run.ID != record.Run.ID {
			return fmt.Errorf("journal metadata names another conversation")
		}
		record.Run, record.Conversation = event.Metadata.Run, event.Metadata.Conversation
		record.FinishedAt = event.Metadata.FinishedAt
	case "line":
		var display conversationEvent
		if json.Unmarshal([]byte(event.Text), &display) != nil || display.Kind == "" {
			return fmt.Errorf("invalid conversation event")
		}
		record.Trace = append(record.Trace, event.Text)
		if len(record.Trace) > traceRetained {
			record.Trace = append([]string(nil), record.Trace[len(record.Trace)-traceRetained:]...)
		}
		record.TraceVersion++
		if display.Kind == "assistant" {
			record.Partial = ""
		}
	case "delta":
		if len(record.Partial)+len(event.Text) > conversationPartialLimit {
			return fmt.Errorf("journal draft exceeds limit")
		}
		record.Partial += event.Text
	case "draft":
		if len(event.Text) > conversationPartialLimit {
			return fmt.Errorf("journal draft exceeds limit")
		}
		record.Partial = event.Text
	default:
		return fmt.Errorf("unknown conversation journal operation")
	}
	record.JournalSequence = event.Sequence
	return nil
}

func (j *conversationJournal) failed(err error) {
	if err != nil && j.failure == nil {
		log.Printf("[Agent] Conversation %s could not be saved: %v", j.record.Run.ID, err)
	}
	j.failure = err
}

// emitLocked writes and syncs before changing the visible trace/draft. Following
// an I/O error, keep the in-memory state and report degraded durability until a
// complete checkpoint repairs the journal; never append behind a torn write.
func (j *conversationJournal) emitLocked(event conversationJournalEvent) {
	event.Sequence = j.record.JournalSequence + 1
	data, err := json.Marshal(event)
	if err == nil && j.failure == nil {
		err = j.store.appendConversation(j.record.Run.ID, append(data, '\n'))
	}
	if err != nil {
		j.failed(err)
	}
	if err := applyConversationJournal(&j.record, event); err != nil {
		j.failed(err)
		return
	}
	j.bytes += len(data) + 1
	if j.bytes >= conversationJournalLimit {
		j.checkpointLocked()
	}
}

func (j *conversationJournal) line(text string) {
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.emitLocked(conversationJournalEvent{Kind: "line", Text: text})
}

func (j *conversationJournal) partial(text string) {
	if j == nil || text == "" {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if text == j.record.Partial {
		return
	}
	event := conversationJournalEvent{Kind: "draft", Text: text}
	if strings.HasPrefix(text, j.record.Partial) {
		event.Kind, event.Text = "delta", strings.TrimPrefix(text, j.record.Partial)
	}
	j.emitLocked(event)
}

func (j *conversationJournal) setMetadata(metadata conversationMetadata) {
	if j == nil {
		return
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if string(data) == j.metadata {
		return
	}
	// Keep immutable copies of desktop slices after the queue lock is released.
	var copy conversationMetadata
	if json.Unmarshal(data, &copy) != nil {
		return
	}
	j.emitLocked(conversationJournalEvent{Kind: "metadata", Metadata: &copy})
	j.metadata = string(data)
}

func (j *conversationJournal) checkpointLocked() {
	if j.saved == j.record.JournalSequence && j.failure == nil {
		return
	}
	if err := j.store.save(j.record); err != nil {
		j.failed(err)
		return
	}
	// A crash after rename but before truncation is harmless: recovery skips all
	// journal entries already covered by the snapshot's sequence number.
	if err := j.store.truncateConversation(j.record.Run.ID); err != nil {
		j.failed(err)
		return
	}
	j.saved, j.bytes = j.record.JournalSequence, 0
	j.failure = nil
}
func (j *conversationJournal) checkpoint() { j.mu.Lock(); defer j.mu.Unlock(); j.checkpointLocked() }
func (j *conversationJournal) draft() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.record.Partial
}
func (j *conversationJournal) err() error {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.failure
}

// recoverConversationJournal runs before the daemon publishes any runs. Only a
// complete, ordered prefix is applied. A torn tail is removed before new writes.
func (s *runStore) recoverConversationJournal(record *storedRun) {
	file, err := os.Open(s.journalPath(record.Run.ID))
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		log.Printf("[Agent] Conversation journal unavailable: %v", err)
		return
	}
	reader := bufio.NewReader(file)
	var valid int64
	broken := false
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF && len(line) == 0 {
			break
		}
		if err != nil {
			broken = true
			break
		}
		var event conversationJournalEvent
		if json.Unmarshal(bytes.TrimSpace(line), &event) != nil || event.Sequence == 0 {
			broken = true
			break
		}
		if event.Sequence > record.JournalSequence {
			if event.Sequence != record.JournalSequence+1 || applyConversationJournal(record, event) != nil {
				broken = true
				break
			}
		}
		valid += int64(len(line))
	}
	_ = file.Close()
	if broken {
		if err := os.Truncate(s.journalPath(record.Run.ID), valid); err != nil {
			log.Printf("[Agent] Conversation journal tail could not be repaired: %v", err)
		}
	}
}

func conversationRunMetadata(run *controlledRun) conversationMetadata {
	metadata := conversationMetadata{Run: run.desktop}
	metadata.Run.CancelRequested = false
	metadata.Run.WaitingSince = time.Time{}
	metadata.Run.QueueSequence = 0
	if c := run.conversation; c != nil && (!run.canceled || run.resumeAfterShutdown) && run.taskID == "" && len(c.env) == 0 {
		metadata.Conversation = &storedConversation{Session: c.session, Mode: providerConversationMode(run.desktop.Provider, c.mode), Effort: c.effort, ExtraDirs: append([]string(nil), c.extraDirs...)}
		metadata.Run.Status = "running"
	}
	if run.canceled && !run.resumeAfterShutdown {
		metadata.Run.Status = conversationStoppedStatus(run)
		if run.finishedAt.IsZero() {
			run.finishedAt = time.Now().UTC()
		}
		metadata.FinishedAt = run.finishedAt
	}
	return metadata
}

func (d *agentDaemon) enableConversationJournalLocked(run *controlledRun) error {
	if d.store == nil || run.journal != nil || run.conversation == nil || run.taskID != "" || len(run.conversation.env) > 0 || (run.desktop.Provider != "claude" && run.desktop.Provider != "codex") {
		return nil
	}
	trace := run.trace
	trace.mu.Lock()
	defer trace.mu.Unlock()
	metadata := conversationRunMetadata(run)
	record := storedRun{Run: metadata.Run, Conversation: metadata.Conversation, Trace: append([]string(nil), trace.lines...), TraceVersion: trace.version, Partial: run.conversation.partial, FinishedAt: metadata.FinishedAt}
	if err := d.store.save(record); err != nil {
		return err
	}
	j := &conversationJournal{store: d.store, record: record}
	data, _ := json.Marshal(metadata)
	j.metadata = string(data)
	run.journal, run.conversation.journal, trace.journal = j, j, j
	return nil
}

func (d *agentDaemon) updateConversationJournalLocked(run *controlledRun) {
	if err := d.enableConversationJournalLocked(run); err != nil {
		log.Printf("[Agent] Conversation journal could not start: %v", err)
		return
	}
	if run.journal != nil {
		run.journal.setMetadata(conversationRunMetadata(run))
	}
}

// Keep an unfinished reply as a separate, clearly marked history entry. The
// committed event clears the saved draft atomically, including across crashes
// during recovery itself. It never becomes a prompt or an instruction to retry.
func preserveConversationDraft(run *controlledRun) {
	if run.journal == nil {
		return
	}
	if text := run.journal.draft(); text != "" {
		conversationWriteEvent(run.trace, conversationEvent{Kind: "assistant", Text: text, Interrupted: true})
	}
}
