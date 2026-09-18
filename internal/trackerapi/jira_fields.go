package trackerapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// Sprint and Team are custom fields whose ids differ per Jira site. They are
// looked up once per site, by schema then by display name, and remembered for
// the life of the process: a field id does not change within a site.

type jiraFieldIDs struct {
	Sprint string
	Team   string
}

var jiraFieldCache sync.Map // base URL -> jiraFieldIDs

// resetJiraFieldCache forgets every discovered id, for tests that rebuild a
// site under the same URL.
func resetJiraFieldCache() { jiraFieldCache = sync.Map{} }

// jiraFields returns the sprint and team field ids of the client's site. A site
// exposing neither is not an error: the sync then leaves sprint and team
// empty, and the answer is not cached so a later configuration can be found.
func (c *Client) jiraFields(ctx context.Context) (jiraFieldIDs, error) {
	if cached, ok := jiraFieldCache.Load(c.JiraURL); ok {
		return cached.(jiraFieldIDs), nil
	}
	var fields []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Schema struct {
			Type   string `json:"type"`
			Custom string `json:"custom"`
		} `json:"schema"`
	}
	if err := c.jira(ctx, http.MethodGet, "/rest/api/3/field", nil, nil, &fields); err != nil {
		return jiraFieldIDs{}, err
	}
	var ids jiraFieldIDs
	for _, f := range fields {
		switch {
		case ids.Sprint == "" && strings.HasSuffix(f.Schema.Custom, ":gh-sprint"):
			ids.Sprint = f.ID
		case ids.Team == "" && (f.Schema.Type == "team" || strings.HasSuffix(f.Schema.Custom, ":atlassian-team")):
			ids.Team = f.ID
		}
	}
	// Fall back on the display name when the schema does not identify the
	// field: older sites, or a locally defined "Team" field.
	for _, f := range fields {
		name := strings.ToLower(strings.TrimSpace(f.Name))
		if ids.Sprint == "" && name == "sprint" {
			ids.Sprint = f.ID
		}
		if ids.Team == "" && (name == "team" || name == "équipe" || name == "equipe") {
			ids.Team = f.ID
		}
	}
	if ids.Sprint != "" || ids.Team != "" {
		jiraFieldCache.Store(c.JiraURL, ids)
	}
	return ids, nil
}

// parseJiraSprint picks the sprint to display. A work item carried across
// sprints holds several: the active one wins, otherwise the last of the list,
// which is the most recent one Jira reports.
func parseJiraSprint(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	type sprint struct {
		Name  string `json:"name"`
		State string `json:"state"`
	}
	var sprints []sprint
	if err := json.Unmarshal(raw, &sprints); err == nil {
		chosen := ""
		for _, s := range sprints {
			if strings.TrimSpace(s.Name) == "" {
				continue
			}
			if strings.EqualFold(s.State, "active") {
				return s.Name
			}
			chosen = s.Name
		}
		return chosen
	}
	var single sprint
	if err := json.Unmarshal(raw, &single); err == nil && single.Name != "" {
		return single.Name
	}
	var asText string
	if err := json.Unmarshal(raw, &asText); err == nil {
		return strings.TrimSpace(asText)
	}
	return ""
}

// parseJiraTeam reads the Team field, an object whose label lives under one of
// several keys depending on the site. The id is kept verbatim: a site-scoped
// team carries a suffix that is part of the id, not noise to trim.
func parseJiraTeam(raw json.RawMessage) (id string, name string) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", ""
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		if v, ok := obj["id"].(string); ok {
			id = strings.TrimSpace(v)
		}
		for _, key := range []string{"name", "title", "value", "displayName"} {
			if v, ok := obj[key].(string); ok && strings.TrimSpace(v) != "" {
				return id, strings.TrimSpace(v)
			}
		}
		return id, ""
	}
	var asText string
	if err := json.Unmarshal(raw, &asText); err == nil {
		return "", strings.TrimSpace(asText)
	}
	return "", ""
}
