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
	"strconv"
	"strings"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// GitlabAdapter drives a GitLab project, on gitlab.com or a self-managed
// instance, through tracker.TicketingSystem: the REST API v4 for issues,
// notes, labels, boards and milestones, GraphQL for iterations. It is a port
// of Taskativ's GitLab client onto the shared Client, with Sectile's mapping:
// the stage is a `#<stage>` label, the macro `macro:` / `parent:` labels, the
// team a `team::<name>` scoped label, a board column a board list, and a
// sprint either a milestone or an iteration (ADR 0030).
type GitlabAdapter struct {
	tracker.BaseTicketingSystem
	client *Client
	tiers  *gitlabTierCache
}

// The GitLab adapter answers the optional interfaces too.
var (
	_ tracker.PullRequestDiscoverer = (*GitlabAdapter)(nil)
	_ tracker.SprintManager         = (*GitlabAdapter)(nil)
)

// NewGitlabAdapter creates the TicketingSystem adapter for GitLab.
func NewGitlabAdapter(client *Client) *GitlabAdapter {
	return &GitlabAdapter{
		BaseTicketingSystem: tracker.BaseTicketingSystem{
			TrackerName: "gitlab",
			Capabilities: []tracker.Capability{
				tracker.CapCreate, tracker.CapUpdate, tracker.CapDelete, tracker.CapSync,
				tracker.CapGet, tracker.CapComment, tracker.CapLabels, tracker.CapAssign,
				tracker.CapTransition, tracker.CapSprint, tracker.CapSprintManage, tracker.CapTeam,
				tracker.CapBoard, tracker.CapPullRequests, tracker.CapIncrementalSync,
			},
		},
		client: client,
		tiers:  newGitlabTierCache(),
	}
}

func contextProjectID(ctx context.Context, p *models.Project) string {
	if p != nil && p.ID != "" {
		return p.ID
	}
	return tracker.Project(ctx)
}

// forProject is the client of one read, as on GitHub: the acting person's own
// token where they stored one, the server credential otherwise, including for
// the synchronisation, which names nobody (#464). A sealed token nobody
// unlocked refuses the read.
func (g *GitlabAdapter) forProject(ctx context.Context, p *models.Project) (*Client, string, error) {
	client, _, err := g.client.ForActingUser(tracker.ActingUser(ctx), "gitlab", contextProjectID(ctx, p))
	if err != nil {
		return nil, "", err
	}
	projectPath, err := client.gitlabProjectPath()
	if err != nil {
		return nil, "", err
	}
	return client, projectPath, nil
}

// forWrite is the client of one write: GitLab attributes an issue, a note or a
// label change to the account behind the token, so a person writes with their
// own token or not at all, and unattended work with the server's (#482).
func (g *GitlabAdapter) forWrite(ctx context.Context, p *models.Project) (*Client, string, error) {
	client, err := g.client.ForWrite(ctx, "gitlab", contextProjectID(ctx, p))
	if err != nil {
		return nil, "", err
	}
	projectPath, err := client.gitlabProjectPath()
	if err != nil {
		return nil, "", err
	}
	return client, projectPath, nil
}

// FormatTaskID gives a work item its local identity: gl-<projectID>-<iid>. A
// GitLab project is one Sectile project, so two of them never share an id,
// and the gl- prefix keeps them apart from GitHub's gh- ones.
func (g *GitlabAdapter) FormatTaskID(projectID string, key string, rawID string) string {
	iid, err := gitlabIssueIID(key)
	if err != nil && rawID != "" {
		iid, err = gitlabIssueIID(rawID)
	}
	num := strings.TrimPrefix(strings.TrimSpace(key), "#")
	if err == nil {
		num = strconv.Itoa(iid)
	}
	if projectID != "" && projectID != "default" {
		return fmt.Sprintf("gl-%s-%s", projectID, num)
	}
	return "gl-" + num
}

func gitlabIssuePath(projectPath string, iid int) string {
	return fmt.Sprintf("/projects/%s/issues/%d", gitlabProjectSegment(projectPath), iid)
}

// Issues.

