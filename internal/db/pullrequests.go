package db

import (
	"encoding/json"
	"strings"

	"tasks/internal/models"
)

// The set rules themselves live in `internal/models`, shared with the agent's
// own adjustment pre-check. What belongs here is only how the set is stored:
// a JSON column, alongside `pr_url`, which is the set's last link and is written
// by the same statement so the two never diverge.

// decodePullRequestLinks reads the stored set. A row written before the column
// existed, or one whose content is unreadable, reads as an empty set rather
// than failing the task read: the links are evidence, not the task itself.
func decodePullRequestLinks(raw string) []models.TaskPullRequest {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var links []models.TaskPullRequest
	if err := json.Unmarshal([]byte(raw), &links); err != nil {
		return nil
	}
	return models.NormalizePullRequestLinks(links)
}

func encodePullRequestLinks(links []models.TaskPullRequest) string {
	links = models.NormalizePullRequestLinks(links)
	if len(links) == 0 {
		return "[]"
	}
	encoded, err := json.Marshal(links)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

// pullRequestURLValue turns the current PR into what the `pr_url` column holds,
// NULL once every link has been detached.
func pullRequestURLValue(links []models.TaskPullRequest) *string {
	url := models.CurrentPullRequest(links)
	if url == "" {
		return nil
	}
	return &url
}
