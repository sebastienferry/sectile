package trackerapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// A GitLab sprint is one of two things: a project milestone, on every tier, or
// a group iteration, on Premium. The sprint id says which, milestone:<id> or
// iteration:<id>, so moving a work item into a sprint never has to guess or
// probe (D5). What the instance does not offer is absent from the lists, and a
// write that needs it is refused (D6).

const (
	gitlabMilestoneKind = "milestone"
	gitlabIterationKind = "iteration"
)

type gitlabSprintID struct {
	kind string
	id   int64
}

func (s gitlabSprintID) String() string { return s.kind + ":" + strconv.FormatInt(s.id, 10) }

func (s gitlabSprintID) iterationGID() string {
	return "gid://gitlab/Iteration/" + strconv.FormatInt(s.id, 10)
}

// parseGitlabSprintID reads a sprint id. A bare number, which a value stored
// before GitLab had two sprint kinds would be, is refused rather than guessed.
func parseGitlabSprintID(raw string) (gitlabSprintID, error) {
	kind, id, ok := strings.Cut(strings.TrimSpace(raw), ":")
	n, err := strconv.ParseInt(strings.TrimSpace(id), 10, 64)
	kind = strings.ToLower(strings.TrimSpace(kind))
	if !ok || err != nil || n < 1 || (kind != gitlabMilestoneKind && kind != gitlabIterationKind) {
		return gitlabSprintID{}, fmt.Errorf("identifiant de sprint GitLab illisible : %q (attendu milestone:<id> ou iteration:<id>)", raw)
	}
	return gitlabSprintID{kind: kind, id: n}, nil
}

// gitlabIDFromGID reads the number at the end of a GraphQL global id.
func gitlabIDFromGID(gid string) int64 {
	n, _ := strconv.ParseInt(gid[strings.LastIndex(gid, "/")+1:], 10, 64)
	return n
}

// The Premium probe. Whether a group has iterations is a property of the
// instance's licence and of the group, which changes rarely: it is asked once
// and remembered for an hour. A transport failure or a server error is not an
// answer and is not remembered, so the next call asks again.

const gitlabTierTTL = time.Hour

type gitlabTierEntry struct {
	iterations bool
	checkedAt  time.Time
}

type gitlabTierCache struct {
	mu      sync.Mutex
	entries map[string]gitlabTierEntry
	now     func() time.Time
}

func newGitlabTierCache() *gitlabTierCache {
	return &gitlabTierCache{entries: map[string]gitlabTierEntry{}, now: time.Now}
}

// gitlabProjectLocation is where a project sits: its full path, which GraphQL
// addresses it by, and its group, which owns the iterations. A project in a
// personal namespace has no group, hence no iterations.
type gitlabProjectLocation struct {
	fullPath string
	group    string
}

func (c *Client) gitlabLocate(ctx context.Context, projectPath string) (gitlabProjectLocation, error) {
	if _, err := strconv.ParseInt(projectPath, 10, 64); err != nil {
		group := ""
		if strings.Contains(projectPath, "/") {
			group = path.Dir(projectPath)
		}
		return gitlabProjectLocation{fullPath: projectPath, group: group}, nil
	}
	var project struct {
		PathWithNamespace string `json:"path_with_namespace"`
		Namespace         struct {
			Kind     string `json:"kind"`
			FullPath string `json:"full_path"`
		} `json:"namespace"`
	}
	if err := c.gitlab(ctx, http.MethodGet, "/projects/"+gitlabProjectSegment(projectPath), nil, nil, &project); err != nil {
		return gitlabProjectLocation{}, err
	}
	loc := gitlabProjectLocation{fullPath: project.PathWithNamespace}
	if project.Namespace.Kind == "group" {
		loc.group = project.Namespace.FullPath
	}
	return loc, nil
}

