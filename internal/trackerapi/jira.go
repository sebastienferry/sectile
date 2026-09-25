package trackerapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"tasks/internal/models"
	"tasks/internal/tracker"
)

// JiraAdapter drives a Jira Cloud site through tracker.TicketingSystem: REST v3
// for work items, Agile 1.0 for boards and sprints, the site gateway for team
// members. It is a port of the client removed in e6e16ec, minus the acli
// fallbacks the server may not run, on top of the shared Client and its
// per-project credentials.
type JiraAdapter struct {
	tracker.BaseTicketingSystem
	client *Client
}

// NewJiraAdapter creates the TicketingSystem adapter for Jira Cloud.
func NewJiraAdapter(client *Client) *JiraAdapter {
	return &JiraAdapter{
		BaseTicketingSystem: tracker.BaseTicketingSystem{
			TrackerName: "jira",
			Capabilities: []tracker.Capability{
				tracker.CapCreate, tracker.CapUpdate, tracker.CapDelete, tracker.CapSync,
				tracker.CapGet, tracker.CapComment, tracker.CapLabels, tracker.CapAssign,
				tracker.CapTransition, tracker.CapSprint, tracker.CapTeam, tracker.CapEpic,
				tracker.CapBoard, tracker.CapIncrementalSync, tracker.CapSprintManage,
			},
		},
		client: client,
	}
}

// forProject is the client carrying the credentials of one call: the project's,
// or the acting user's own when the context names one and they stored a token.
// A Jira write is attributed to the account its token belongs to, which is why
// this is resolved per call rather than once per project.
func (j *JiraAdapter) forProject(ctx context.Context, p *models.Project) (*Client, error) {
	projectID := ""
	if p != nil {
		projectID = p.ID
	}
	// Half the write side takes a key and nothing else, so those calls pass no
	// project and read it from the context instead. Resolving with no project at
	// all ignored the project's own site: the sync reached one instance and
	// every sprint move, team write and assignment reached another.
	if projectID == "" {
		projectID = tracker.Project(ctx)
	}
	user := tracker.ActingUser(ctx)
	client, personal, err := j.client.ForActingUser(user, "jira", projectID)
	if err != nil {
		return nil, err
	}
	// Jira attributes a write to the account behind the token. So an operation
	// somebody asked for either carries their own token or does not happen:
	// writing under the server account would put a name on it that nobody
	// chose. Unattended work names nobody and keeps the server credential.
	if user != "" && !personal {
		return nil, fmt.Errorf("no personal Jira token for this user: store one in your profile, or the work would be attributed to the server account")
	}
	return client, nil
}

func (j *JiraAdapter) projectKey(p *models.Project) (string, error) {
	key := jiraProjectKey(p)
	if key == "" {
		return "", fmt.Errorf("configure the Jira project key on the project")
	}
	return key, nil
}

// FormatTaskID gives a work item its local identity: jira-<projectID>-<KEY>.
func (j *JiraAdapter) FormatTaskID(projectID string, key string, rawID string) string {
	clean, err := cleanJiraKey(key)
	if err != nil && rawID != "" {
		clean, err = cleanJiraKey(rawID)
	}
	if err != nil {
		clean = strings.ToUpper(strings.TrimSpace(key))
	}
	if projectID != "" && projectID != "default" {
		return fmt.Sprintf("jira-%s-%s", projectID, clean)
	}
	return "jira-" + clean
}

// fieldsFor is the field list of one read, custom ids included. A site that
// cannot be asked which fields it has is an error rather than a site with none:
// swallowing it imported every work item with an empty sprint and an empty
// team, reported the sync as a success, and left nobody any way to tell that
// from a site that genuinely has neither field.
func (j *JiraAdapter) fieldsFor(ctx context.Context, c *Client) ([]string, jiraFieldIDs, error) {
	ids, err := c.jiraFields(ctx)
	if err != nil {
		return nil, jiraFieldIDs{}, fmt.Errorf("jira did not say which fields it has: %w", err)
	}
	fields := append([]string{}, jiraBaseFields...)
	if ids.Sprint != "" {
		fields = append(fields, ids.Sprint)
	}
	if ids.Team != "" {
		fields = append(fields, ids.Team)
	}
	return fields, ids, nil
}