func (g *GitlabAdapter) SyncIssues(ctx context.Context, req tracker.SyncRequest) ([]models.Task, error) {
	c, projectPath, err := g.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("scope", "all")
	query.Set("state", "all")
	query.Set("order_by", "updated_at")
	query.Set("sort", "desc")
	if req.UpdatedWithinMin > 0 {
		// GitLab takes an absolute instant; UTC so no timezone has to be agreed.
		since := time.Now().UTC().Add(-time.Duration(req.UpdatedWithinMin) * time.Minute)
		query.Set("updated_after", since.Format(time.RFC3339))
	}
	items, err := c.gitlabPages(ctx, "/projects/"+gitlabProjectSegment(projectPath)+"/issues", query)
	if err != nil {
		return nil, err
	}
	listLabels := g.listLabelSet(ctx, c, projectPath)
	tasks := make([]models.Task, 0, len(items))
	for _, raw := range items {
		var item GitlabIssueItem
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, fmt.Errorf("GitLab a renvoyé un ticket illisible : %w", err)
		}
		task, err := gitlabTask(item, listLabels)
		if err != nil {
			return nil, err
		}
		task.Position = len(tasks)
		tasks = append(tasks, *task)
	}
	return tasks, nil
}

// listLabelSet is the set of labels the project's boards use as lists, which
// is what a card's column is read from. A project whose boards cannot be read
// still synchronises; its cards then sit in Open or Closed.
func (g *GitlabAdapter) listLabelSet(ctx context.Context, c *Client, projectPath string) map[string]bool {
	boards, err := c.gitlabBoards(ctx, projectPath)
	if err != nil {
		log.Printf("[gitlab] boards de %s illisibles, colonnes réduites à Open et Closed : %v", projectPath, err)
		return nil
	}
	return boardListLabels(boards)
}

func (g *GitlabAdapter) GetIssue(ctx context.Context, req tracker.GetIssueRequest) (*models.Task, error) {
	iid, err := gitlabIssueIID(req.Key)
	if err != nil {
		return nil, err
	}
	c, projectPath, err := g.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	var item GitlabIssueItem
	if err := c.gitlab(ctx, http.MethodGet, gitlabIssuePath(projectPath, iid), nil, nil, &item); err != nil {
		return nil, err
	}
	return gitlabTask(item, g.listLabelSet(ctx, c, projectPath))
}

// gitlabIssueTypes are the work item types the issues API creates.
var gitlabIssueTypes = []string{"issue", "incident", "task", "test_case"}

func (g *GitlabAdapter) CreateIssue(ctx context.Context, req tracker.CreateIssueRequest) (*models.Task, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, fmt.Errorf("le titre du ticket est obligatoire")
	}
	c, projectPath, err := g.forWrite(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	labels := cleanGitlabLabels(req.Labels)
	hasStage := false
	for _, l := range labels {
		if isStageLabel(l) {
			hasStage = true
		}
	}
	if !hasStage {
		labels = append(labels, "#new")
	}
	if team := strings.TrimSpace(req.Team); team != "" {
		labels = append(removeTeamLabels(labels), gitlabTeamPrefix+team)
	}
	if parent := strings.TrimSpace(req.ParentKey); parent != "" {
		labels = append(labels, "parent:"+parent)
	}
	payload := map[string]any{"title": title, "labels": strings.Join(cleanGitlabLabels(labels), ",")}
	if d := strings.TrimSpace(req.Description); d != "" {
		payload["description"] = d
	}
	if t := strings.ToLower(strings.TrimSpace(req.IssueType)); t != "" {
		if !containsString(gitlabIssueTypes, t) {
			return nil, fmt.Errorf("type de ticket GitLab inconnu : %q (attendu %s)", req.IssueType, strings.Join(gitlabIssueTypes, ", "))
		}
		payload["issue_type"] = t
	}
	if a := strings.TrimSpace(req.Assignee); a != "" {
		id, err := c.gitlabMemberID(ctx, projectPath, a)
		if err != nil {
			return nil, err
		}
		payload["assignee_ids"] = []int64{id}
	}
	var iterationSprint string
	if s := strings.TrimSpace(req.Sprint); s != "" {
		sprint, err := parseGitlabSprintID(s)
		if err != nil {
			return nil, err
		}
		if sprint.kind == gitlabMilestoneKind {
			payload["milestone_id"] = sprint.id
		} else {
			iterationSprint = s
		}
	}
	var item GitlabIssueItem
	if err := c.gitlab(ctx, http.MethodPost, "/projects/"+gitlabProjectSegment(projectPath)+"/issues", nil, payload, &item); err != nil {
		return nil, err
	}
	if item.IID < 1 {
		return nil, fmt.Errorf("GitLab n'a pas confirmé la création du ticket")
	}
	if iterationSprint != "" {
		if err := g.setSprint(ctx, c, projectPath, iterationSprint, []int{item.IID}); err != nil {
			return nil, fmt.Errorf("ticket #%d créé, mais son itération n'a pas pu être posée : %w", item.IID, err)
		}
	}
	return gitlabTask(item, nil)
}

