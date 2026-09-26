package db

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"tasks/internal/models"
)

// viewFixture is a board with two projects synchronising overlapping tickets
// and a third that no view below selects.
type viewFixture struct {
	db               *DB
	alpha, beta, gam string
}

func newViewFixture(t *testing.T) viewFixture {
	t.Helper()
	database, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	return seedViewFixture(t, database)
}

// seedViewFixture fills any engine's database, so that the PostgreSQL smoke
// test checks the same selection as the SQLite suite.
func seedViewFixture(t *testing.T, database *DB) viewFixture {
	t.Helper()
	f := viewFixture{db: database}
	for _, target := range []struct {
		name, slug string
		id         *string
	}{{"Alpha", "alpha", &f.alpha}, {"Beta", "beta", &f.beta}, {"Gamma", "gamma", &f.gam}} {
		p, err := database.CreateProject(models.CreateProjectRequest{Name: target.name, Slug: target.slug})
		if err != nil {
			t.Fatalf("CreateProject(%s): %v", target.name, err)
		}
		*target.id = p.ID
	}

	imported := []models.Task{
		{ProjectID: f.alpha, Key: "#1", Title: "alpha backend", Labels: []string{"Backend"}},
		{ProjectID: f.alpha, Key: "#2", Title: "alpha backend-api", Labels: []string{"backend-api"}},
		{ProjectID: f.alpha, Key: "#3", Title: "alpha ui", Labels: []string{"ui", "sprint_1"}},
		{ProjectID: f.alpha, Key: "#42", Title: "shared story", Labels: []string{"platform"}},
		{ProjectID: f.beta, Key: "#42", Title: "shared story", Labels: []string{"platform"}},
		{ProjectID: f.beta, Key: "#5", Title: "beta api", Labels: []string{"api-backend"}, Assignee: "sam"},
		{ProjectID: f.beta, Key: "#6", Title: "beta unlabelled"},
		{ProjectID: f.gam, Key: "#7", Title: "gamma backend", Labels: []string{"backend"}},
		{ProjectID: f.beta, Key: "#10", Title: "beta equipe", Labels: []string{"Équipe"}},
	}
	for i := range imported {
		imported[i].Status = models.StatusToClarify
		imported[i].Priority = models.PriorityMedium
	}
	if err := database.ImportOrUpdateTasks(imported); err != nil {
		t.Fatalf("ImportOrUpdateTasks: %v", err)
	}
	return f
}

func (f viewFixture) create(t *testing.T, user, name string, projects, labels []string) *models.BoardView {
	t.Helper()
	view, err := f.db.CreateBoardView(user, models.BoardViewRequest{Name: &name, ProjectIDs: &projects, Labels: &labels})
	if err != nil {
		t.Fatalf("CreateBoardView(%q): %v", name, err)
	}
	return view
}

func (f viewFixture) titles(t *testing.T, user, viewID string, assignee string) []string {
	t.Helper()
	tasks, err := f.db.GetTasksInScope(TaskScope{UserID: user, ViewID: viewID}, "", "", "", "", "", "", assignee, "", nil, nil, false)
	if err != nil {
		t.Fatalf("GetTasksInScope: %v", err)
	}
	titles := []string{}
	for _, task := range tasks {
		titles = append(titles, task.Title+"@"+task.ProjectID)
	}
	slices.Sort(titles)
	return titles
}