// iterationsAvailable reports whether the group has iterations the token can
// see: a Premium licence, a group namespace and the permission to read it.
func (g *GitlabAdapter) iterationsAvailable(ctx context.Context, c *Client, group string) bool {
	if strings.TrimSpace(group) == "" {
		return false
	}
	key := strings.TrimRight(c.GitlabURL, "/") + "|" + strings.ToLower(group)
	g.tiers.mu.Lock()
	entry, ok := g.tiers.entries[key]
	now := g.tiers.now()
	g.tiers.mu.Unlock()
	if ok && now.Sub(entry.checkedAt) < gitlabTierTTL {
		return entry.iterations
	}
	var out struct {
		Group *struct {
			IterationCadences struct {
				Nodes []struct {
					ID string `json:"id"`
				} `json:"nodes"`
			} `json:"iterationCadences"`
		} `json:"group"`
	}
	err := c.gitlabGraphQL(ctx, `query($path: ID!) { group(fullPath: $path) { iterationCadences(first: 1) { nodes { id } } } }`, map[string]any{"path": group}, &out)
	if err != nil && !gitlabDefinitiveRefusal(err) {
		return false
	}
	available := err == nil && out.Group != nil
	g.tiers.mu.Lock()
	g.tiers.entries[key] = gitlabTierEntry{iterations: available, checkedAt: now}
	g.tiers.mu.Unlock()
	return available
}

// gitlabDefinitiveRefusal separates "the instance says no" (a GraphQL error
// such as an unknown field on a Free instance, a 403, a 404) from a failure to
// ask (transport error, 5xx, rate limit), which says nothing about the tier.
func gitlabDefinitiveRefusal(err error) bool {
	var gqlErr *gitlabGraphQLError
	if errors.As(err, &gqlErr) {
		return true
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Status == http.StatusForbidden || httpErr.Status == http.StatusNotFound
	}
	return false
}

func iterationsUnsupported() error {
	return fmt.Errorf("les itérations ne sont pas disponibles sur cette instance GitLab : %w", tracker.Unsupported("gitlab", tracker.CapSprint))
}

// Reading.

func gitlabMilestoneSprint(m gitlabMilestone, today string) models.TrackerSprint {
	state := "active"
	switch {
	case m.State == "closed":
		state = "closed"
	case m.StartDate != "" && m.StartDate > today:
		state = "future"
	}
	return models.TrackerSprint{
		ID:        gitlabSprintID{kind: gitlabMilestoneKind, id: m.ID}.String(),
		Name:      m.Title,
		State:     state,
		StartDate: m.StartDate,
		EndDate:   m.DueDate,
	}
}

type gitlabIterationNode struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	State            string `json:"state"`
	StartDate        string `json:"startDate"`
	DueDate          string `json:"dueDate"`
	IterationCadence *struct {
		ID        string `json:"id"`
		Automatic bool   `json:"automatic"`
	} `json:"iterationCadence"`
}

func (n gitlabIterationNode) automatic() bool {
	return n.IterationCadence != nil && n.IterationCadence.Automatic
}

func (n gitlabIterationNode) sprint() models.TrackerSprint {
	state := "active"
	switch strings.ToLower(n.State) {
	case "closed":
		state = "closed"
	case "upcoming":
		state = "future"
	}
	return models.TrackerSprint{
		ID:        gitlabSprintID{kind: gitlabIterationKind, id: gitlabIDFromGID(n.ID)}.String(),
		Name:      gitlabIterationName(n.Title, n.StartDate, n.DueDate),
		State:     state,
		StartDate: n.StartDate,
		EndDate:   n.DueDate,
	}
}

const gitlabIterationFields = `id title state startDate dueDate iterationCadence { id automatic }`