// cleanGitlabLabels trims, drops empties and duplicates. GitLab labels may
// carry spaces; a comma would split one label in two, so it is refused.
func cleanGitlabLabels(labels []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, l := range labels {
		l = strings.TrimSpace(strings.ReplaceAll(l, ",", " "))
		if l == "" || seen[strings.ToLower(l)] {
			continue
		}
		seen[strings.ToLower(l)] = true
		out = append(out, l)
	}
	return out
}

func removeTeamLabels(labels []string) []string {
	out := labels[:0:0]
	for _, l := range labels {
		if !strings.HasPrefix(strings.ToLower(l), gitlabTeamPrefix) {
			out = append(out, l)
		}
	}
	return out
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// labelDelta is a label change expressed as GitLab's add_labels and
// remove_labels, which leave every other label of the issue alone. A label in
// both lists is removed: the removal is the newer intent. Posing a stage label
// removes every other one, so a stage write keeps exactly one.
func labelDelta(add, remove []string) (adds, removes []string) {
	dropped := map[string]bool{}
	removes = []string{}
	for _, l := range cleanGitlabLabels(remove) {
		dropped[strings.ToLower(l)] = true
		removes = append(removes, l)
	}
	adds = []string{}
	stagePosed := map[string]bool{}
	for _, l := range cleanGitlabLabels(add) {
		if dropped[strings.ToLower(l)] {
			continue
		}
		adds = append(adds, l)
		if isStageLabel(l) {
			stagePosed[strings.ToLower(l)] = true
		}
	}
	if len(stagePosed) > 0 {
		for _, s := range gitlabStageLabels {
			if !stagePosed[s] && !dropped[s] {
				removes = append(removes, s)
				dropped[s] = true
			}
		}
	}
	return adds, removes
}

func (g *GitlabAdapter) UpdateIssue(ctx context.Context, req tracker.UpdateIssueRequest) error {
	key := req.Key
	if key == "" && req.Task != nil {
		key = req.Task.Key
	}
	iid, err := gitlabIssueIID(key)
	if err != nil {
		return err
	}
	c, projectPath, err := g.forWrite(ctx, req.Project)
	if err != nil {
		return err
	}
	payload := map[string]any{}
	if req.Title != nil {
		payload["title"] = *req.Title
	}
	if req.Description != nil {
		payload["description"] = *req.Description
	}
	adds, removes := labelDelta(req.Labels, req.RemovedLabels)
	// The column first: a named target decides the state and swaps the list
	// labels, over whatever list label the task's own labels still carry. The
	// finished rule comes after and wins, since finishing a work item closes
	// its issue whatever column the stage maps to (D10). A column GitLab no
	// longer has does not cost the rest of the update: it is reported after.
	stateEvent := ""
	var moveErr error
	if target := strings.TrimSpace(req.TargetStatus); target != "" {
		move, err := g.columnMove(ctx, c, projectPath, target)
		if err != nil {
			moveErr = err
		} else {
			adds = append(withoutLabels(adds, move.remove), move.add...)
			removes = append(withoutLabels(removes, move.add), move.remove...)
			stateEvent = move.stateEvent
		}
	}
	if closed, explicit := isFinishedStatus(req.Status, req.Labels); explicit {
		switch {
		case closed:
			stateEvent = "close"
		case stateEvent == "":
			stateEvent = "reopen"
		}
	}
	if len(adds) > 0 {
		payload["add_labels"] = strings.Join(adds, ",")
	}
	if len(removes) > 0 {
		payload["remove_labels"] = strings.Join(removes, ",")
	}
	if stateEvent != "" {
		payload["state_event"] = stateEvent
	}
	if req.Assignee != nil {
		ids, err := c.gitlabAssigneeIDs(ctx, projectPath, *req.Assignee)
		if err != nil {
			return err
		}
		payload["assignee_ids"] = ids
	}
	if len(payload) > 0 {
		if err := c.gitlab(ctx, http.MethodPut, gitlabIssuePath(projectPath, iid), nil, payload, nil); err != nil {
			return err
		}
	}
	return moveErr
}

// withoutLabels drops from labels those in drop, case-insensitively.
func withoutLabels(labels, drop []string) []string {
	out := []string{}
	for _, l := range labels {
		if !containsFold(drop, l) {
			out = append(out, l)
		}
	}
	return out
}

func containsFold(list []string, s string) bool {
	for _, item := range list {
		if strings.EqualFold(item, s) {
			return true
		}
	}
	return false
}

// DeleteIssue closes the issue, or deletes it when the caller asks for a
// deletion. Only a project owner may delete an issue on GitLab, so a refused
// deletion falls back to closing, which is what the board means by removing a
// card anyway.
func (g *GitlabAdapter) DeleteIssue(ctx context.Context, req tracker.DeleteIssueRequest) error {
	iid, err := gitlabIssueIID(req.Key)
	if err != nil {
		return err
	}
	c, projectPath, err := g.forWrite(ctx, req.Project)
	if err != nil {
		return err
	}
	closeIssue := func() error {
		return c.gitlab(ctx, http.MethodPut, gitlabIssuePath(projectPath, iid), nil, map[string]any{"state_event": "close"}, nil)
	}
	if req.CloseOnly {
		return closeIssue()
	}
	err = c.gitlab(ctx, http.MethodDelete, gitlabIssuePath(projectPath, iid), nil, nil, nil)
	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr.Status == http.StatusForbidden {
		return closeIssue()
	}
	return err
}

// Notes.

func (g *GitlabAdapter) AddComment(ctx context.Context, req tracker.AddCommentRequest) error {
	if strings.TrimSpace(req.Body) == "" {
		return fmt.Errorf("le commentaire est vide")
	}
	iid, err := gitlabIssueIID(req.Key)
	if err != nil {
		return err
	}
	c, projectPath, err := g.forWrite(ctx, req.Project)
	if err != nil {
		return err
	}
	return c.gitlab(ctx, http.MethodPost, gitlabIssuePath(projectPath, iid)+"/notes", nil, map[string]any{"body": req.Body}, nil)
}

// GetComments returns the issue's user notes, oldest first. System notes
// ("changed the label", "closed") are GitLab's history, not comments.
func (g *GitlabAdapter) GetComments(ctx context.Context, req tracker.GetCommentsRequest) ([]models.TaskComment, error) {
	iid, err := gitlabIssueIID(req.Key)
	if err != nil {
		return nil, err
	}
	c, projectPath, err := g.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("sort", "asc")
	query.Set("order_by", "created_at")
	items, err := c.gitlabPages(ctx, gitlabIssuePath(projectPath, iid)+"/notes", query)
	if err != nil {
		return nil, err
	}
	comments := []models.TaskComment{}
	for _, raw := range items {
		var note struct {
			ID        int64      `json:"id"`
			Body      string     `json:"body"`
			System    bool       `json:"system"`
			CreatedAt *time.Time `json:"created_at"`
			Author    gitlabUser `json:"author"`
		}
		if err := json.Unmarshal(raw, &note); err != nil {
			return nil, fmt.Errorf("GitLab a renvoyé une note illisible : %w", err)
		}
		if note.System {
			continue
		}
		comments = append(comments, models.TaskComment{
			ID:        strconv.FormatInt(note.ID, 10),
			Author:    note.Author.Username,
			Body:      note.Body,
			CreatedAt: note.CreatedAt,
			Source:    "gitlab",
		})
	}
	return comments, nil
}

// Labels, assignee.

func (g *GitlabAdapter) UpdateLabels(ctx context.Context, key string, add []string, remove []string) error {
	iid, err := gitlabIssueIID(key)
	if err != nil {
		return err
	}
	adds, removes := labelDelta(add, remove)
	if len(adds) == 0 && len(removes) == 0 {
		return nil
	}
	c, projectPath, err := g.forWrite(ctx, nil)
	if err != nil {
		return err
	}
	payload := map[string]any{}
	if len(adds) > 0 {
		payload["add_labels"] = strings.Join(adds, ",")
	}
	if len(removes) > 0 {
		payload["remove_labels"] = strings.Join(removes, ",")
	}
	return c.gitlab(ctx, http.MethodPut, gitlabIssuePath(projectPath, iid), nil, payload, nil)
}

func (g *GitlabAdapter) Assign(ctx context.Context, key string, personID string) error {
	iid, err := gitlabIssueIID(key)
	if err != nil {
		return err
	}
	c, projectPath, err := g.forWrite(ctx, nil)
	if err != nil {
		return err
	}
	ids, err := c.gitlabAssigneeIDs(ctx, projectPath, personID)
	if err != nil {
		return err
	}
	return c.gitlab(ctx, http.MethodPut, gitlabIssuePath(projectPath, iid), nil, map[string]any{"assignee_ids": ids}, nil)
}

// gitlabAssigneeIDs is the assignee_ids of one assignment: [0] unassigns,
// which is how the API spells it.
func (c *Client) gitlabAssigneeIDs(ctx context.Context, projectPath, person string) ([]int64, error) {
	if strings.TrimSpace(person) == "" {
		return []int64{0}, nil
	}
	id, err := c.gitlabMemberID(ctx, projectPath, person)
	if err != nil {
		return nil, err
	}
	return []int64{id}, nil
}

// gitlabMemberID resolves a person to a GitLab user id: a number is one
// already, anything else is a username looked up among the project's members.
func (c *Client) gitlabMemberID(ctx context.Context, projectPath, person string) (int64, error) {
	person = strings.TrimPrefix(strings.TrimSpace(person), "@")
	if id, err := strconv.ParseInt(person, 10, 64); err == nil && id > 0 {
		return id, nil
	}
	members, err := c.gitlabMembers(ctx, projectPath, person, 0)
	if err != nil {
		return 0, err
	}
	for _, m := range members {
		if strings.EqualFold(m.Username, person) {
			return m.ID, nil
		}
	}
	return 0, fmt.Errorf("aucun membre GitLab « %s » dans le projet", person)
}

// gitlabMembers lists the project's members, inherited ones included,
// narrowed by a search when query is set. limit > 0 reads one page of that
// size; otherwise every page is read.
func (c *Client) gitlabMembers(ctx context.Context, projectPath, query string, limit int) ([]gitlabUser, error) {
	q := url.Values{}
	if s := strings.TrimSpace(query); s != "" {
		q.Set("query", s)
	}
	path := "/projects/" + gitlabProjectSegment(projectPath) + "/members/all"
	var members []gitlabUser
	if limit > 0 {
		if limit > gitlabPageSize {
			limit = gitlabPageSize
		}
		q.Set("per_page", strconv.Itoa(limit))
		if err := c.gitlab(ctx, http.MethodGet, path, q, nil, &members); err != nil {
			return nil, err
		}
		return members, nil
	}
	items, err := c.gitlabPages(ctx, path, q)
	if err != nil {
		return nil, err
	}
	for _, raw := range items {
		var m gitlabUser
		if json.Unmarshal(raw, &m) == nil && m.ID > 0 {
			members = append(members, m)
		}
	}
	return members, nil
}

func (g *GitlabAdapter) SearchAssignable(ctx context.Context, key string, query string, limit int) ([]tracker.Person, error) {
	c, projectPath, err := g.forProject(ctx, nil)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	members, err := c.gitlabMembers(ctx, projectPath, query, limit)
	if err != nil {
		return nil, err
	}
	people := make([]tracker.Person, 0, len(members))
	for _, m := range members {
		name := m.Name
		if name == "" {
			name = m.Username
		}
		people = append(people, tracker.Person{ID: strconv.FormatInt(m.ID, 10), DisplayName: name, AvatarURL: m.AvatarURL, Active: m.State == "active"})
	}
	return people, nil
}

// Teams: one `team::<name>` scoped label per issue. A GitLab team has no id of
// its own, so its name serves as one.

func (g *GitlabAdapter) SetTeam(ctx context.Context, key string, teamID string) error {
	iid, err := gitlabIssueIID(key)
	if err != nil {
		return err
	}
	c, projectPath, err := g.forWrite(ctx, nil)
	if err != nil {
		return err
	}
	var item GitlabIssueItem
	if err := c.gitlab(ctx, http.MethodGet, gitlabIssuePath(projectPath, iid), nil, nil, &item); err != nil {
		return err
	}
	team := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(teamID), gitlabTeamPrefix))
	want := ""
	if team != "" {
		want = gitlabTeamPrefix + team
	}
	var removes []string
	for _, l := range item.Labels {
		if strings.HasPrefix(strings.ToLower(l), gitlabTeamPrefix) && l != want {
			removes = append(removes, l)
		}
	}
	payload := map[string]any{}
	if want != "" {
		payload["add_labels"] = want
	}
	if len(removes) > 0 {
		payload["remove_labels"] = strings.Join(removes, ",")
	}
	if len(payload) == 0 {
		return nil
	}
	return c.gitlab(ctx, http.MethodPut, gitlabIssuePath(projectPath, iid), nil, payload, nil)
}

