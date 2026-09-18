package trackerapi

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"tasks/internal/models"
	"time"
)

// jiraBaseFields is what every read of a work item asks for. The sprint and
// team custom field ids are appended per site.
var jiraBaseFields = []string{
	"summary", "description", "status", "priority", "assignee", "labels",
	"issuetype", "parent", "created", "updated", "statuscategorychangedate",
}

// jiraTimeLayouts are the stamps Jira Cloud writes ("2026-08-25T09:12:33.000+0200").
var jiraTimeLayouts = []string{"2006-01-02T15:04:05.000-0700", "2006-01-02T15:04:05.000Z07:00", time.RFC3339}

func parseJiraTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range jiraTimeLayouts {
		if ts, err := time.Parse(layout, value); err == nil {
			return &ts
		}
	}
	return nil
}

// jiraIssue is the typed part of a work item; the custom fields are read from
// Raw because their ids are site-specific.
type jiraIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"`
		Status      *struct {
			Name           string `json:"name"`
			StatusCategory struct {
				Key string `json:"key"`
			} `json:"statusCategory"`
		} `json:"status"`
		Priority *struct {
			Name string `json:"name"`
		} `json:"priority"`
		Assignee *struct {
			AccountID   string `json:"accountId"`
			DisplayName string `json:"displayName"`
		} `json:"assignee"`
		Labels    []string `json:"labels"`
		IssueType *struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Parent *struct {
			Key    string `json:"key"`
			Fields struct {
				Summary   string `json:"summary"`
				IssueType *struct {
					Name string `json:"name"`
				} `json:"issuetype"`
			} `json:"fields"`
		} `json:"parent"`
		Created                  string `json:"created"`
		Updated                  string `json:"updated"`
		StatusCategoryChangeDate string `json:"statuscategorychangedate"`
	} `json:"fields"`
	Raw map[string]json.RawMessage `json:"-"`
}

func decodeJiraIssue(raw json.RawMessage) (*jiraIssue, error) {
	var issue jiraIssue
	if err := json.Unmarshal(raw, &issue); err != nil {
		return nil, fmt.Errorf("Jira returned an unreadable work item: %w", err)
	}
	if strings.TrimSpace(issue.Key) == "" {
		return nil, fmt.Errorf("Jira returned a work item without a key")
	}
	var dynamic struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	_ = json.Unmarshal(raw, &dynamic)
	issue.Raw = dynamic.Fields
	return &issue, nil
}

// jiraPriority maps Jira's named priorities onto the four Sectile has.
func jiraPriority(name string) models.Priority {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "highest", "blocker", "critical":
		return models.PriorityUrgent
	case "high", "major":
		return models.PriorityHigh
	case "low", "lowest", "minor", "trivial":
		return models.PriorityLow
	default:
		return models.PriorityMedium
	}
}

// jiraPriorityName is the reverse mapping, for a write.
func jiraPriorityName(p models.Priority) string {
	switch p {
	case models.PriorityUrgent:
		return "Highest"
	case models.PriorityHigh:
		return "High"
	case models.PriorityLow:
		return "Low"
	default:
		return "Medium"
	}
}

// jiraWorkflowStatus applies the label rule every tracker shares: a workflow
// label decides the Sectile status. It answers false when no label decided.
func jiraWorkflowStatus(labels []string) (models.Status, bool) {
	status := models.StatusToClarify
	decided := false
	for _, l := range labels {
		switch strings.ToLower(strings.TrimPrefix(strings.TrimSpace(l), "#")) {
		case "new", "untouched":
			status, decided = models.StatusToClarify, true
		case "clarified":
			status, decided = models.StatusClarified, true
		case "specified":
			status, decided = models.StatusToImplement, true
		case "implemented":
			status, decided = models.StatusToTest, true
		case "reviewed":
			status, decided = models.StatusToClose, true
		case "finished", "closed", "done":
			status, decided = models.StatusFinished, true
		}
	}
	return status, decided
}