func TestBoardViewRoundTripAndOrder(t *testing.T) {
	f := newViewFixture(t)

	first := f.create(t, "u1", "  Platform  ", []string{f.alpha, "beta", f.alpha}, []string{" platform ", "", "Platform", "ops"})
	second := f.create(t, "u1", "Second", []string{f.gam}, nil)

	if first.Name != "Platform" {
		t.Errorf("name = %q, want it trimmed", first.Name)
	}
	if want := []string{f.alpha, f.beta}; !slices.Equal(first.ProjectIDs, want) {
		t.Errorf("projects = %v, want the slug resolved and duplicates dropped: %v", first.ProjectIDs, want)
	}
	if want := []string{"platform", "ops"}; !slices.Equal(first.Labels, want) {
		t.Errorf("labels = %v, want %v (trimmed, empties dropped, first spelling kept)", first.Labels, want)
	}
	if !slices.Equal(second.Labels, []string{}) {
		t.Errorf("a view without label stores an empty list, got %#v", second.Labels)
	}

	views, err := f.db.ListBoardViews("u1")
	if err != nil {
		t.Fatalf("ListBoardViews: %v", err)
	}
	if len(views) != 2 || views[0].ID != first.ID || views[1].ID != second.ID {
		t.Fatalf("views = %+v, want creation order", views)
	}
	got, err := f.db.GetBoardView("u1", first.ID)
	if err != nil {
		t.Fatalf("GetBoardView: %v", err)
	}
	if got.Name != "Platform" || !slices.Equal(got.ProjectIDs, first.ProjectIDs) || !slices.Equal(got.Labels, first.Labels) {
		t.Errorf("GetBoardView = %+v, want %+v", got, first)
	}
}

func TestBoardViewValidation(t *testing.T) {
	f := newViewFixture(t)
	f.create(t, "u1", "Platform", []string{f.alpha}, nil)

	name := func(s string) *string { return &s }
	projects := func(ids ...string) *[]string { return &ids }
	cases := []struct {
		label string
		req   models.BoardViewRequest
		want  error
	}{
		{"empty name", models.BoardViewRequest{Name: name("   "), ProjectIDs: projects(f.alpha)}, ErrBoardViewNameRequired},
		{"same name, other case and spacing", models.BoardViewRequest{Name: name(" platform "), ProjectIDs: projects(f.alpha)}, ErrBoardViewNameTaken},
		{"no project", models.BoardViewRequest{Name: name("Other"), ProjectIDs: projects()}, ErrBoardViewNoProject},
		{"unknown project", models.BoardViewRequest{Name: name("Other"), ProjectIDs: projects("nope")}, ErrBoardViewUnknownProject},
	}
	for _, c := range cases {
		if _, err := f.db.CreateBoardView("u1", c.req); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.label, err, c.want)
		}
	}

	// Names are unique per owner, not across the board.
	f.create(t, "u2", "Platform", []string{f.alpha}, nil)
}

func TestBoardViewUpdatePatchesOnlyWhatIsSent(t *testing.T) {
	f := newViewFixture(t)
	view := f.create(t, "u1", "Platform", []string{f.alpha, f.beta}, []string{"platform"})
	f.create(t, "u1", "Other", []string{f.alpha}, nil)

	projects := []string{f.alpha}
	updated, err := f.db.UpdateBoardView("u1", view.ID, models.BoardViewRequest{ProjectIDs: &projects})
	if err != nil {
		t.Fatalf("UpdateBoardView: %v", err)
	}
	if updated.Name != "Platform" || !slices.Equal(updated.Labels, []string{"platform"}) || !slices.Equal(updated.ProjectIDs, projects) {
		t.Errorf("updated = %+v, want only the projects changed", updated)
	}

	sameName := "PLATFORM"
	if _, err := f.db.UpdateBoardView("u1", view.ID, models.BoardViewRequest{Name: &sameName}); err != nil {
		t.Errorf("renaming a view to a new case of its own name: %v", err)
	}
	clash := "other"
	if _, err := f.db.UpdateBoardView("u1", view.ID, models.BoardViewRequest{Name: &clash}); !errors.Is(err, ErrBoardViewNameTaken) {
		t.Errorf("renaming onto another view's name: err = %v, want ErrBoardViewNameTaken", err)
	}
}

