package trackerapi

import (
	"fmt"
	"strings"
	"tasks/internal/models"
	"time"
)

const linearIssueFields = `id identifier title description url priority createdAt updatedAt state{id name type color} assignee{id name displayName avatarUrl} labels{nodes{id name}}`

type pageInfo struct {
	HasNextPage bool
	EndCursor   string
}

func (c *Client) SyncFromLinear(teamKey string) ([]models.Task, error) {
	var nodes []LinearIssueNode
	var cursor any
	seen := map[string]bool{}
	for {
		var res struct {
			Issues struct {
				Nodes    []LinearIssueNode
				PageInfo pageInfo
			}
		}
		filter := map[string]any{}
		if teamKey != "" {
			filter["team"] = map[string]any{"key": map[string]any{"eq": teamKey}}
		}
		err := c.linear(`query($after:String,$filter:IssueFilter){issues(first:100,after:$after,filter:$filter){nodes{`+linearIssueFields+`} pageInfo{hasNextPage endCursor}}}`, map[string]any{"after": cursor, "filter": filter}, &res)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, res.Issues.Nodes...)
		p := res.Issues.PageInfo
		if !p.HasNextPage {
			break
		}
		if p.EndCursor == "" || seen[p.EndCursor] {
			return nil, fmt.Errorf("invalid Linear pagination cursor")
		}
		seen[p.EndCursor] = true
		cursor = p.EndCursor
	}
	return linearTasks(LinearQueryResponse{Nodes: nodes})
}

type linearTeam struct {
	ID   string
	Key  string
	Name string
}

func (c *Client) teams() ([]linearTeam, error) {
	var teams []linearTeam
	var after any
	seen := map[string]bool{}
	for {
		var res struct {
			Teams struct {
				Nodes    []linearTeam
				PageInfo pageInfo
			}
		}
		if err := c.linear(`query($after:String){teams(first:100,after:$after){nodes{id key name} pageInfo{hasNextPage endCursor}}}`, map[string]any{"after": after}, &res); err != nil {
			return nil, err
		}
		teams = append(teams, res.Teams.Nodes...)
		p := res.Teams.PageInfo
		if !p.HasNextPage {
			return teams, nil
		}
		if p.EndCursor == "" || seen[p.EndCursor] {
			return nil, fmt.Errorf("invalid Linear teams cursor")
		}
		seen[p.EndCursor] = true
		after = p.EndCursor
	}
}
func (c *Client) teamID(key string) (string, error) {
	teams, err := c.teams()
	if err != nil {
		return "", err
	}
	for _, team := range teams {
		if strings.EqualFold(team.Key, key) || team.ID == key {
			return team.ID, nil
		}
	}
	if key == "" && len(teams) == 1 {
		return teams[0].ID, nil
	}
	return "", fmt.Errorf("configure an explicit Linear team key")
}
func (c *Client) issue(key string) (LinearIssueNode, string, error) {
	var res struct {
		Issue struct {
			LinearIssueNode
			Team struct{ ID string }
		}
	}
	err := c.linear(`query($id:String!){issue(id:$id){`+linearIssueFields+` team{id}}}`, map[string]any{"id": key}, &res)
	if err != nil {
		return LinearIssueNode{}, "", err
	}
	if res.Issue.ID == "" {
		return LinearIssueNode{}, "", fmt.Errorf("Linear issue not found")
	}
	return res.Issue.LinearIssueNode, res.Issue.Team.ID, nil
}
func linearPriority(p models.Priority) int {
	switch p {
	case models.PriorityUrgent:
		return 1
	case models.PriorityHigh:
		return 2
	case models.PriorityMedium:
		return 3
	case models.PriorityLow:
		return 4
	}
	return 0
}
func (c *Client) labelIDs(team string, names []string) ([]string, error) {
	type label struct {
		ID, Name string
		Team     *struct{ ID string }
	}
	var labels []label
	var after any
	seen := map[string]bool{}
	for {
		var res struct {
			IssueLabels struct {
				Nodes    []label
				PageInfo pageInfo
			}
		}
		err := c.linear(`query($after:String){issueLabels(first:100,after:$after){nodes{id name team{id}} pageInfo{hasNextPage endCursor}}}`, map[string]any{"after": after}, &res)
		if err != nil {
			return nil, err
		}
		labels = append(labels, res.IssueLabels.Nodes...)
		p := res.IssueLabels.PageInfo
		if !p.HasNextPage {
			break
		}
		if p.EndCursor == "" || seen[p.EndCursor] {
			return nil, fmt.Errorf("invalid Linear labels cursor")
		}
		seen[p.EndCursor] = true
		after = p.EndCursor
	}
	ids := []string{}
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			continue
		}
		id := ""
		for _, l := range labels {
			if (l.Team == nil || l.Team.ID == team) && strings.EqualFold(strings.TrimPrefix(l.Name, "#"), strings.TrimPrefix(name, "#")) {
				id = l.ID
				if l.Team != nil {
					break
				}
			}
		}
		if id == "" {
			var res struct {
				IssueLabelCreate struct {
					Success    bool
					IssueLabel struct{ ID string }
				}
			}
			err := c.linear(`mutation($input:IssueLabelCreateInput!){issueLabelCreate(input:$input){success issueLabel{id}}}`, map[string]any{"input": map[string]any{"name": name, "teamId": team}}, &res)
			if err != nil {
				return nil, err
			}
			if !res.IssueLabelCreate.Success || res.IssueLabelCreate.IssueLabel.ID == "" {
				return nil, fmt.Errorf("Linear did not confirm label creation")
			}
			id = res.IssueLabelCreate.IssueLabel.ID
		}
		ids = append(ids, id)
	}
	return ids, nil
}