func (g *GitlabAdapter) SearchTeams(ctx context.Context, req tracker.TeamSearchRequest) ([]models.TrackerTeam, error) {
	c, projectPath, err := g.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("search", gitlabTeamPrefix)
	items, err := c.gitlabPages(ctx, "/projects/"+gitlabProjectSegment(projectPath)+"/labels", q)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(strings.TrimSpace(req.Query))
	teams := []models.TrackerTeam{}
	seen := map[string]bool{}
	for _, raw := range items {
		var label struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(raw, &label) != nil || !strings.HasPrefix(strings.ToLower(label.Name), gitlabTeamPrefix) {
			continue
		}
		name := strings.TrimSpace(label.Name[len(gitlabTeamPrefix):])
		if name == "" || seen[strings.ToLower(name)] || (needle != "" && !strings.Contains(strings.ToLower(name), needle)) {
			continue
		}
		seen[strings.ToLower(name)] = true
		teams = append(teams, models.TrackerTeam{ID: name, Name: name})
	}
	sort.Slice(teams, func(a, b int) bool { return strings.ToLower(teams[a].Name) < strings.ToLower(teams[b].Name) })
	return teams, nil
}

// TeamMembers answers the project's members: a label carries no people.
func (g *GitlabAdapter) TeamMembers(ctx context.Context, req tracker.TeamRequest) ([]models.TeamMember, error) {
	c, projectPath, err := g.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	members, err := c.gitlabMembers(ctx, projectPath, "", 0)
	if err != nil {
		return nil, err
	}
	out := make([]models.TeamMember, 0, len(members))
	for _, m := range members {
		name := m.Name
		if name == "" {
			name = m.Username
		}
		out = append(out, models.TeamMember{TeamID: req.TeamID, TeamName: req.TeamID, AccountID: strconv.FormatInt(m.ID, 10), DisplayName: name, AvatarURL: m.AvatarURL, Active: m.State == "active"})
	}
	return out, nil
}