func TestBoardViewOfAnotherUserDoesNotExist(t *testing.T) {
	f := newViewFixture(t)
	view := f.create(t, "owner", "Platform", []string{f.alpha}, nil)

	if _, err := f.db.GetBoardView("intruder", view.ID); !errors.Is(err, ErrBoardViewNotFound) {
		t.Errorf("get: err = %v, want ErrBoardViewNotFound", err)
	}
	name := "Stolen"
	if _, err := f.db.UpdateBoardView("intruder", view.ID, models.BoardViewRequest{Name: &name}); !errors.Is(err, ErrBoardViewNotFound) {
		t.Errorf("update: err = %v, want ErrBoardViewNotFound", err)
	}
	if err := f.db.DeleteBoardView("intruder", view.ID); !errors.Is(err, ErrBoardViewNotFound) {
		t.Errorf("delete: err = %v, want ErrBoardViewNotFound", err)
	}
	if _, err := f.db.GetTasksInScope(TaskScope{UserID: "intruder", ViewID: view.ID}, "", "", "", "", "", "", "", "", nil, nil, false); !errors.Is(err, ErrBoardViewNotFound) {
		t.Errorf("tasks: err = %v, want ErrBoardViewNotFound", err)
	}
	if _, err := f.db.GetTaskFacetsInScope(TaskScope{UserID: "intruder", ViewID: view.ID}); !errors.Is(err, ErrBoardViewNotFound) {
		t.Errorf("facets: err = %v, want ErrBoardViewNotFound", err)
	}
	if views, _ := f.db.ListBoardViews("intruder"); len(views) != 0 {
		t.Errorf("intruder lists %d views, want none", len(views))
	}

	if err := f.db.DeleteBoardView("owner", view.ID); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	if _, err := f.db.GetBoardView("owner", view.ID); !errors.Is(err, ErrBoardViewNotFound) {
		t.Errorf("after delete: err = %v, want ErrBoardViewNotFound", err)
	}
}

func TestBoardViewSelection(t *testing.T) {
	checkBoardViewSelection(t, newViewFixture(t))
}

func checkBoardViewSelection(t *testing.T, f viewFixture) {
	t.Helper()
	a, b := "@"+f.alpha, "@"+f.beta

	cases := []struct {
		name     string
		projects []string
		labels   []string
		assignee string
		want     []string
	}{
		{
			name:     "whole label, any case, never a substring",
			projects: []string{f.alpha, f.beta},
			labels:   []string{"BACKEND"},
			want:     []string{"alpha backend" + a},
		},
		{
			name:     "any of the labels",
			projects: []string{f.alpha, f.beta},
			labels:   []string{"ui", "api-backend"},
			want:     []string{"alpha ui" + a, "beta api" + b},
		},
		{
			name:     "a LIKE wildcard in a label is literal",
			projects: []string{f.alpha},
			labels:   []string{"sprint_1"},
			want:     []string{"alpha ui" + a},
		},
		{
			name:     "no label selects every ticket of the projects, and only them",
			projects: []string{f.beta},
			want:     []string{"beta api" + b, "beta equipe" + b, "beta unlabelled" + b, "shared story" + b},
		},
		{
			name:     "a remote story synchronised by two projects shows twice",
			projects: []string{f.alpha, f.beta},
			labels:   []string{"platform"},
			want:     []string{"shared story" + a, "shared story" + b},
		},
		{
			// Neither engine folds É: SQLite's LOWER cannot, and PostgreSQL's
			// is asked not to, so that a cluster's collation cannot change what
			// a view selects. The label matches its own spelling everywhere.
			name:     "an accented label matches its own spelling",
			projects: []string{f.alpha, f.beta},
			labels:   []string{"Équipe"},
			want:     []string{"beta equipe" + b},
		},
		{
			// The other half of the same promise: case is folded for ASCII
			// letters only. A PostgreSQL cluster whose LOWER folds `É` to `é`
			// would otherwise select the ticket here and nowhere else.
			name:     "an accented letter is not folded, whatever the engine",
			projects: []string{f.alpha, f.beta},
			labels:   []string{"équipe"},
			want:     []string{},
		},
		{
			name:     "board filters narrow the view",
			projects: []string{f.alpha, f.beta},
			assignee: "sam",
			want:     []string{"beta api" + b},
		},
	}
	for i, c := range cases {
		view := f.create(t, "u1", "view-"+string(rune('a'+i)), c.projects, c.labels)
		slices.Sort(c.want)
		if got := f.titles(t, "u1", view.ID, c.assignee); !slices.Equal(got, c.want) {
			t.Errorf("%s:\n got  %v\n want %v", c.name, got, c.want)
		}
	}
}

