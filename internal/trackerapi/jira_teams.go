package trackerapi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"tasks/internal/models"
)

// Atlassian teams do not live in the Jira API: a work item carries a team id in
// its Team field, but the people behind that id are served by the teams
// service, under /gateway/api/v4/teams on the site itself. It takes the site id
// (the cloud id) as a query parameter and answers with account ids only, so the
// display names, e-mails and avatars come from the Jira user API.
//
// The whole chain works with the same Basic auth as the rest of the adapter,
// which is what makes it usable at all: the organisation-scoped public Teams
// API needs an org-admin token. The gateway path is not a documented part of
// the Jira API, so everything that touches it lives in this file and a failure
// here is meant to degrade to "members unknown", never to fail a sync.

const (
	jiraTeamMembersPageSize = 50
	jiraTeamMembersMaxPages = 20
	jiraUserBulkBatch       = 40
)

// jiraCloudID reads the site id the teams endpoints take.
func (c *Client) jiraCloudID(ctx context.Context) (string, error) {
	var payload struct {
		CloudID string `json:"cloudId"`
	}
	if err := c.jira(ctx, http.MethodGet, "/_edge/tenant_info", nil, nil, &payload); err != nil {
		return "", fmt.Errorf("Jira site id unreadable: %w", err)
	}
	if strings.TrimSpace(payload.CloudID) == "" {
		return "", fmt.Errorf("the Jira site returned no cloudId")
	}
	return strings.TrimSpace(payload.CloudID), nil
}

// jiraTeamMemberAccountIDs returns the account ids of a team's full members: an
// invited-but-not-joined entry is not somebody work can be assigned to.
func (c *Client) jiraTeamMemberAccountIDs(ctx context.Context, teamID string) ([]string, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	siteID, err := c.jiraCloudID(ctx)
	if err != nil {
		return nil, err
	}
	var accountIDs []string
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < jiraTeamMembersMaxPages; page++ {
		query := url.Values{}
		query.Set("siteId", siteID)
		query.Set("first", fmt.Sprint(jiraTeamMembersPageSize))
		if cursor != "" {
			query.Set("after", cursor)
		}
		var payload struct {
			Entities []struct {
				MembershipID struct {
					MemberID string `json:"memberId"`
				} `json:"membershipId"`
				State string `json:"state"`
			} `json:"entities"`
			Cursor *string `json:"cursor"`
		}
		if err := c.jira(ctx, http.MethodGet, "/gateway/api/v4/teams/"+url.PathEscape(teamID)+"/members", query, nil, &payload); err != nil {
			return accountIDs, err
		}
		for _, e := range payload.Entities {
			id := strings.TrimSpace(e.MembershipID.MemberID)
			if id == "" || seen[id] {
				continue
			}
			if e.State != "" && !strings.EqualFold(e.State, "FULL_MEMBER") {
				continue
			}
			seen[id] = true
			accountIDs = append(accountIDs, id)
		}
		if payload.Cursor == nil || strings.TrimSpace(*payload.Cursor) == "" || len(payload.Entities) == 0 {
			break
		}
		cursor = strings.TrimSpace(*payload.Cursor)
	}
	return accountIDs, nil
}

type jiraUser struct {
	AccountID    string            `json:"accountId"`
	DisplayName  string            `json:"displayName"`
	EmailAddress string            `json:"emailAddress"`
	Active       bool              `json:"active"`
	AccountType  string            `json:"accountType"`
	AvatarUrls   map[string]string `json:"avatarUrls"`
}

// member converts a Jira user into a team member. App accounts are dropped: a
// bot is a member of nothing a human would assign work to.
func (u jiraUser) member() (models.TeamMember, bool) {
	if strings.TrimSpace(u.AccountID) == "" {
		return models.TeamMember{}, false
	}
	if u.AccountType != "" && !strings.EqualFold(u.AccountType, "atlassian") {
		return models.TeamMember{}, false
	}
	avatar := u.AvatarUrls["48x48"]
	if avatar == "" {
		avatar = u.AvatarUrls["32x32"]
	}
	return models.TeamMember{
		AccountID:   u.AccountID,
		DisplayName: strings.TrimSpace(u.DisplayName),
		Email:       strings.TrimSpace(u.EmailAddress),
		AvatarURL:   avatar,
		Active:      u.Active,
	}, true
}