// ListSprints returns the project's milestones, those of its ancestor groups
// included, then the group's iterations when the instance has them. GitLab
// sprints are not per board, so the board is ignored.
func (g *GitlabAdapter) ListSprints(ctx context.Context, req tracker.BoardRequest) ([]models.TrackerSprint, error) {
	c, projectPath, err := g.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("include_ancestors", "true")
	items, err := c.gitlabPages(ctx, "/projects/"+gitlabProjectSegment(projectPath)+"/milestones", query)
	if err != nil {
		return nil, err
	}
	today := time.Now().Format("2006-01-02")
	sprints := make([]models.TrackerSprint, 0, len(items))
	for _, raw := range items {
		var m gitlabMilestone
		if json.Unmarshal(raw, &m) == nil && m.ID > 0 && strings.TrimSpace(m.Title) != "" {
			sprints = append(sprints, gitlabMilestoneSprint(m, today))
		}
	}
	loc, err := c.gitlabLocate(ctx, projectPath)
	if err != nil || !g.iterationsAvailable(ctx, c, loc.group) {
		// No iterations is a Free instance or a personal namespace, not a
		// failure of the list.
		return sprints, nil
	}
	cursor := ""
	for page := 0; page < gitlabMaxPages; page++ {
		var out struct {
			Group *struct {
				Iterations struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []gitlabIterationNode `json:"nodes"`
				} `json:"iterations"`
			} `json:"group"`
		}
		vars := map[string]any{"path": loc.group}
		if cursor != "" {
			vars["after"] = cursor
		}
		q := `query($path: ID!, $after: String) { group(fullPath: $path) { iterations(includeAncestors: true, first: 100, after: $after) { pageInfo { hasNextPage endCursor } nodes { ` + gitlabIterationFields + ` } } } }`
		if err := c.gitlabGraphQL(ctx, q, vars, &out); err != nil {
			return nil, err
		}
		if out.Group == nil {
			break
		}
		for _, n := range out.Group.Iterations.Nodes {
			if gitlabIDFromGID(n.ID) > 0 {
				sprints = append(sprints, n.sprint())
			}
		}
		if !out.Group.Iterations.PageInfo.HasNextPage || out.Group.Iterations.PageInfo.EndCursor == "" {
			break
		}
		cursor = out.Group.Iterations.PageInfo.EndCursor
	}
	return sprints, nil
}

// gitlabIteration reads one iteration; nil when GitLab no longer knows it.
func (c *Client) gitlabIteration(ctx context.Context, id gitlabSprintID) (*gitlabIterationNode, error) {
	var out struct {
		Iteration *gitlabIterationNode `json:"iteration"`
	}
	if err := c.gitlabGraphQL(ctx, `query($id: IterationID!) { iteration(id: $id) { `+gitlabIterationFields+` } }`, map[string]any{"id": id.iterationGID()}, &out); err != nil {
		return nil, err
	}
	return out.Iteration, nil
}

// Moving work items.

func (g *GitlabAdapter) SetSprint(ctx context.Context, sprintID string, keys []string) error {
	iids := make([]int, 0, len(keys))
	for _, k := range keys {
		if iid, err := gitlabIssueIID(k); err == nil {
			iids = append(iids, iid)
		}
	}
	if len(iids) == 0 {
		return fmt.Errorf("aucun ticket à déplacer")
	}
	c, projectPath, err := g.forWrite(ctx, nil)
	if err != nil {
		return err
	}
	return g.setSprint(ctx, c, projectPath, sprintID, iids)
}

// setSprint puts issues into one sprint, or out of every sprint when sprintID
// is empty. A work item has one sprint: moving it into a milestone clears its
// iteration and the reverse, or the card would keep showing the other one.
func (g *GitlabAdapter) setSprint(ctx context.Context, c *Client, projectPath, sprintID string, iids []int) error {
	var target gitlabSprintID
	if strings.TrimSpace(sprintID) != "" {
		parsed, err := parseGitlabSprintID(sprintID)
		if err != nil {
			return err
		}
		target = parsed
	}
	loc, err := c.gitlabLocate(ctx, projectPath)
	if err != nil {
		return err
	}
	iterations := g.iterationsAvailable(ctx, c, loc.group)
	if target.kind == gitlabIterationKind && !iterations {
		return iterationsUnsupported()
	}
	for _, iid := range iids {
		milestone := int64(0)
		if target.kind == gitlabMilestoneKind {
			milestone = target.id
		}
		if err := c.gitlab(ctx, http.MethodPut, gitlabIssuePath(projectPath, iid), nil, map[string]any{"milestone_id": milestone}, nil); err != nil {
			return fmt.Errorf("#%d : %w", iid, err)
		}
		if !iterations {
			continue
		}
		var iteration any
		if target.kind == gitlabIterationKind {
			iteration = target.iterationGID()
		}
		if err := c.gitlabSetIteration(ctx, loc.fullPath, iid, iteration); err != nil {
			return fmt.Errorf("#%d : %w", iid, err)
		}
	}
	return nil
}

// gitlabSetIteration sets an issue's iteration, or clears it with a nil id:
// GraphQL wants an explicit null there.
func (c *Client) gitlabSetIteration(ctx context.Context, projectFullPath string, iid int, iterationGID any) error {
	var out struct {
		IssueSetIteration struct {
			Errors []string `json:"errors"`
		} `json:"issueSetIteration"`
	}
	input := map[string]any{"projectPath": projectFullPath, "iid": strconv.Itoa(iid), "iterationId": iterationGID}
	if err := c.gitlabGraphQL(ctx, `mutation($input: IssueSetIterationInput!) { issueSetIteration(input: $input) { errors } }`, map[string]any{"input": input}, &out); err != nil {
		return err
	}
	return gitlabMutationErrors("itération non affectée", out.IssueSetIteration.Errors)
}