func TestBoardViewSelectionFollowsLabelChanges(t *testing.T) {
	f := newViewFixture(t)
	view := f.create(t, "u1", "Platform", []string{f.alpha, f.beta}, []string{"platform"})

	if err := f.db.ImportOrUpdateTasks([]models.Task{{
		ProjectID: f.beta, Key: "#6", Title: "beta unlabelled", Labels: []string{"Platform"},
		Status: models.StatusToClarify, Priority: models.PriorityMedium,
	}}); err != nil {
		t.Fatalf("ImportOrUpdateTasks: %v", err)
	}
	if got := f.titles(t, "u1", view.ID, ""); !slices.Contains(got, "beta unlabelled@"+f.beta) {
		t.Errorf("a ticket that gained the label is missing: %v", got)
	}
}

func TestBoardViewFacetsDescribeTheView(t *testing.T) {
	f := newViewFixture(t)
	view := f.create(t, "u1", "Platform", []string{f.alpha, f.beta}, []string{"platform", "api-backend"})

	facets, err := f.db.GetTaskFacetsInScope(TaskScope{UserID: "u1", ViewID: view.ID})
	if err != nil {
		t.Fatalf("GetTaskFacetsInScope: %v", err)
	}
	if facets.Total != 3 {
		t.Errorf("total = %d, want the three tickets of the view", facets.Total)
	}
	labels := map[string]int{}
	for _, l := range facets.Labels {
		labels[l.Value] = l.Count
	}
	if labels["platform"] != 2 || labels["api-backend"] != 1 || labels["Backend"] != 0 || labels["backend"] != 0 {
		t.Errorf("label facets = %v, want only the view's tickets counted", labels)
	}
	if !slices.Equal(facets.Assignees, []string{"sam"}) {
		t.Errorf("assignees = %v, want [sam]", facets.Assignees)
	}
}

func TestDeletingAProjectRemovesItFromViews(t *testing.T) {
	f := newViewFixture(t)
	both := f.create(t, "u1", "Both", []string{f.alpha, f.beta}, nil)
	only := f.create(t, "u1", "Only beta", []string{f.beta}, nil)

	if err := f.db.DeleteProject(f.beta); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	got, err := f.db.GetBoardView("u1", both.ID)
	if err != nil {
		t.Fatalf("GetBoardView: %v", err)
	}
	if !slices.Equal(got.ProjectIDs, []string{f.alpha}) {
		t.Errorf("projects = %v, want the deleted one dropped", got.ProjectIDs)
	}

	emptied, err := f.db.GetBoardView("u1", only.ID)
	if err != nil {
		t.Fatalf("a view left without project must stay: %v", err)
	}
	if len(emptied.ProjectIDs) != 0 {
		t.Errorf("projects = %v, want none", emptied.ProjectIDs)
	}
	if tasks := f.titles(t, "u1", only.ID, ""); len(tasks) != 0 {
		t.Errorf("a view without project shows %v, want nothing", tasks)
	}
}

// The rules the board applies everywhere keep applying inside a view, and the
// existing filters narrow it: containers stay hidden unless asked for, and the
// substring label filter combines with the view's whole-label rule.
func TestBoardViewKeepsTheBoardRules(t *testing.T) {
	f := newViewFixture(t)
	if err := f.db.ImportOrUpdateTasks([]models.Task{{
		ProjectID: f.alpha, Key: "#99", Title: "platform epic", Labels: []string{"platform"}, IssueType: "Epic",
		Status: models.StatusToClarify, Priority: models.PriorityMedium,
	}}); err != nil {
		t.Fatalf("ImportOrUpdateTasks: %v", err)
	}
	view := f.create(t, "u1", "Platform", []string{f.alpha, f.beta}, []string{"platform", "ui"})
	scope := TaskScope{UserID: "u1", ViewID: view.ID}

	titles := func(label string, issueTypes []string) []string {
		t.Helper()
		tasks, err := f.db.GetTasksInScope(scope, "", "", "", label, "", "", "", "", nil, issueTypes, false)
		if err != nil {
			t.Fatalf("GetTasksInScope: %v", err)
		}
		out := []string{}
		for _, task := range tasks {
			out = append(out, task.Title+"@"+task.ProjectID)
		}
		slices.Sort(out)
		return out
	}

	if got := titles("", nil); slices.Contains(got, "platform epic@"+f.alpha) {
		t.Errorf("a container shows by default: %v", got)
	}
	if got := titles("", []string{"Epic"}); !slices.Equal(got, []string{"platform epic@" + f.alpha}) {
		t.Errorf("asking for epics = %v, want the epic only", got)
	}
	want := []string{"alpha ui@" + f.alpha}
	if got := titles("u", nil); !slices.Equal(got, want) {
		t.Errorf("label filter inside the view = %v, want %v", got, want)
	}
}

