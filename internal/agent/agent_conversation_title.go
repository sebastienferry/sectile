package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxConversationTitle = 80

func automaticConversationTitle(message string) string {
	line := strings.Split(strings.TrimSpace(message), "\n")[0]
	runes := []rune(strings.Join(strings.Fields(line), " "))
	if len(runes) > maxConversationTitle {
		runes = runes[:maxConversationTitle-1]
		return string(runes) + "…"
	}
	return string(runes)
}

func taskSkillLabel(skillID string) string {
	id := strings.TrimSpace(skillID)
	if idx := strings.LastIndex(id, ":"); idx >= 0 {
		id = id[idx+1:]
	}
	id = strings.TrimSuffix(id, "-issue")
	switch id {
	case "clarify":
		return "Clarify"
	case "specify":
		return "Specify"
	case "implement":
		return "Implement"
	case "adjust":
		return "Adjust"
	case "handoff":
		return "Handoff"
	case "pickup":
		return "Pickup"
	case "discuss":
		return "Discuss"
	case "create_pr":
		return "Create PR"
	case "":
		return ""
	default:
		runes := []rune(id)
		runes[0] = unicode.ToUpper(runes[0])
		return string(runes)
	}
}

func taskConversationTitle(taskTitle, skillID string) string {
	base := strings.Join(strings.Fields(strings.TrimSpace(taskTitle)), " ")
	skill := taskSkillLabel(skillID)
	if base == "" {
		return skill
	}
	if skill == "" {
		return base
	}
	suffix := " - " + skill
	if strings.HasSuffix(strings.ToLower(base), strings.ToLower(suffix)) {
		return base
	}
	return base + suffix
}

func titleFromConversationTrace(lines []string) string {
	for _, line := range lines {
		var event conversationEvent
		if json.Unmarshal([]byte(line), &event) == nil && event.Kind == "user" {
			if title := automaticConversationTitle(event.Text); title != "" {
				return title
			}
		}
	}
	return ""
}

func (d *agentDaemon) desktopConversationTitle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var input struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		http.Error(w, "Invalid conversation title", 400)
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" || utf8.RuneCountInString(input.Title) > maxConversationTitle || strings.IndexFunc(input.Title, unicode.IsControl) >= 0 {
		http.Error(w, "Choose a single-line title between 1 and 80 characters", 400)
		return
	}
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	run := d.queue.runs[input.ID]
	if run == nil || !run.desktop.Conversation || run.desktop.TaskID != "" {
		http.Error(w, "Project conversation not found", 404)
		return
	}
	if d.store == nil {
		http.Error(w, "Local history storage unavailable", 503)
		return
	}
	previousTitle := run.desktop.Title
	saved := false
	defer func() {
		if !saved {
			run.desktop.Title = previousTitle
		}
	}()
	run.desktop.Title = input.Title
	if run.journal != nil {
		d.updateConversationJournalLocked(run)
		if run.journal.err() != nil {
			http.Error(w, "Could not save title locally", 503)
			return
		}
	} else {
		// A read-only history may be renamed without reconstructing a provider.
		// Keep the sequence watermark so an old WAL cannot overwrite the new title.
		raw, err := os.ReadFile(d.store.path(input.ID))
		var record storedRun
		if err == nil {
			err = json.Unmarshal(raw, &record)
		}
		if err != nil || record.Run.ID != input.ID || record.Version != runStoreVersion {
			http.Error(w, "Saved conversation unavailable", 503)
			return
		}
		d.store.recoverConversationJournal(&record)
		record.Run.Title = input.Title
		if err := d.store.save(record); err != nil {
			http.Error(w, fmt.Sprintf("Could not save title: %v", err), 503)
			return
		}
	}
	saved = true
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"id": input.ID, "title": input.Title})
}