// Managing sprints.

// gitlabSprintDate writes a date the way GitLab takes it, YYYY-MM-DD.
func gitlabSprintDate(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.Format("2006-01-02"), nil
	}
	if _, err := time.Parse("2006-01-02", raw); err != nil {
		return "", fmt.Errorf("date invalide %q : attendu AAAA-MM-JJ", raw)
	}
	return raw, nil
}

// gitlabDueDay is the last day of a sprint, which a milestone's due date names
// inclusively. Batch creation ends a sprint one second before the next one
// starts, 08:59:59 when sprints start at 09:00: a sprint that ends before the
// hour it started at has its last full day the day before.
func gitlabDueDay(start, end time.Time) time.Time {
	clock := func(t time.Time) int { return t.Hour()*3600 + t.Minute()*60 + t.Second() }
	if !start.IsZero() && clock(end) < clock(start) {
		return end.AddDate(0, 0, -1)
	}
	return end
}

// createKind is the kind of sprint "create sprints" produces. It is always a
// milestone until the owner decides otherwise (spec #398, open point O1); the
// decision lands here and nowhere else.
func createKind(tracker.SprintCreateRequest) string { return gitlabMilestoneKind }

// CreateSprint creates one milestone with its dates.
func (g *GitlabAdapter) CreateSprint(ctx context.Context, req tracker.SprintCreateRequest) (models.TrackerSprint, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return models.TrackerSprint{}, fmt.Errorf("un sprint doit avoir un nom")
	}
	if createKind(req) != gitlabMilestoneKind {
		return models.TrackerSprint{}, iterationsUnsupported()
	}
	c, projectPath, err := g.forWrite(ctx, req.Project)
	if err != nil {
		return models.TrackerSprint{}, err
	}
	body := map[string]any{"title": name}
	if !req.Start.IsZero() {
		body["start_date"] = req.Start.Format("2006-01-02")
	}
	if !req.End.IsZero() {
		body["due_date"] = gitlabDueDay(req.Start, req.End).Format("2006-01-02")
	}
	var created gitlabMilestone
	if err := c.gitlab(ctx, http.MethodPost, "/projects/"+gitlabProjectSegment(projectPath)+"/milestones", nil, body, &created); err != nil {
		return models.TrackerSprint{}, err
	}
	if created.ID < 1 {
		return models.TrackerSprint{}, fmt.Errorf("GitLab n'a pas confirmé la création du jalon")
	}
	return gitlabMilestoneSprint(created, time.Now().Format("2006-01-02")), nil
}

func automaticIterationRefusal(n *gitlabIterationNode) error {
	return fmt.Errorf("l'itération %s est planifiée automatiquement par sa cadence : modifiez-la sur GitLab", gitlabIterationName(n.Title, n.StartDate, n.DueDate))
}