// jiraTask converts a work item into a task. site is the base URL the browse
// link is built on; fields carries the discovered custom field ids.
func jiraTask(site string, issue *jiraIssue, fields jiraFieldIDs) *models.Task {
	key := strings.ToUpper(strings.TrimSpace(issue.Key))
	status, decided := jiraWorkflowStatus(issue.Fields.Labels)
	trackerStatus := ""
	if issue.Fields.Status != nil {
		trackerStatus = issue.Fields.Status.Name
		if !decided && strings.EqualFold(issue.Fields.Status.StatusCategory.Key, "done") {
			status = models.StatusFinished
		}
	}

	priority := models.PriorityMedium
	if issue.Fields.Priority != nil {
		priority = jiraPriority(issue.Fields.Priority.Name)
	}
	assignee := ""
	if issue.Fields.Assignee != nil {
		assignee = strings.TrimSpace(issue.Fields.Assignee.DisplayName)
		if assignee == "" {
			assignee = issue.Fields.Assignee.AccountID
		}
	}
	issueType := ""
	if issue.Fields.IssueType != nil {
		issueType = issue.Fields.IssueType.Name
	}
	parentKey, parentTitle, parentType := "", "", ""
	if issue.Fields.Parent != nil {
		parentKey = strings.ToUpper(strings.TrimSpace(issue.Fields.Parent.Key))
		parentTitle = issue.Fields.Parent.Fields.Summary
		if issue.Fields.Parent.Fields.IssueType != nil {
			parentType = issue.Fields.Parent.Fields.IssueType.Name
		}
		if parentType == "" {
			parentType = "Epic"
		}
	}
	sprint, teamID, team := "", "", ""
	if fields.Sprint != "" {
		sprint = parseJiraSprint(issue.Raw[fields.Sprint])
	}
	if fields.Team != "" {
		teamID, team = parseJiraTeam(issue.Raw[fields.Team])
	}

	labels := append([]string{}, issue.Fields.Labels...)
	if labels == nil {
		labels = []string{}
	}
	now := time.Now()
	task := &models.Task{
		ID:               "jira-" + key,
		Key:              key,
		Title:            issue.Fields.Summary,
		Description:      ADFToMarkdown(issue.Fields.Description),
		Status:           status,
		TrackerStatus:    trackerStatus,
		Priority:         priority,
		Labels:           labels,
		Assignee:         assignee,
		Sprint:           sprint,
		Team:             team,
		TeamID:           teamID,
		Source:           "jira",
		IssueType:        issueType,
		ParentKey:        parentKey,
		ParentTitle:      parentTitle,
		ParentType:       parentType,
		TrackerCreatedAt: parseJiraTime(issue.Fields.Created),
		TrackerUpdatedAt: parseJiraTime(issue.Fields.Updated),
		StatusChangedAt:  parseJiraTime(issue.Fields.StatusCategoryChangeDate),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if site != "" {
		u := jiraBrowseURL(site, key)
		task.ExternalURL = &u
	}
	return task
}

func jiraBrowseURL(site, key string) string {
	return strings.TrimSuffix(site, "/") + "/browse/" + key
}

// jiraProjectKey is the project's Jira key: the configured one, else the slug
// upper-cased without dashes, which is what projects created before the
// dedicated field stored.
func jiraProjectKey(p *models.Project) string {
	if p == nil {
		return ""
	}
	if key := strings.ToUpper(strings.TrimSpace(p.JiraProject)); key != "" {
		return key
	}
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(p.Slug), "-", ""))
}

// jiraJQL builds the project query, restricted to the configured issue types.
func jiraJQL(projectKey string, issueTypes []string, extra string) string {
	jql := "project = " + projectKey
	var quoted []string
	for _, t := range issueTypes {
		if t = strings.TrimSpace(t); t != "" {
			quoted = append(quoted, `"`+strings.ReplaceAll(t, `"`, `\"`)+`"`)
		}
	}
	if len(quoted) > 0 {
		jql += " AND issuetype IN (" + strings.Join(quoted, ", ") + ")"
	}
	if extra != "" {
		jql += " AND " + extra
	}
	return jql + " ORDER BY updated DESC"
}

// jiraKeyPattern is what a Jira key looks like: a project key, a dash, a
// number. Matching it is what tells a bare key from the local identity that
// wraps one, without assuming a prefix "JIRA" could not be a project's own key.
var jiraKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*-[0-9]+$`)

// cleanJiraKey accepts a Jira key in any of the forms Sectile stores, the bare
// key and the local identity jira-<projectID>-<KEY>, and returns the bare
// upper-cased key. It refuses anything else, a GitHub-shaped key included,
// rather than sending it to a site.
func cleanJiraKey(key string) (string, error) {
	s := strings.ToUpper(strings.TrimSpace(key))
	if jiraKeyPattern.MatchString(s) {
		return s, nil
	}
	// A local identity ends with the key, whatever the project id is made of —
	// but it also begins with JIRA-, and requiring that is what keeps the last
	// two segments of somebody else's identity out. Without it "gh-default-42"
	// came back as the perfectly plausible Jira key "DEFAULT-42".
	if rest, ok := strings.CutPrefix(s, "JIRA-"); ok {
		if parts := strings.Split(rest, "-"); len(parts) >= 2 {
			candidate := parts[len(parts)-2] + "-" + parts[len(parts)-1]
			if jiraKeyPattern.MatchString(candidate) {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("issue key %q is not a Jira key", key)
}