// Read side.

func (g *GitlabAdapter) ListIssueTypes(ctx context.Context, req tracker.ProjectRequest) ([]string, error) {
	return append([]string{}, gitlabIssueTypes...), nil
}

// RequiredCreateFields is empty: a GitLab issue needs a title and nothing else.
func (g *GitlabAdapter) RequiredCreateFields(ctx context.Context, req tracker.CreateMetaRequest) ([]tracker.RequiredField, error) {
	return []tracker.RequiredField{}, nil
}

// IssuePullRequests returns the merge requests GitLab relates to the issue,
// oldest first, so the last one is the current one.
func (g *GitlabAdapter) IssuePullRequests(ctx context.Context, req tracker.IssuePullRequestsRequest) ([]models.TaskPullRequest, error) {
	iid, err := gitlabIssueIID(req.Key)
	if err != nil {
		return nil, err
	}
	c, projectPath, err := g.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	items, err := c.gitlabPages(ctx, gitlabIssuePath(projectPath, iid)+"/related_merge_requests", nil)
	if err != nil {
		return nil, err
	}
	type mergeRequest struct {
		WebURL       string    `json:"web_url"`
		SourceBranch string    `json:"source_branch"`
		CreatedAt    time.Time `json:"created_at"`
	}
	found := make([]mergeRequest, 0, len(items))
	for _, raw := range items {
		var mr mergeRequest
		if json.Unmarshal(raw, &mr) == nil && strings.TrimSpace(mr.WebURL) != "" {
			found = append(found, mr)
		}
	}
	sort.SliceStable(found, func(a, b int) bool { return found[a].CreatedAt.Before(found[b].CreatedAt) })
	links := make([]models.TaskPullRequest, 0, len(found))
	for _, mr := range found {
		links = append(links, models.TaskPullRequest{URL: mr.WebURL, Branch: mr.SourceBranch})
	}
	return links, nil
}