type LinearState struct {
	ID    string
	Name  string
	Type  string
	Color string
}

func (c *Client) LinearStates(team string) ([]LinearState, error) {
	id, err := c.teamID(team)
	if err != nil {
		return nil, err
	}
	return c.statesByID(id)
}
func (c *Client) statesByID(team string) ([]LinearState, error) {
	var states []LinearState
	var after any
	seen := map[string]bool{}
	for {
		var res struct {
			WorkflowStates struct {
				Nodes    []LinearState
				PageInfo pageInfo
			}
		}
		err := c.linear(`query($team:ID!,$after:String){workflowStates(first:100,after:$after,filter:{team:{id:{eq:$team}}}){nodes{id name type color} pageInfo{hasNextPage endCursor}}}`, map[string]any{"team": team, "after": after}, &res)
		if err != nil {
			return nil, err
		}
		states = append(states, res.WorkflowStates.Nodes...)
		p := res.WorkflowStates.PageInfo
		if !p.HasNextPage {
			return states, nil
		}
		if p.EndCursor == "" || seen[p.EndCursor] {
			return nil, fmt.Errorf("invalid Linear states cursor")
		}
		seen[p.EndCursor] = true
		after = p.EndCursor
	}
}
func (c *Client) stateID(team string, status models.Status) (string, error) {
	states, err := c.statesByID(team)
	if err != nil {
		return "", err
	}
	name := mapStatusToLinearState(status)
	for _, s := range states {
		if strings.EqualFold(s.Name, string(status)) || strings.EqualFold(s.Name, name) {
			return s.ID, nil
		}
	}
	return "", fmt.Errorf("Linear state %q is not configured for this team", name)
}
func (c *Client) CreateLinearIssue(teamKey, title, description string, priority models.Priority, labels []string) (*models.Task, error) {
	team, err := c.teamID(teamKey)
	if err != nil {
		return nil, err
	}
	ids, err := c.labelIDs(team, labels)
	if err != nil {
		return nil, err
	}
	var res struct {
		IssueCreate struct {
			Success bool
			Issue   LinearIssueNode
		}
	}
	err = c.linear(`mutation($input:IssueCreateInput!){issueCreate(input:$input){success issue{`+linearIssueFields+`}}}`, map[string]any{"input": map[string]any{"teamId": team, "title": title, "description": description, "priority": linearPriority(priority), "labelIds": ids}}, &res)
	if err != nil {
		return nil, err
	}
	if !res.IssueCreate.Success || res.IssueCreate.Issue.ID == "" {
		return nil, fmt.Errorf("Linear did not confirm issue creation")
	}
	tasks, err := linearTasks(LinearQueryResponse{Nodes: []LinearIssueNode{res.IssueCreate.Issue}})
	if err != nil {
		return nil, err
	}
	return &tasks[0], nil
}
func (c *Client) UpdateLinearIssueState(key string, status models.Status) error {
	return c.UpdateLinearIssue(key, nil, nil, nil, &status, nil)
}
func (c *Client) UpdateLinearIssue(key string, title, description *string, priority *models.Priority, status *models.Status, labels []string) error {
	issue, team, err := c.issue(key)
	if err != nil {
		return err
	}
	input := map[string]any{}
	if title != nil {
		input["title"] = *title
	}
	if description != nil {
		input["description"] = *description
	}
	if priority != nil {
		input["priority"] = linearPriority(*priority)
	}
	if status != nil {
		id, err := c.stateID(team, *status)
		if err != nil {
			return err
		}
		input["stateId"] = id
	}
	if labels != nil {
		ids, err := c.labelIDs(team, labels)
		if err != nil {
			return err
		}
		input["labelIds"] = ids
	}
	if len(input) == 0 {
		return nil
	}
	var res struct{ IssueUpdate struct{ Success bool } }
	err = c.linear(`mutation($id:String!,$input:IssueUpdateInput!){issueUpdate(id:$id,input:$input){success}}`, map[string]any{"id": issue.ID, "input": input}, &res)
	if err == nil && !res.IssueUpdate.Success {
		err = fmt.Errorf("Linear did not confirm issue update")
	}
	return err
}
func (c *Client) addLinearComment(key, body string) error {
	issue, _, err := c.issue(key)
	if err != nil {
		return err
	}
	var res struct{ CommentCreate struct{ Success bool } }
	err = c.linear(`mutation($input:CommentCreateInput!){commentCreate(input:$input){success}}`, map[string]any{"input": map[string]any{"issueId": issue.ID, "body": body}}, &res)
	if err == nil && !res.CommentCreate.Success {
		err = fmt.Errorf("Linear did not confirm comment creation")
	}
	return err
}
func (c *Client) GetLinearIssueComments(repoPath, key string) ([]models.TaskComment, error) {
	var comments []models.TaskComment
	var after any
	seen := map[string]bool{}
	for {
		var res struct {
			Issue struct {
				Comments struct {
					Nodes []struct {
						ID        string
						Body      string
						CreatedAt time.Time
						User      struct {
							Name        string
							DisplayName string
						}
					}
					PageInfo pageInfo
				}
			}
		}
		err := c.linear(`query($id:String!,$after:String){issue(id:$id){comments(first:100,after:$after){nodes{id body createdAt user{name displayName}} pageInfo{hasNextPage endCursor}}}}`, map[string]any{"id": key, "after": after}, &res)
		if err != nil {
			return nil, err
		}
		for _, n := range res.Issue.Comments.Nodes {
			author := n.User.DisplayName
			if author == "" {
				author = n.User.Name
			}
			comments = append(comments, models.TaskComment{ID: n.ID, Author: author, Body: n.Body, CreatedAt: &n.CreatedAt, Source: "linear"})
		}
		p := res.Issue.Comments.PageInfo
		if !p.HasNextPage {
			return comments, nil
		}
		if p.EndCursor == "" || seen[p.EndCursor] {
			return nil, fmt.Errorf("invalid Linear comments cursor")
		}
		seen[p.EndCursor] = true
		after = p.EndCursor
	}
}