func (j *JiraAdapter) search(ctx context.Context, c *Client, jql string) ([]models.Task, error) {
	fields, ids, err := j.fieldsFor(ctx, c)
	if err != nil {
		return nil, err
	}
	pages, err := c.jiraSearchPages(ctx, jql, fields)
	if err != nil {
		return nil, err
	}
	// Asked once for the whole page, not once per work item: the scheme is a
	// property of the site, and it is cached across calls anyway.
	priorities := c.jiraPriorities(ctx)
	tasks := make([]models.Task, 0, len(pages))
	unreadable := 0
	for _, raw := range pages {
		issue, err := decodeJiraIssue(raw)
		if err != nil {
			// One work item of an unexpected shape is not a reason to import
			// none of the other two thousand. It is counted, so the import does
			// not silently come back short.
			unreadable++
			continue
		}
		task := jiraTask(c.JiraURL, issue, ids, priorities)
		task.Position = len(tasks)
		tasks = append(tasks, *task)
	}
	if unreadable > 0 && len(tasks) == 0 {
		return nil, fmt.Errorf("jira returned %d unreadable work items and nothing else", unreadable)
	}
	if unreadable > 0 {
		log.Printf("[jira] %d work item(s) skipped: unreadable shape", unreadable)
	}
	return tasks, nil
}

func (j *JiraAdapter) SyncIssues(ctx context.Context, req tracker.SyncRequest) ([]models.Task, error) {
	key, err := j.projectKey(req.Project)
	if err != nil {
		return nil, err
	}
	var types []string
	if req.Project != nil {
		types = models.NormalizeIssueTypes(req.Project.IssueTypes)
	}
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	// An incremental read is the same search with one more clause. It comes
	// back ordered by `updated` like the full one, so a window that turns out
	// to hold more work items than a page is paginated the same way.
	return j.search(ctx, c, jiraJQL(key, types, jiraUpdatedWithin(req.UpdatedWithinMin)))
}

func (j *JiraAdapter) GetIssue(ctx context.Context, req tracker.GetIssueRequest) (*models.Task, error) {
	key, err := cleanJiraKey(req.Key)
	if err != nil {
		return nil, err
	}
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	fields, ids, err := j.fieldsFor(ctx, c)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("fields", strings.Join(fields, ","))
	var raw json.RawMessage
	if err := c.jira(ctx, http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(key), query, nil, &raw); err != nil {
		return nil, err
	}
	issue, err := decodeJiraIssue(raw)
	if err != nil {
		return nil, err
	}
	return jiraTask(c.JiraURL, issue, ids, c.jiraPriorities(ctx)), nil
}

func (j *JiraAdapter) CreateIssue(ctx context.Context, req tracker.CreateIssueRequest) (*models.Task, error) {
	projectKey, err := j.projectKey(req.Project)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, fmt.Errorf("issue title is required")
	}
	issueType := strings.TrimSpace(req.IssueType)
	if issueType == "" && req.Project != nil {
		if types := models.NormalizeIssueTypes(req.Project.IssueTypes); len(types) > 0 {
			issueType = types[0]
		}
	}
	if issueType == "" {
		issueType = "Task"
	}
	fields := map[string]any{
		"project":   map[string]string{"key": projectKey},
		"issuetype": map[string]string{"name": issueType},
		"summary":   title,
	}
	if strings.TrimSpace(req.Description) != "" {
		fields["description"] = MarkdownToADF(req.Description)
	}
	if labels := cleanLabels(req.Labels); len(labels) > 0 {
		fields["labels"] = labels
	}
	if parent := strings.TrimSpace(req.ParentKey); parent != "" {
		fields["parent"] = map[string]string{"key": strings.ToUpper(parent)}
	}
	// Fields the site makes mandatory. A select list takes an option id, a free
	// text field takes the text: sending every one of them as {"id": …} made
	// creation impossible on any project with a mandatory text or number field,
	// with Jira answering that the value must be a string. Which is which comes
	// from the site itself, and is only asked for when there is something to
	// ask about.
	if len(req.Fields) > 0 {
		options, err := j.optionFields(ctx, req.Project, issueType)
		if err != nil {
			return nil, err
		}
		for id, value := range req.Fields {
			id, value = strings.TrimSpace(id), strings.TrimSpace(value)
			if id == "" || value == "" {
				continue
			}
			if options[id] {
				fields[id] = map[string]string{"id": value}
			} else {
				fields[id] = value
			}
		}
	}
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	// The creation screen decides: which options this project's scheme has,
	// and whether it carries the field at all. A project whose screen has no
	// priority is created without one rather than refused over it, and the
	// level is put on afterwards.
	priorityCarried := false
	if req.Priority != "" {
		screen, readable := c.jiraCreatePriorities(ctx, projectKey, issueType)
		if value, ok := c.priorityFieldFor(ctx, screen, readable, req.Priority, projectKey+"/"+issueType); ok {
			fields["priority"] = value
			priorityCarried = true
		}
	}
	var created struct {
		Key string `json:"key"`
	}
	if err := c.jira(ctx, http.MethodPost, "/rest/api/3/issue", nil, map[string]any{"fields": fields}, &created); err != nil {
		// Jira names a missing mandatory field by id, and only the first ones
		// it meets. The creation screen is read after a refusal only, so a
		// creation that succeeds costs no extra request.
		var httpErr *HTTPError
		if errors.As(err, &httpErr) && httpErr.Status == http.StatusBadRequest {
			if missing := j.missingRequiredFields(ctx, req.Project, issueType, fields); missing != "" {
				return nil, fmt.Errorf("%w; fields this project requires on creation for %s: %s", err, issueType, missing)
			}
		}
		return nil, err
	}
	if strings.TrimSpace(created.Key) == "" {
		return nil, fmt.Errorf("tracker did not confirm a created issue")
	}
	if assignee := strings.TrimSpace(req.Assignee); assignee != "" {
		if err := j.assign(ctx, c, created.Key, assignee); err != nil {
			return nil, err
		}
	}
	if req.Priority != "" && !priorityCarried {
		j.setPriorityAfterCreate(ctx, c, created.Key, req.Priority)
	}
	task, err := j.GetIssue(ctx, tracker.GetIssueRequest{Project: req.Project, Key: created.Key})
	if err != nil {
		// The work item exists: answer with what is known rather than failing
		// a creation the site confirmed.
		key := strings.ToUpper(created.Key)
		u := jiraBrowseURL(c.JiraURL, key)
		return &models.Task{ID: "jira-" + key, Key: key, Title: title, Description: req.Description, Status: models.StatusToClarify, Priority: models.PriorityMedium, Labels: cleanLabels(req.Labels), Source: "jira", IssueType: issueType, ExternalURL: &u}, nil
	}
	return task, nil
}

