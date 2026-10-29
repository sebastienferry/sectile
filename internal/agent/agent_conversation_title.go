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
		http.Error(w, "Nom de conversation invalide", 400)
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" || utf8.RuneCountInString(input.Title) > maxConversationTitle || strings.IndexFunc(input.Title, unicode.IsControl) >= 0 {
		http.Error(w, "Choisissez un nom de 1 à 80 caractères sur une seule ligne", 400)
		return
	}
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	run := d.queue.runs[input.ID]
	if run == nil || !run.desktop.Conversation || run.desktop.TaskID != "" {
		http.Error(w, "Conversation de projet introuvable", 404)
		return
	}
	if d.store == nil {
		http.Error(w, "Stockage local de l’historique indisponible", 503)
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
			http.Error(w, "Impossible d’enregistrer le nom localement", 503)
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
			http.Error(w, "Conversation enregistrée indisponible", 503)
			return
		}
		d.store.recoverConversationJournal(&record)
		record.Run.Title = input.Title
		if err := d.store.save(record); err != nil {
			http.Error(w, fmt.Sprintf("Impossible d’enregistrer le nom : %v", err), 503)
			return
		}
	}
	saved = true
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"id": input.ID, "title": input.Title})
}
