package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ReportMetadata identifies one immutable report publication, including retries.
type ReportMetadata struct {
	PublicationID string `json:"publicationId"`
	Stage         string `json:"stage"`
	Round         int    `json:"round,omitempty"`
	Summary       string `json:"summary"`
}

func (r ReportMetadata) Validate(stage string) error {
	switch r.Stage {
	case "clarified", "specified", "implemented", "reviewed", "finished":
	default:
		return fmt.Errorf("étape de rapport invalide")
	}
	if stage != "" && stage != r.Stage {
		return fmt.Errorf("l'étape du rapport ne correspond pas à la transition")
	}
	if strings.TrimSpace(r.PublicationID) == "" || len(r.PublicationID) > 200 || strings.TrimSpace(r.Summary) == "" {
		return fmt.Errorf("identifiant de publication et résumé requis")
	}
	if r.Round < 0 || (r.Round > 0 && r.Stage != "clarified") {
		return fmt.Errorf("numéro de clarification invalide")
	}
	return nil
}

type ReportAttachment struct {
	ID   string `json:"id"`
	Name string `json:"filename"`
	URL  string `json:"content"`
}

// TaskReport preserves the supplied body independently of tracker availability.
type TaskReport struct {
	ID        string `json:"reportId"`
	TaskID    string `json:"taskId"`
	TrackerID string `json:"trackerId"`
	ActorID   string `json:"actorId"`
	ReportMetadata
	Body          string            `json:"body"`
	PRURLs        []string          `json:"prUrls,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
	Attachment    *ReportAttachment `json:"attachment,omitempty"`
	UploadStatus  string            `json:"uploadStatus"`
	CommentID     string            `json:"commentId,omitempty"`
	CommentStatus string            `json:"commentStatus"`
	CommentError  string            `json:"commentError,omitempty"`
	Warning       string            `json:"warning,omitempty"`
}

// Jira returns attachment IDs as numbers on some sites and strings on others.
func (a *ReportAttachment) UnmarshalJSON(raw []byte) error {
	var wire struct {
		ID   json.RawMessage `json:"id"`
		Name string          `json:"filename"`
		URL  string          `json:"content"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return err
	}
	var id string
	if len(wire.ID) > 0 && string(wire.ID) != "null" {
		if wire.ID[0] == '"' {
			if err := json.Unmarshal(wire.ID, &id); err != nil {
				return err
			}
		} else {
			var number json.Number
			if err := json.Unmarshal(wire.ID, &number); err != nil {
				return err
			}
			id = number.String()
		}
	}
	a.ID = id
	a.Name = wire.Name
	a.URL = wire.URL
	return nil
}