// setPriorityAfterCreate puts the level on a work item whose creation screen
// would not carry it. Such projects exist — their creation screen has no
// priority field while their edit screen does — and one extra request beats
// dropping the level the caller asked for.
//
// A refusal here is logged and not returned: the work item exists, and failing
// its creation over a field the site would not take on the way in is exactly
// what this whole path avoids. The read that follows answers with the priority
// the site actually holds, so nothing claims a level that did not stick.
func (j *JiraAdapter) setPriorityAfterCreate(ctx context.Context, c *Client, key string, p models.Priority) {
	screen, readable := c.jiraEditPriorities(ctx, key)
	value, ok := c.priorityFieldFor(ctx, screen, readable, p, key)
	if !ok {
		return
	}
	payload := map[string]any{"fields": map[string]any{"priority": value}}
	if err := c.jira(ctx, http.MethodPut, "/rest/api/3/issue/"+url.PathEscape(key), nil, payload, nil); err != nil {
		log.Printf("[jira] %s was created, but its priority could not be set: %v", key, err)
	}
}

func cleanLabels(labels []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, l := range labels {
		// Jira refuses spaces in labels; the workflow labels never carry any.
		l = strings.ReplaceAll(strings.TrimSpace(l), " ", "-")
		if l == "" || seen[l] {
			continue
		}
		seen[l] = true
		out = append(out, l)
	}
	return out
}

func (j *JiraAdapter) UpdateIssue(ctx context.Context, req tracker.UpdateIssueRequest) error {
	key := req.Key
	if key == "" && req.Task != nil {
		key = req.Task.Key
	}
	key, err := cleanJiraKey(key)
	if err != nil {
		return err
	}
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return err
	}

	fields := map[string]any{}
	if req.Title != nil {
		fields["summary"] = *req.Title
	}
	if req.Description != nil {
		fields["description"] = MarkdownToADF(*req.Description)
	}
	if req.Priority != nil && *req.Priority != "" {
		screen, readable := c.jiraEditPriorities(ctx, key)
		if value, ok := c.priorityFieldFor(ctx, screen, readable, *req.Priority, key); ok {
			fields["priority"] = value
		}
	}
	update := map[string]any{}
	if ops := labelOps(req.Labels, req.RemovedLabels); len(ops) > 0 {
		update["labels"] = ops
	}
	payload := map[string]any{}
	if len(fields) > 0 {
		payload["fields"] = fields
	}
	if len(update) > 0 {
		payload["update"] = update
	}
	if len(payload) > 0 {
		if err := c.jira(ctx, http.MethodPut, "/rest/api/3/issue/"+url.PathEscape(key), nil, payload, nil); err != nil {
			return err
		}
	}
	if req.Assignee != nil {
		if err := j.assign(ctx, c, key, *req.Assignee); err != nil {
			return err
		}
	}
	// The status: a named target first, else the finished rule. A stage change
	// that only moves a label reaches neither and leaves the workflow alone.
	if target := strings.TrimSpace(req.TargetStatus); target != "" {
		return c.jiraTransitionTo(ctx, key, target)
	}
	if closed, explicit := isFinishedStatus(req.Status, req.Labels); explicit && closed {
		return c.jiraTransitionDone(ctx, key)
	}
	return nil
}