func TestBoardViewSizeIsBounded(t *testing.T) {
	f := newViewFixture(t)
	labels := make([]string, 0, maxBoardViewLabels+1)
	for i := 0; i <= maxBoardViewLabels; i++ {
		labels = append(labels, "label-"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	name, projects := "Too many", []string{f.alpha}
	if _, err := f.db.CreateBoardView("u1", models.BoardViewRequest{Name: &name, ProjectIDs: &projects, Labels: &labels}); !errors.Is(err, ErrBoardViewTooLarge) {
		t.Errorf("%d labels: err = %v, want ErrBoardViewTooLarge", len(labels), err)
	}
	long := strings.Repeat("é", maxBoardViewNameLength+1)
	if _, err := f.db.CreateBoardView("u1", models.BoardViewRequest{Name: &long, ProjectIDs: &projects}); !errors.Is(err, ErrBoardViewTooLarge) {
		t.Errorf("a %d-character name: err = %v, want ErrBoardViewTooLarge", maxBoardViewNameLength+1, err)
	}
	fits := strings.Repeat("é", maxBoardViewNameLength)
	f.create(t, "u1", fits, projects, labels[:maxBoardViewLabels])
}

func TestDeletingAUserDeletesTheirViews(t *testing.T) {
	f := newViewFixture(t)
	if _, err := f.db.SignInLocal("admin@example.com"); err != nil {
		t.Fatal(err)
	}
	member, err := f.db.SignInLocal("member@example.com")
	if err != nil {
		t.Fatal(err)
	}
	f.create(t, member.ID, "Mine", []string{f.alpha}, nil)
	if err := f.db.DeleteUser(member.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	var left int
	if err := f.db.conn.QueryRow("SELECT COUNT(*) FROM board_views WHERE user_id = ?", member.ID).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d views outlived their owner", left)
	}
}

// TestViewLabelPredicateFoldsASCIIOnly pins the SQL itself, on both engines and
// without a server. The fold has to be the same on either side of the
// comparison — asciiLower on the value, LowerASCII on the column — and
// PostgreSQL's has to name a collation, otherwise the very same query selects
// differently on two clusters (docs/adrs/0025).
func TestViewLabelPredicateFoldsASCIIOnly(t *testing.T) {
	cases := []struct {
		engine  string
		lowered string
		want    string
	}{
		{"SQLite", sqliteDialect{}.LowerASCII("labels"), "(LOWER(labels) LIKE ? ESCAPE '!')"},
		{"PostgreSQL", postgresDialect{}.LowerASCII("labels"), `(LOWER(labels COLLATE "C") LIKE ? ESCAPE '!')`},
	}
	for _, c := range cases {
		cond, args := viewLabelScope([]string{"Équipe"}, c.lowered)
		if cond != c.want {
			t.Errorf("%s condition = %q, want %q", c.engine, cond, c.want)
		}
		if len(args) != 1 || args[0] != `%"Équipe"%` {
			t.Errorf("%s args = %#v, want the label with its accent kept", c.engine, args)
		}
	}
	// The quoted collation name must not disturb the placeholder rewriting.
	cond, _ := viewLabelScope([]string{"ui", "ops"}, postgresDialect{}.LowerASCII("labels"))
	if got, want := rebindNumbered(cond), `(LOWER(labels COLLATE "C") LIKE $1 ESCAPE '!' OR LOWER(labels COLLATE "C") LIKE $2 ESCAPE '!')`; got != want {
		t.Errorf("rebound = %q, want %q", got, want)
	}
}