// UpdateSprint renames, re-dates, closes or reopens a milestone, or renames
// and re-dates an iteration of a manual cadence. GitLab derives an
// iteration's state from its dates, so an iteration is never closed by hand.
func (g *GitlabAdapter) UpdateSprint(ctx context.Context, project *models.Project, sprintID string, patch models.SprintPatch) (models.TrackerSprint, error) {
	id, err := parseGitlabSprintID(sprintID)
	if err != nil {
		return models.TrackerSprint{}, err
	}
	body := map[string]any{}
	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" {
			return models.TrackerSprint{}, fmt.Errorf("un sprint doit garder un nom")
		}
		body["title"] = name
	}
	startKey, endKey := "start_date", "due_date"
	if id.kind == gitlabIterationKind {
		startKey, endKey = "startDate", "dueDate"
	}
	if patch.Start != nil {
		d, err := gitlabSprintDate(*patch.Start)
		if err != nil {
			return models.TrackerSprint{}, err
		}
		body[startKey] = d
	}
	if patch.End != nil {
		d, err := gitlabSprintDate(*patch.End)
		if err != nil {
			return models.TrackerSprint{}, err
		}
		body[endKey] = d
	}
	if patch.State != nil {
		state := strings.ToLower(strings.TrimSpace(*patch.State))
		if state != "active" && state != "future" && state != "closed" {
			return models.TrackerSprint{}, fmt.Errorf("état de sprint invalide %q", *patch.State)
		}
		if id.kind == gitlabIterationKind {
			return models.TrackerSprint{}, fmt.Errorf("l'état d'une itération GitLab suit ses dates : changez ses dates plutôt que son état")
		}
		body["state_event"] = "activate"
		if state == "closed" {
			body["state_event"] = "close"
		}
	}
	if len(body) == 0 {
		return models.TrackerSprint{}, fmt.Errorf("rien à modifier sur ce sprint")
	}
	c, projectPath, err := g.forWrite(ctx, project)
	if err != nil {
		return models.TrackerSprint{}, err
	}
	if id.kind == gitlabMilestoneKind {
		var updated gitlabMilestone
		if err := c.gitlab(ctx, http.MethodPut, fmt.Sprintf("/projects/%s/milestones/%d", gitlabProjectSegment(projectPath), id.id), nil, body, &updated); err != nil {
			return models.TrackerSprint{}, err
		}
		return gitlabMilestoneSprint(updated, time.Now().Format("2006-01-02")), nil
	}
	loc, err := c.gitlabLocate(ctx, projectPath)
	if err != nil {
		return models.TrackerSprint{}, err
	}
	if !g.iterationsAvailable(ctx, c, loc.group) {
		return models.TrackerSprint{}, iterationsUnsupported()
	}
	current, err := c.gitlabIteration(ctx, id)
	if err != nil {
		return models.TrackerSprint{}, err
	}
	if current == nil {
		return models.TrackerSprint{}, fmt.Errorf("itération GitLab %d introuvable", id.id)
	}
	if current.automatic() {
		return models.TrackerSprint{}, automaticIterationRefusal(current)
	}
	body["groupPath"] = loc.group
	body["id"] = id.iterationGID()
	var out struct {
		UpdateIteration struct {
			Errors    []string            `json:"errors"`
			Iteration gitlabIterationNode `json:"iteration"`
		} `json:"updateIteration"`
	}
	if err := c.gitlabGraphQL(ctx, `mutation($input: UpdateIterationInput!) { updateIteration(input: $input) { errors iteration { `+gitlabIterationFields+` } } }`, map[string]any{"input": body}, &out); err != nil {
		return models.TrackerSprint{}, err
	}
	if err := gitlabMutationErrors("itération non modifiée", out.UpdateIteration.Errors); err != nil {
		return models.TrackerSprint{}, err
	}
	return out.UpdateIteration.Iteration.sprint(), nil
}

// DeleteSprint deletes a milestone or an iteration of a manual cadence. One
// GitLab no longer knows counts as deleted.
func (g *GitlabAdapter) DeleteSprint(ctx context.Context, project *models.Project, sprintID string) error {
	id, err := parseGitlabSprintID(sprintID)
	if err != nil {
		return err
	}
	c, projectPath, err := g.forWrite(ctx, project)
	if err != nil {
		return err
	}
	if id.kind == gitlabMilestoneKind {
		err := c.gitlab(ctx, http.MethodDelete, fmt.Sprintf("/projects/%s/milestones/%d", gitlabProjectSegment(projectPath), id.id), nil, nil, nil)
		var httpErr *HTTPError
		if errors.As(err, &httpErr) && httpErr.Status == http.StatusNotFound {
			return nil
		}
		return err
	}
	loc, err := c.gitlabLocate(ctx, projectPath)
	if err != nil {
		return err
	}
	if !g.iterationsAvailable(ctx, c, loc.group) {
		return iterationsUnsupported()
	}
	current, err := c.gitlabIteration(ctx, id)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	if current.automatic() {
		return automaticIterationRefusal(current)
	}
	var out struct {
		IterationDelete struct {
			Errors []string `json:"errors"`
		} `json:"iterationDelete"`
	}
	if err := c.gitlabGraphQL(ctx, `mutation($input: IterationDeleteInput!) { iterationDelete(input: $input) { errors } }`, map[string]any{"input": map[string]any{"id": id.iterationGID()}}, &out); err != nil {
		return err
	}
	return gitlabMutationErrors("itération non supprimée", out.IterationDelete.Errors)
}