// labelOps expresses a label change as Jira update operations: the labels
// wanted are added, the ones removed are removed, and the site keeps the rest.
// A label in both lists is removed: the caller's removal is the newer intent.
func labelOps(add, remove []string) []map[string]string {
	dropped := map[string]bool{}
	for _, l := range cleanLabels(remove) {
		dropped[l] = true
	}
	var ops []map[string]string
	for _, l := range cleanLabels(add) {
		if !dropped[l] {
			ops = append(ops, map[string]string{"add": l})
		}
	}
	for l := range dropped {
		ops = append(ops, map[string]string{"remove": l})
	}
	sort.Slice(ops, func(a, b int) bool {
		ka, kb := "", ""
		for k := range ops[a] {
			ka = k + ops[a][k]
		}
		for k := range ops[b] {
			kb = k + ops[b][k]
		}
		return ka < kb
	})
	return ops
}

func (j *JiraAdapter) DeleteIssue(ctx context.Context, req tracker.DeleteIssueRequest) error {
	key, err := cleanJiraKey(req.Key)
	if err != nil {
		return err
	}
	// A Jira deletion is destructive and irreversible; closing is what the
	// board means by removing a card, whether or not CloseOnly is set.
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return err
	}
	return c.jiraTransitionDone(ctx, key)
}

// jiraTransition is one workflow transition available from the current status.
type jiraTransition struct {
	ID       string
	Name     string
	ToName   string
	Category string
}

func (c *Client) jiraTransitions(ctx context.Context, key string) ([]jiraTransition, error) {
	var payload struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				Name           string `json:"name"`
				StatusCategory struct {
					Key string `json:"key"`
				} `json:"statusCategory"`
			} `json:"to"`
		} `json:"transitions"`
	}
	if err := c.jira(ctx, http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(key)+"/transitions", nil, nil, &payload); err != nil {
		return nil, err
	}
	out := make([]jiraTransition, 0, len(payload.Transitions))
	for _, tr := range payload.Transitions {
		out = append(out, jiraTransition{ID: tr.ID, Name: tr.Name, ToName: tr.To.Name, Category: tr.To.StatusCategory.Key})
	}
	return out, nil
}

// jiraStatusOf reads the status a work item is in, and nothing else.
func (c *Client) jiraStatusOf(ctx context.Context, key string) (string, error) {
	var payload struct {
		Fields struct {
			Status struct {
				Name string `json:"name"`
			} `json:"status"`
		} `json:"fields"`
	}
	query := url.Values{"fields": []string{"status"}}
	if err := c.jira(ctx, http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(key), query, nil, &payload); err != nil {
		return "", err
	}
	return payload.Fields.Status.Name, nil
}

func (c *Client) jiraRunTransition(ctx context.Context, key, transitionID string) error {
	return c.jira(ctx, http.MethodPost, "/rest/api/3/issue/"+url.PathEscape(key)+"/transitions", nil,
		map[string]any{"transition": map[string]string{"id": transitionID}}, nil)
}

