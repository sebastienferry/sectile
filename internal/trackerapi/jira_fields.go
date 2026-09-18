package trackerapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Sprint and Team are custom fields whose ids differ per Jira site. They are
// looked up by schema then by display name, and remembered for a while: a field
// id does not change within a site, but which fields exist does — an
// administrator adding the Team field to a site that had none must not need a
// server restart to be seen. Hence an expiry rather than a permanent answer:
// half a discovery kept forever meant the site was reported as having no Team
// field for as long as the process lived.

type jiraFieldIDs struct {
	Sprint string
	Team   string
}

// jiraFieldTTL is how long a discovery stands. Long enough that it is not one
// call per operation, short enough that a field added today is seen today.
const jiraFieldTTL = 10 * time.Minute

type jiraFieldEntry struct {
	ids    jiraFieldIDs
	expiry time.Time
}

var jiraFieldCache sync.Map // base URL -> jiraFieldEntry

// resetJiraFieldCache forgets every discovered id, for tests that rebuild a
// site under the same URL.
func resetJiraFieldCache() { jiraFieldCache = sync.Map{} }

// jiraFields returns the sprint and team field ids of the client's site. A site
// exposing neither is not an error: the sync then leaves sprint and team empty.
// A site that could not be asked is an error, and the caller must not confuse
// the two — importing every work item with no sprint and no team looks exactly
// like a site that has neither.
func (c *Client) jiraFields(ctx context.Context) (jiraFieldIDs, error) {
	if cached, ok := jiraFieldCache.Load(c.JiraURL); ok {
		if entry, ok := cached.(jiraFieldEntry); ok && time.Now().Before(entry.expiry) {
			return entry.ids, nil
		}
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
	jiraFieldCache.Store(c.JiraURL, jiraFieldEntry{ids: ids, expiry: time.Now().Add(jiraFieldTTL)})
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
//
// The field also comes back as a bare string on many sites, and that string is
// the id — it is the form jiraSetTeam writes first. Returning it as the name
// put a UUID on the board and left nothing to resolve the team's members with.
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
		text := strings.TrimSpace(asText)
		if looksLikeTeamID(text) {
			return text, ""
		}
		return "", text
	}
	return "", ""
}

// looksLikeTeamID tells an identifier from a label. Jira team ids are UUIDs,
// sometimes with a site suffix, or Atlassian resource names; a team called
// "Platform" is neither. A team named in nothing but hexadecimal letters would
// be read as an id, which is the price of not knowing which of the two a bare
// string is.
func looksLikeTeamID(text string) bool {
	if text == "" {
		return false
	}
	if strings.HasPrefix(text, "ari:") {
		return true
	}
	for _, r := range text {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F', r == '-':
		default:
			return false
		}
	}
	// A bare word of fewer than eight hex characters is far more likely to be a
	// short label than an identifier.
	return len(text) >= 8
}