// jiraLookupUsers resolves account ids into people. Deactivated accounts are
// kept and flagged: a ticket still shows the name of whoever owned it.
func (c *Client) jiraLookupUsers(ctx context.Context, accountIDs []string) ([]models.TeamMember, error) {
	var members []models.TeamMember
	for start := 0; start < len(accountIDs); start += jiraUserBulkBatch {
		end := start + jiraUserBulkBatch
		if end > len(accountIDs) {
			end = len(accountIDs)
		}
		query := url.Values{}
		for _, id := range accountIDs[start:end] {
			query.Add("accountId", id)
		}
		query.Set("maxResults", fmt.Sprint(jiraUserBulkBatch))
		var payload struct {
			Values []jiraUser `json:"values"`
		}
		if err := c.jira(ctx, http.MethodGet, "/rest/api/3/user/bulk", query, nil, &payload); err != nil {
			return members, err
		}
		for _, u := range payload.Values {
			if m, ok := u.member(); ok {
				members = append(members, m)
			}
		}
	}
	return members, nil
}

// jiraTeamMembers reads the people of one team, ready to be stored.
func (c *Client) jiraTeamMembers(ctx context.Context, teamID string) ([]models.TeamMember, error) {
	accountIDs, err := c.jiraTeamMemberAccountIDs(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if len(accountIDs) == 0 {
		return []models.TeamMember{}, nil
	}
	members, err := c.jiraLookupUsers(ctx, accountIDs)
	if err != nil {
		return members, err
	}
	for i := range members {
		members[i].TeamID = teamID
	}
	return members, nil
}

// jiraSearchTeams looks the site's teams up by name. The teams service has no
// search reachable with a user token, but JQL autocompletion does the job and
// answers both the label and the id, which is what writing the field needs.
func (c *Client) jiraSearchTeams(ctx context.Context, query string) ([]models.TrackerTeam, error) {
	params := url.Values{}
	params.Set("fieldName", "Team")
	params.Set("fieldValue", strings.TrimSpace(query))
	var payload struct {
		Results []struct {
			Value       string `json:"value"`
			DisplayName string `json:"displayName"`
		} `json:"results"`
	}
	if err := c.jira(ctx, http.MethodGet, "/rest/api/3/jql/autocompletedata/suggestions", params, nil, &payload); err != nil {
		return nil, err
	}
	out := make([]models.TrackerTeam, 0, len(payload.Results))
	for _, r := range payload.Results {
		id := strings.TrimSpace(r.Value)
		// Autocompletion wraps the matched part in <b> markers.
		name := strings.TrimSpace(strings.NewReplacer("<b>", "", "</b>", "").Replace(r.DisplayName))
		if id == "" || name == "" {
			continue
		}
		out = append(out, models.TrackerTeam{ID: id, Name: name})
	}
	return out, nil
}

// jiraSetTeam writes the Team field, or clears it when teamID is empty. Sites
// differ on whether the field wants the bare id or an object carrying it, so
// the object form is tried when the bare one is refused.
func (c *Client) jiraSetTeam(ctx context.Context, key, teamID string) error {
	fields, err := c.jiraFields(ctx)
	if err != nil {
		return err
	}
	if fields.Team == "" {
		return fmt.Errorf("this Jira site has no Team field")
	}
	teamID = strings.TrimSpace(teamID)
	values := []any{nil}
	if teamID != "" {
		values = []any{teamID, map[string]string{"id": teamID}}
	}
	var lastErr error
	for _, value := range values {
		payload := map[string]any{"fields": map[string]any{fields.Team: value}}
		if err := c.jira(ctx, http.MethodPut, "/rest/api/3/issue/"+url.PathEscape(key), nil, payload, nil); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

// jiraAssignable looks up who a work item can be assigned to, through the very
// endpoint Jira's own edit screen uses, so the answer is exactly who the site
// accepts on that ticket, permissions included.
func (c *Client) jiraAssignable(ctx context.Context, key, query string, limit int) ([]models.TeamMember, error) {
	if limit <= 0 {
		limit = 20
	}
	params := url.Values{}
	params.Set("issueKey", key)
	params.Set("query", strings.TrimSpace(query))
	params.Set("maxResults", fmt.Sprint(limit))
	var users []jiraUser
	if err := c.jira(ctx, http.MethodGet, "/rest/api/3/user/assignable/search", params, nil, &users); err != nil {
		return nil, err
	}
	out := make([]models.TeamMember, 0, len(users))
	for _, u := range users {
		if m, ok := u.member(); ok {
			out = append(out, m)
		}
	}
	return out, nil
}