func transitionNames(transitions []jiraTransition) string {
	names := make([]string, 0, len(transitions))
	for _, tr := range transitions {
		names = append(names, tr.ToName)
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// jiraTransitionTo moves a work item to the status named as the site names it.
// The transition's own name ("Close Issue") is not the status it leads to
// ("Closed"): the target status is what is matched.
func (c *Client) jiraTransitionTo(ctx context.Context, key, statusName string) error {
	transitions, err := c.jiraTransitions(ctx, key)
	if err != nil {
		return err
	}
	for _, tr := range transitions {
		if strings.EqualFold(strings.TrimSpace(tr.ToName), strings.TrimSpace(statusName)) {
			return c.jiraRunTransition(ctx, key, tr.ID)
		}
	}
	// Already there is done, not failed. A stage change transitions the work
	// item and then updates its fields, and the update transitions again: the
	// second call found no self-transition and reported the whole stage change
	// as failed, after every field had been written.
	if current, err := c.jiraStatusOf(ctx, key); err == nil && strings.EqualFold(strings.TrimSpace(current), strings.TrimSpace(statusName)) {
		return nil
	}
	return fmt.Errorf("no transition of %s leads to %q from its current status (available: %s)", key, statusName, transitionNames(transitions))
}

// jiraTransitionDone runs the first transition whose target status is of the
// done category. A work item already done has no such transition and is left
// as it is.
func (c *Client) jiraTransitionDone(ctx context.Context, key string) error {
	transitions, err := c.jiraTransitions(ctx, key)
	if err != nil {
		return err
	}
	for _, tr := range transitions {
		if strings.EqualFold(tr.Category, "done") {
			return c.jiraRunTransition(ctx, key, tr.ID)
		}
	}
	var current struct {
		Fields struct {
			Status struct {
				StatusCategory struct {
					Key string `json:"key"`
				} `json:"statusCategory"`
			} `json:"status"`
		} `json:"fields"`
	}
	query := url.Values{}
	query.Set("fields", "status")
	if err := c.jira(ctx, http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(key), query, nil, &current); err == nil && strings.EqualFold(current.Fields.Status.StatusCategory.Key, "done") {
		return nil
	}
	return fmt.Errorf("no transition of %s leads to a done status (available: %s)", key, transitionNames(transitions))
}

func (j *JiraAdapter) AddComment(ctx context.Context, req tracker.AddCommentRequest) error {
	if strings.TrimSpace(req.Body) == "" {
		return fmt.Errorf("comment body is required")
	}
	key, err := cleanJiraKey(req.Key)
	if err != nil {
		return err
	}
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return err
	}
	return c.jira(ctx, http.MethodPost, "/rest/api/3/issue/"+url.PathEscape(key)+"/comment", nil,
		map[string]any{"body": MarkdownToADF(req.Body)}, nil)
}

func (j *JiraAdapter) GetComments(ctx context.Context, req tracker.GetCommentsRequest) ([]models.TaskComment, error) {
	key, err := cleanJiraKey(req.Key)
	if err != nil {
		return nil, err
	}
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	comments := []models.TaskComment{}
	startAt := 0
	for page := 0; page < jiraMaxPages; page++ {
		query := url.Values{}
		query.Set("orderBy", "created")
		query.Set("startAt", fmt.Sprint(startAt))
		query.Set("maxResults", fmt.Sprint(jiraPageSize))
		var payload struct {
			Comments []struct {
				ID     string `json:"id"`
				Author struct {
					DisplayName string `json:"displayName"`
				} `json:"author"`
				Body    json.RawMessage `json:"body"`
				Created string          `json:"created"`
			} `json:"comments"`
			Total int `json:"total"`
		}
		if err := c.jira(ctx, http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(key)+"/comment", query, nil, &payload); err != nil {
			return nil, err
		}
		for _, cm := range payload.Comments {
			comments = append(comments, models.TaskComment{
				ID:        cm.ID,
				Author:    cm.Author.DisplayName,
				Body:      ADFToMarkdown(cm.Body),
				CreatedAt: parseJiraTime(cm.Created),
				Source:    "jira",
			})
		}
		startAt += len(payload.Comments)
		if len(payload.Comments) == 0 || startAt >= payload.Total {
			break
		}
	}
	return comments, nil
}

// Writer side.

func (j *JiraAdapter) assign(ctx context.Context, c *Client, key, accountID string) error {
	var payload map[string]any
	if id := strings.TrimSpace(accountID); id != "" {
		payload = map[string]any{"accountId": id}
	} else {
		// null unassigns; the empty string is refused.
		payload = map[string]any{"accountId": nil}
	}
	return c.jira(ctx, http.MethodPut, "/rest/api/3/issue/"+url.PathEscape(key)+"/assignee", nil, payload, nil)
}

func (j *JiraAdapter) Assign(ctx context.Context, key string, personID string) error {
	clean, err := cleanJiraKey(key)
	if err != nil {
		return err
	}
	c, err := j.forProject(ctx, nil)
	if err != nil {
		return err
	}
	return j.assign(ctx, c, clean, personID)
}

func (j *JiraAdapter) Transition(ctx context.Context, key string, status string) error {
	clean, err := cleanJiraKey(key)
	if err != nil {
		return err
	}
	c, err := j.forProject(ctx, nil)
	if err != nil {
		return err
	}
	return c.jiraTransitionTo(ctx, clean, status)
}

func (j *JiraAdapter) SetSprint(ctx context.Context, sprintID string, keys []string) error {
	clean := make([]string, 0, len(keys))
	for _, k := range keys {
		if c, err := cleanJiraKey(k); err == nil {
			clean = append(clean, c)
		}
	}
	if len(clean) == 0 {
		return fmt.Errorf("no work item to move")
	}
	path := "/rest/agile/1.0/backlog/issue"
	if id := strings.TrimSpace(sprintID); id != "" {
		path = "/rest/agile/1.0/sprint/" + url.PathEscape(id) + "/issue"
	}
	c, err := j.forProject(ctx, nil)
	if err != nil {
		return err
	}
	// The Agile API takes fifty keys per call at most.
	const batch = 50
	for start := 0; start < len(clean); start += batch {
		end := start + batch
		if end > len(clean) {
			end = len(clean)
		}
		if err := c.jira(ctx, http.MethodPost, path, nil, map[string]any{"issues": clean[start:end]}, nil); err != nil {
			return err
		}
	}
	return nil
}

func (j *JiraAdapter) SetTeam(ctx context.Context, key string, teamID string) error {
	clean, err := cleanJiraKey(key)
	if err != nil {
		return err
	}
	c, err := j.forProject(ctx, nil)
	if err != nil {
		return err
	}
	return c.jiraSetTeam(ctx, clean, teamID)
}

func (j *JiraAdapter) SetParent(ctx context.Context, key string, parentKey string) error {
	clean, err := cleanJiraKey(key)
	if err != nil {
		return err
	}
	var parent any
	if p := strings.TrimSpace(parentKey); p != "" {
		parent = map[string]string{"key": strings.ToUpper(p)}
	}
	c, err := j.forProject(ctx, nil)
	if err != nil {
		return err
	}
	return c.jira(ctx, http.MethodPut, "/rest/api/3/issue/"+url.PathEscape(clean), nil,
		map[string]any{"fields": map[string]any{"parent": parent}}, nil)
}

func (j *JiraAdapter) UpdateLabels(ctx context.Context, key string, add []string, remove []string) error {
	clean, err := cleanJiraKey(key)
	if err != nil {
		return err
	}
	ops := labelOps(add, remove)
	if len(ops) == 0 {
		return nil
	}
	c, err := j.forProject(ctx, nil)
	if err != nil {
		return err
	}
	return c.jira(ctx, http.MethodPut, "/rest/api/3/issue/"+url.PathEscape(clean), nil,
		map[string]any{"update": map[string]any{"labels": ops}}, nil)
}

func (j *JiraAdapter) SearchAssignable(ctx context.Context, key string, query string, limit int) ([]tracker.Person, error) {
	clean, err := cleanJiraKey(key)
	if err != nil {
		return nil, err
	}
	c, err := j.forProject(ctx, nil)
	if err != nil {
		return nil, err
	}
	members, err := c.jiraAssignable(ctx, clean, query, limit)
	if err != nil {
		return nil, err
	}
	people := make([]tracker.Person, 0, len(members))
	for _, m := range members {
		people = append(people, tracker.Person{ID: m.AccountID, DisplayName: m.DisplayName, Email: m.Email, AvatarURL: m.AvatarURL, Active: m.Active})
	}
	return people, nil
}

// Read side.

func (j *JiraAdapter) ListBoards(ctx context.Context, req tracker.BoardsRequest) ([]models.TrackerBoard, error) {
	key, err := j.projectKey(req.Project)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("projectKeyOrId", key)
	// The three board kinds that carry a column configuration. "simple" is what
	// the Agile API calls the board of a team-managed project: it exposes the
	// same columnConfig as the others, so excluding it left every team-managed
	// project with no board at all, and its column detection failing with
	// "no board on project <KEY>".
	query.Set("type", "scrum,kanban,simple")
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	items, err := c.jiraAgilePages(ctx, "/rest/agile/1.0/board", query)
	if err != nil {
		return nil, err
	}
	boards := make([]models.TrackerBoard, 0, len(items))
	for _, raw := range items {
		var b struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
		}
		if json.Unmarshal(raw, &b) != nil || b.ID == 0 {
			continue
		}
		boards = append(boards, models.TrackerBoard{ID: fmt.Sprint(b.ID), Name: b.Name, Type: b.Type})
	}
	return boards, nil
}

func (j *JiraAdapter) ListSprints(ctx context.Context, req tracker.BoardRequest) ([]models.TrackerSprint, error) {
	boardID := strings.TrimSpace(req.BoardID)
	if boardID == "" && req.Project != nil {
		boardID = strings.TrimSpace(req.Project.BoardID)
	}
	if boardID == "" {
		return nil, fmt.Errorf("select a board on the project first")
	}
	query := url.Values{}
	query.Set("state", "active,future,closed")
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	items, err := c.jiraAgilePages(ctx, "/rest/agile/1.0/board/"+url.PathEscape(boardID)+"/sprint", query)
	if err != nil {
		return nil, err
	}
	out := make([]models.TrackerSprint, 0, len(items))
	for _, raw := range items {
		var sp struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			State     string `json:"state"`
			StartDate string `json:"startDate"`
			EndDate   string `json:"endDate"`
		}
		if json.Unmarshal(raw, &sp) != nil || strings.TrimSpace(sp.Name) == "" {
			continue
		}
		out = append(out, models.TrackerSprint{ID: fmt.Sprint(sp.ID), Name: sp.Name, State: strings.ToLower(sp.State), StartDate: sp.StartDate, EndDate: sp.EndDate})
	}
	return out, nil
}

// statusNamesByID resolves the status ids a board configuration carries.
func (c *Client) statusNamesByID(ctx context.Context) (map[string]string, error) {
	var statuses []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := c.jira(ctx, http.MethodGet, "/rest/api/3/status", nil, nil, &statuses); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(statuses))
	for _, st := range statuses {
		out[st.ID] = st.Name
	}
	return out, nil
}

func (j *JiraAdapter) ListBoardColumns(ctx context.Context, req tracker.BoardRequest) ([]models.TrackerColumn, error) {
	boardID := strings.TrimSpace(req.BoardID)
	if boardID == "" && req.Project != nil {
		boardID = strings.TrimSpace(req.Project.BoardID)
	}
	if boardID == "" {
		return nil, fmt.Errorf("select a board on the project first")
	}
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	var cfg struct {
		ColumnConfig struct {
			Columns []struct {
				Name     string `json:"name"`
				Statuses []struct {
					ID string `json:"id"`
				} `json:"statuses"`
			} `json:"columns"`
		} `json:"columnConfig"`
	}
	if err := c.jira(ctx, http.MethodGet, "/rest/agile/1.0/board/"+url.PathEscape(boardID)+"/configuration", nil, nil, &cfg); err != nil {
		return nil, err
	}
	names, err := c.statusNamesByID(ctx)
	if err != nil {
		return nil, err
	}
	columns := make([]models.TrackerColumn, 0, len(cfg.ColumnConfig.Columns))
	for _, col := range cfg.ColumnConfig.Columns {
		statuses := make([]string, 0, len(col.Statuses))
		for _, st := range col.Statuses {
			if name := names[st.ID]; name != "" {
				statuses = append(statuses, name)
			}
		}
		// A column with no status holds no card.
		if len(statuses) == 0 {
			continue
		}
		columns = append(columns, models.TrackerColumn{Name: col.Name, Statuses: statuses})
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("board %s has no usable column", boardID)
	}
	return columns, nil
}

func (j *JiraAdapter) ListStatuses(ctx context.Context, req tracker.ProjectRequest) ([]tracker.TrackerStatus, error) {
	key, err := j.projectKey(req.Project)
	if err != nil {
		return nil, err
	}
	var perType []struct {
		Statuses []struct {
			ID             string `json:"id"`
			Name           string `json:"name"`
			StatusCategory struct {
				Key string `json:"key"`
			} `json:"statusCategory"`
		} `json:"statuses"`
	}
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	if err := c.jira(ctx, http.MethodGet, "/rest/api/3/project/"+url.PathEscape(key)+"/statuses", nil, nil, &perType); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []tracker.TrackerStatus{}
	for _, t := range perType {
		for _, st := range t.Statuses {
			if st.Name == "" || seen[strings.ToLower(st.Name)] {
				continue
			}
			seen[strings.ToLower(st.Name)] = true
			out = append(out, tracker.TrackerStatus{ID: st.ID, Name: st.Name, Category: st.StatusCategory.Key})
		}
	}
	return out, nil
}

// jiraIssueType is one creatable type of a project.
type jiraIssueType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

func (c *Client) jiraIssueTypes(ctx context.Context, projectKey string) ([]jiraIssueType, error) {
	items, err := c.jiraCreateMetaPages(ctx, "/rest/api/3/issue/createmeta/"+url.PathEscape(projectKey)+"/issuetypes", "issueTypes")
	if err != nil {
		return nil, err
	}
	out := make([]jiraIssueType, 0, len(items))
	for _, raw := range items {
		var it jiraIssueType
		if json.Unmarshal(raw, &it) == nil && strings.TrimSpace(it.Name) != "" {
			out = append(out, it)
		}
	}
	return out, nil
}

func (j *JiraAdapter) ListIssueTypes(ctx context.Context, req tracker.ProjectRequest) ([]string, error) {
	key, err := j.projectKey(req.Project)
	if err != nil {
		return nil, err
	}
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	types, err := c.jiraIssueTypes(ctx, key)
	if err != nil {
		return nil, err
	}
	out := []string{}
	seen := map[string]bool{}
	for _, it := range types {
		// Sub-tasks are not board cards: they live under their parent.
		if it.Subtask || seen[it.Name] {
			continue
		}
		seen[it.Name] = true
		out = append(out, it.Name)
	}
	sort.Strings(out)
	return out, nil
}

func (j *JiraAdapter) ListEpics(ctx context.Context, req tracker.ProjectRequest) ([]models.Task, error) {
	key, err := j.projectKey(req.Project)
	if err != nil {
		return nil, err
	}
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	return j.search(ctx, c, jiraJQL(key, nil, "issuetype = Epic"))
}

func (j *JiraAdapter) SearchTeams(ctx context.Context, req tracker.TeamSearchRequest) ([]models.TrackerTeam, error) {
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	return c.jiraSearchTeams(ctx, req.Query)
}

func (j *JiraAdapter) TeamMembers(ctx context.Context, req tracker.TeamRequest) ([]models.TeamMember, error) {
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	return c.jiraTeamMembers(ctx, req.TeamID)
}

// optionFields tells which of the mandatory fields of one issue type are chosen
// from a list the site enumerates, and therefore take an option id rather than
// the value itself.
func (j *JiraAdapter) optionFields(ctx context.Context, project *models.Project, issueType string) (map[string]bool, error) {
	required, err := j.RequiredCreateFields(ctx, tracker.CreateMetaRequest{Project: project, IssueType: issueType})
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, field := range required {
		if len(field.Options) > 0 {
			out[field.ID] = true
		}
	}
	return out, nil
}

// missingRequiredFields lists, as "Name (id)", the fields the creation screen
// of issueType makes mandatory and the creation body sent did not carry. The
// body is what counts, not the request's custom fields: a description, labels
// or a priority the adapter sent are not missing, whatever Jira refused over.
// It answers an empty string when the screen cannot be read: the refusal it
// completes is then returned as Jira wrote it.
func (j *JiraAdapter) missingRequiredFields(ctx context.Context, project *models.Project, issueType string, sent map[string]any) string {
	required, err := j.RequiredCreateFields(ctx, tracker.CreateMetaRequest{Project: project, IssueType: issueType})
	if err != nil {
		return ""
	}
	missing := []string{}
	for _, field := range required {
		if _, ok := sent[field.ID]; ok {
			continue
		}
		missing = append(missing, fmt.Sprintf("%s (%s)", field.Name, field.ID))
	}
	return strings.Join(missing, ", ")
}

// RequiredCreateFields lists what the site makes mandatory on creation for one
// issue type, beyond project, type, summary and parent, with the allowed
// values when the site enumerates them. Fields with a default are left out:
// Jira fills them itself.
func (j *JiraAdapter) RequiredCreateFields(ctx context.Context, req tracker.CreateMetaRequest) ([]tracker.RequiredField, error) {
	key, err := j.projectKey(req.Project)
	if err != nil {
		return nil, err
	}
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	types, err := c.jiraIssueTypes(ctx, key)
	if err != nil {
		return nil, err
	}
	wanted := strings.TrimSpace(req.IssueType)
	if wanted == "" {
		wanted = "Epic"
	}
	typeID := ""
	for _, it := range types {
		if strings.EqualFold(it.Name, wanted) {
			typeID = it.ID
			break
		}
	}
	if typeID == "" {
		return nil, fmt.Errorf("issue type %q does not exist on project %s", wanted, key)
	}
	items, err := c.jiraCreateMetaPages(ctx, "/rest/api/3/issue/createmeta/"+url.PathEscape(key)+"/issuetypes/"+url.PathEscape(typeID), "fields")
	if err != nil {
		return nil, err
	}
	known := map[string]bool{"project": true, "issuetype": true, "summary": true, "parent": true, "reporter": true}
	out := []tracker.RequiredField{}
	for _, raw := range items {
		var f struct {
			FieldID         string `json:"fieldId"`
			Name            string `json:"name"`
			Required        bool   `json:"required"`
			HasDefaultValue bool   `json:"hasDefaultValue"`
			AllowedValues   []struct {
				ID    string `json:"id"`
				Value string `json:"value"`
				Name  string `json:"name"`
			} `json:"allowedValues"`
		}
		if json.Unmarshal(raw, &f) != nil || !f.Required || known[f.FieldID] || f.HasDefaultValue {
			continue
		}
		field := tracker.RequiredField{ID: f.FieldID, Name: f.Name, Options: []tracker.FieldOption{}}
		for _, av := range f.AllowedValues {
			label := av.Value
			if label == "" {
				label = av.Name
			}
			field.Options = append(field.Options, tracker.FieldOption{ID: av.ID, Value: label})
		}
		out = append(out, field)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out, nil
}
