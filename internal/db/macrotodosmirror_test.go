package db

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/testsqlite"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
)

// markedTracker is a Jira site reduced to the comment Sectile owns on an epic:
// it records every write and answers them from a scripted list of failures.
type markedTracker struct {
	tracker.BaseTicketingSystem
	mu     sync.Mutex
	writes []tracker.UpsertMarkedCommentRequest
	actors []string
	// failures are returned by the next writes, one each, before they succeed.
	failures []error
	created  int
}

func newMarkedTracker() *markedTracker {
	return &markedTracker{BaseTicketingSystem: tracker.BaseTicketingSystem{
		TrackerName:  "jira",
		Capabilities: []tracker.Capability{tracker.CapComment, tracker.CapEpic, tracker.CapUpdate, tracker.CapLabels, tracker.CapCreate},
	}}
}

func (f *markedTracker) UpsertMarkedComment(ctx context.Context, req tracker.UpsertMarkedCommentRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes = append(f.writes, req)
	f.actors = append(f.actors, tracker.ActingUser(ctx))
	if len(f.failures) > 0 {
		err := f.failures[0]
		f.failures = f.failures[1:]
		return "", err
	}
	if req.CommentID != "" {
		return req.CommentID, nil
	}
	return "c-1", nil
}

// CreateIssue creates the story of a slicing line, numbered from PE-100.
func (f *markedTracker) CreateIssue(ctx context.Context, req tracker.CreateIssueRequest) (*models.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created++
	key := fmt.Sprintf("PE-%d", 99+f.created)
	return &models.Task{ProjectID: req.Project.ID, Key: key, Title: req.Title, ParentKey: req.ParentKey}, nil
}

func (f *markedTracker) written() []tracker.UpsertMarkedCommentRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]tracker.UpsertMarkedCommentRequest{}, f.writes...)
}

// fastTodosMirror shortens the debounce and the retry pauses for one test.
func fastTodosMirror(t *testing.T) {
	t.Helper()
	delay, pauses := todosMirrorDelay, todosMirrorPauses
	todosMirrorDelay = 20 * time.Millisecond
	todosMirrorPauses = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { todosMirrorDelay, todosMirrorPauses = delay, pauses })
}

func mirrorDB(t *testing.T) *DB {
	t.Helper()
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), NewDB)
	if err != nil {
		t.Fatalf("database not initialised: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func jiraMirrorProject(t *testing.T) (*DB, *models.Project, *markedTracker) {
	t.Helper()
	fastTodosMirror(t)
	database := mirrorDB(t)
	proj, err := database.CreateProject(models.CreateProjectRequest{Name: "Platform", Slug: "platform", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}
	fake := newMarkedTracker()
	database.TrackerRegistry().Register("jira", fake)
	return database, proj, fake
}

func todosOf(texts ...string) []models.MacroTodo {
	out := make([]models.MacroTodo, 0, len(texts))
	for _, text := range texts {
		out = append(out, models.MacroTodo{Text: text})
	}
	return out
}

func as(user string) context.Context {
	return tracker.WithActingUser(context.Background(), user)
}

func TestTodosMirrorEligibility(t *testing.T) {
	database, jira, _ := jiraMirrorProject(t)
	jira.RoadmapProjects = []string{"DS"}
	github := &models.Project{ID: "gh", IssueTracker: "github", GithubRepo: "acme/app"}
	gitlab := &models.Project{ID: "gl", IssueTracker: "gitlab", GithubRepo: "acme/app"}
	local := &models.Project{ID: "lo", IssueTracker: "local"}
	cases := []struct {
		proj       *models.Project
		milestones map[string]bool
		key        string
		kind       string
		reason     string
	}{
		{jira, nil, "PE-12", models.MacroTodosMirrorJiraComment, ""},
		{jira, nil, "DS-4", "", "autre projet Jira"},
		{jira, nil, "M-3", "", "sans épic Jira"},
		{github, nil, "M-3", models.MacroTodosMirrorGithubDescription, ""},
		{github, map[string]bool{"M-3": true}, "M-3", models.MacroTodosMirrorGithubDescription, ""},
		{github, map[string]bool{"M-3": true}, "M-4", "", "n'existe pas"},
		{github, nil, "#12", "", "sans milestone"},
		{gitlab, nil, "M-1", "", "GitLab"},
		{local, nil, "M-1", "", "projet local"},
	}
	for _, c := range cases {
		kind, reason := database.todosMirrorScope(c.proj, c.milestones).eligibility(c.key)
		if kind != c.kind || !strings.Contains(reason, c.reason) || (c.kind == "" && reason == "") {
			t.Errorf("%s %s: kind %q reason %q, want %q containing %q", c.proj.IssueTracker, c.key, kind, reason, c.kind, c.reason)
		}
	}
}

func TestTodosMirrorRefusesATrackerWithoutMarkedComments(t *testing.T) {
	database := mirrorDB(t)
	proj, err := database.CreateProject(models.CreateProjectRequest{Name: "Plain", Slug: "plain", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}
	database.TrackerRegistry().Register("jira", &tracker.BaseTicketingSystem{TrackerName: "jira", Capabilities: []tracker.Capability{tracker.CapComment}})
	if kind, reason := database.todosMirrorScope(proj, nil).eligibility("PE-1"); kind != "" || reason == "" {
		t.Fatalf("kind %q reason %q", kind, reason)
	}
}

func TestRenderTodosMirrorPerKind(t *testing.T) {
	todos := []models.MacroTodo{
		{Text: "Import the priorities", StoryKey: "PE-41"},
		{Text: "Show the\nquarter", Done: true},
		{Text: "Remove the column"},
	}
	jira := renderTodosMirror(models.MacroTodosMirrorJiraComment, todos)
	for _, want := range []string{"### 📋 [Sectile] Todos", "1. ☐ Import the priorities - PE-41", "2. ☑ Show the quarter", "3. ☐ Remove the column", "Liste tenue dans Sectile"} {
		if !strings.Contains(jira, want) {
			t.Errorf("Jira body misses %q:\n%s", want, jira)
		}
	}
	if strings.Contains(jira, todosBlockOpen) {
		t.Error("the Jira body carries no block marker")
	}
	github := renderTodosMirror(models.MacroTodosMirrorGithubDescription, todos)
	if !strings.HasPrefix(github, todosBlockOpen+"\n") || !strings.HasSuffix(github, "\n"+todosBlockClose) {
		t.Errorf("the GitHub block must sit between its markers:\n%s", github)
	}
	for _, want := range []string{"1. [ ] Import the priorities - PE-41", "2. [x] Show the quarter"} {
		if !strings.Contains(github, want) {
			t.Errorf("GitHub block misses %q:\n%s", want, github)
		}
	}

	if got := renderTodosMirror(models.MacroTodosMirrorGithubDescription, nil); got != "" {
		t.Errorf("an empty list removes the GitHub block, got %q", got)
	}
	if got := renderTodosMirror(models.MacroTodosMirrorJiraComment, nil); !strings.Contains(got, "Aucun todo") {
		t.Errorf("an empty list says so on Jira, got %q", got)
	}
}

func TestRenderTodosMirrorKeepsWithinTheBudget(t *testing.T) {
	long := strings.Repeat("x", 900)
	todos := make([]models.MacroTodo, 100)
	for i := range todos {
		todos[i] = models.MacroTodo{Text: fmt.Sprintf("%03d %s", i, long)}
	}
	for _, kind := range []string{models.MacroTodosMirrorJiraComment, models.MacroTodosMirrorGithubDescription} {
		body := renderTodosMirror(kind, todos)
		if len(body) > todosMirrorBudget {
			t.Fatalf("%s: %d bytes over the budget", kind, len(body))
		}
		if !strings.Contains(body, "1. ☐ 000") && !strings.Contains(body, "1. [ ] 000") {
			t.Errorf("%s: the first todos are the ones kept", kind)
		}
		if !strings.Contains(body, "autres todos dans Sectile") || strings.Contains(body, " 099 ") {
			t.Errorf("%s: the cut must be said and the last todos left out", kind)
		}
		if kind == models.MacroTodosMirrorGithubDescription && !strings.HasSuffix(body, todosBlockClose) {
			t.Errorf("the truncated block keeps its closing marker")
		}
	}
}

func TestSplitAndJoinTodosBlock(t *testing.T) {
	block := todosBlockOpen + "\nlist\n" + todosBlockClose
	cases := []struct {
		description, outside, block string
	}{
		{"Notes", "Notes", ""},
		{"", "", ""},
		{"Notes\n\n" + block, "Notes", block},
		{block, "", block},
		{"Notes\n\n" + block + "\n\nAdded below", "Notes\n\nAdded below", block},
		// An opening marker without its closing one: Sectile owns the rest.
		{"Notes\n\n" + todosBlockOpen + "\nhalf", "Notes", todosBlockOpen + "\nhalf"},
	}
	for _, c := range cases {
		outside, got := splitTodosBlock(c.description)
		if outside != c.outside || got != c.block {
			t.Errorf("split(%q) = %q, %q; want %q, %q", c.description, outside, got, c.outside, c.block)
		}
	}
	if got := joinTodosBlock("Notes\n", block); got != "Notes\n\n"+block {
		t.Errorf("join = %q", got)
	}
	if got := joinTodosBlock("", block); got != block {
		t.Errorf("an empty description holds the block only: %q", got)
	}
	if got := joinTodosBlock("Notes", ""); got != "Notes" {
		t.Errorf("no block leaves the text: %q", got)
	}
	for _, description := range []string{"Notes", "Notes\n\n" + block} {
		before, _ := splitTodosBlock(description)
		outside, b := splitTodosBlock(joinTodosBlock(before, block))
		if b != block || outside != "Notes" {
			t.Errorf("round trip of %q: %q %q", description, outside, b)
		}
	}
}

func TestTodosMirrorStatus(t *testing.T) {
	empty := &models.MacroMeta{Key: "PE-1", Todos: []models.MacroTodo{}}
	if s := todosMirrorStatus(empty, models.MacroTodosMirrorJiraComment, "", todosMirrorState{}); !s.UpToDate {
		t.Error("a list never copied and empty waits for nothing")
	}
	listed := &models.MacroMeta{Key: "PE-1", Todos: todosOf("one"), ExternalURL: "https://site/browse/PE-1"}
	if s := todosMirrorStatus(listed, models.MacroTodosMirrorJiraComment, "", todosMirrorState{}); s.UpToDate {
		t.Error("a list never copied is waiting")
	}
	hash := todosMirrorHash(renderTodosMirror(models.MacroTodosMirrorJiraComment, listed.Todos))
	s := todosMirrorStatus(listed, models.MacroTodosMirrorJiraComment, "", todosMirrorState{ref: "42", hash: hash})
	if !s.UpToDate || s.URL != "https://site/browse/PE-1?focusedCommentId=42" {
		t.Errorf("status %+v", s)
	}
	if s := todosMirrorStatus(listed, "", "projet local", todosMirrorState{}); s.Kind != "" || s.Reason != "projet local" || s.UpToDate {
		t.Errorf("local status %+v", s)
	}
}

func TestTodosMirrorSavesSettleIntoOneWriteOfTheLastList(t *testing.T) {
	database, proj, fake := jiraMirrorProject(t)
	todosMirrorDelay = 150 * time.Millisecond

	for i, list := range [][]models.MacroTodo{todosOf("a"), todosOf("a", "b"), todosOf("b", "a")} {
		user := fmt.Sprintf("user-%d", i)
		if _, err := database.UpdateMacro(as(user), proj.ID, "PE-12", nil, nil, nil, nil, &list, nil); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, "the copy", func() bool { return len(fake.written()) > 0 })
	time.Sleep(200 * time.Millisecond)
	writes := fake.written()
	if len(writes) != 1 {
		t.Fatalf("three saves within the delay make one write, got %d", len(writes))
	}
	if !strings.Contains(writes[0].Body, "1. ☐ b\n2. ☐ a") {
		t.Errorf("the write renders the last list:\n%s", writes[0].Body)
	}
	if writes[0].Marker != todosMirrorMarker || writes[0].Key != "PE-12" || writes[0].CommentID != "" {
		t.Errorf("write %+v", writes[0])
	}
	if fake.actors[0] != "user-2" {
		t.Errorf("the write is signed by the last saver, got %q", fake.actors[0])
	}
	waitFor(t, "the recorded state", func() bool {
		state, _ := database.readTodosMirrorState(proj.ID, "PE-12")
		return state.ref == "c-1" && state.hash != "" && state.at != nil
	})
	macro, err := database.GetMacro(proj.ID, "PE-12")
	if err != nil || macro.TodosMirror == nil || !macro.TodosMirror.UpToDate || macro.TodosMirror.Kind != models.MacroTodosMirrorJiraComment {
		t.Fatalf("macro %+v %v", macro, err)
	}
}

func TestTodosMirrorIsNotScheduledForALocalOnlyMacro(t *testing.T) {
	database, proj, fake := jiraMirrorProject(t)
	proj.RoadmapProjects = []string{"DS"}
	for _, key := range []string{"DS-4", "M-2"} {
		list := todosOf("a")
		if _, err := database.UpdateMacro(as("ada"), proj.ID, key, nil, nil, nil, nil, &list, nil); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(100 * time.Millisecond)
	if n := len(fake.written()); n != 0 {
		t.Fatalf("no tracker call for a macro whose list stays in Sectile, got %d", n)
	}
	if refusal := database.TodosMirrorRefusal(proj.ID, "M-2"); !strings.Contains(refusal, "restent dans Sectile") {
		t.Errorf("refusal %q", refusal)
	}
	if refusal := database.TodosMirrorRefusal(proj.ID, "PE-2"); refusal != "" {
		t.Errorf("an epic of the project is copied, got %q", refusal)
	}
}

func TestPushMacroTodosMirrorRewritesTheSameComment(t *testing.T) {
	database, proj, fake := jiraMirrorProject(t)
	ctx := as("ada")
	list := todosOf("a")
	if _, err := database.SaveMacroMeta(proj.ID, "PE-12", nil, nil, nil, &list); err != nil {
		t.Fatal(err)
	}
	if _, err := database.PushMacroTodosMirror(ctx, proj.ID, "PE-12", false); err != nil {
		t.Fatal(err)
	}
	note, err := database.PushMacroTodosMirror(ctx, proj.ID, "PE-12", false)
	if err != nil || !strings.Contains(note, "déjà à jour") || len(fake.written()) != 1 {
		t.Fatalf("an unchanged list makes no call: %q %v, %d writes", note, err, len(fake.written()))
	}
	list = append(list, models.MacroTodo{Text: "b", Done: true})
	if _, err := database.SaveMacroMeta(proj.ID, "PE-12", nil, nil, nil, &list); err != nil {
		t.Fatal(err)
	}
	if _, err := database.PushMacroTodosMirror(ctx, proj.ID, "PE-12", false); err != nil {
		t.Fatal(err)
	}
	if _, err := database.PushMacroTodosMirror(ctx, proj.ID, "PE-12", true); err != nil {
		t.Fatal(err)
	}
	writes := fake.written()
	if len(writes) != 3 || writes[1].CommentID != "c-1" || writes[2].CommentID != "c-1" {
		t.Fatalf("the remembered comment is rewritten, and republishing forces a write: %+v", writes)
	}
}

func TestPushMacroTodosMirrorRetriesPassingFailures(t *testing.T) {
	database, proj, fake := jiraMirrorProject(t)
	fake.failures = []error{&trackerapi.HTTPError{Status: http.StatusServiceUnavailable}, &trackerapi.HTTPError{Status: http.StatusTooManyRequests}}
	list := todosOf("a")
	if _, err := database.SaveMacroMeta(proj.ID, "PE-12", nil, nil, nil, &list); err != nil {
		t.Fatal(err)
	}
	if _, err := database.PushMacroTodosMirror(as("ada"), proj.ID, "PE-12", false); err != nil {
		t.Fatalf("the third attempt succeeds: %v", err)
	}
	if n := len(fake.written()); n != 3 {
		t.Fatalf("attempts = %d, want 3", n)
	}
}

func TestPushMacroTodosMirrorKeepsTheLastFailureUntilASuccess(t *testing.T) {
	database, proj, fake := jiraMirrorProject(t)
	fake.failures = []error{&trackerapi.MissingPersonalCredentialError{Tracker: "jira"}}
	list := todosOf("a")
	if _, err := database.SaveMacroMeta(proj.ID, "PE-12", nil, nil, nil, &list); err != nil {
		t.Fatal(err)
	}
	_, err := database.PushMacroTodosMirror(as("ada"), proj.ID, "PE-12", false)
	if err == nil || trackerapi.MissingCredentialTracker(err) != "jira" || !strings.Contains(err.Error(), "gardés dans Sectile") {
		t.Fatalf("the refusal keeps the credential chain: %v", err)
	}
	if n := len(fake.written()); n != 1 {
		t.Fatalf("a permanent failure is not retried, %d attempts", n)
	}
	macro, _ := database.GetMacro(proj.ID, "PE-12")
	if m := macro.TodosMirror; m.UpToDate || m.Error == "" || m.CredentialMissing != "jira" {
		t.Fatalf("status after the failure %+v", m)
	}
	if _, err := database.PushMacroTodosMirror(as("ada"), proj.ID, "PE-12", false); err != nil {
		t.Fatal(err)
	}
	macro, _ = database.GetMacro(proj.ID, "PE-12")
	if m := macro.TodosMirror; !m.UpToDate || m.Error != "" || m.CredentialMissing != "" {
		t.Fatalf("a success clears the failure %+v", m)
	}
}

func TestTodosMirrorJobRunsThroughTheQueue(t *testing.T) {
	database, proj, fake := jiraMirrorProject(t)
	list := todosOf("a")
	if _, err := database.SaveMacroMeta(proj.ID, "PE-12", nil, nil, nil, &list); err != nil {
		t.Fatal(err)
	}
	act, err := database.EnqueueTrackerOp(as("ada"), TrackerOp{Kind: TrackerOpEpicTodos, ProjectID: proj.ID, TaskKey: "PE-12", EpicKey: "PE-12"})
	if err != nil || !strings.Contains(act.Action, "Todos de PE-12") {
		t.Fatalf("activity %+v %v", act, err)
	}
	waitFor(t, "the activity", func() bool {
		got, _ := database.GetActivityByID(act.ID)
		return got != nil && got.Status == string(models.ActivityStatusCompleted)
	})
	if len(fake.written()) != 1 || fake.actors[0] != "ada" {
		t.Fatalf("writes %d actors %v", len(fake.written()), fake.actors)
	}
}

func TestEverySaveOfTheTodosSchedulesTheCopy(t *testing.T) {
	database, proj, fake := jiraMirrorProject(t)
	list := todosOf("a")
	if _, err := database.SaveMacroMeta(proj.ID, "PE-12", nil, nil, nil, &list); err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateTask(models.CreateTaskRequest{ProjectID: proj.ID, Title: "Child", ParentKey: "PE-12"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := database.TodosFromMacroStories(as("ada"), proj.ID, "PE-12"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the copy after an import", func() bool { return len(fake.written()) == 1 })
	if !strings.Contains(fake.written()[0].Body, "Child") {
		t.Errorf("the import is copied:\n%s", fake.written()[0].Body)
	}
	// Changing only a target leaves the body as it was: no second write.
	list2, _ := database.GetMacro(proj.ID, "PE-12")
	todos := list2.Todos
	todos[0].TargetTrackerProject = "DS"
	if _, err := database.UpdateMacro(as("ada"), proj.ID, "PE-12", nil, nil, nil, nil, &todos, nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	if n := len(fake.written()); n != 1 {
		t.Fatalf("a save the copy does not show writes nothing, got %d writes", n)
	}
}

func TestSlicingFromTheSpecificationIsCopied(t *testing.T) {
	database, proj, fake := jiraMirrorProject(t)
	framework := "openspec"
	if _, err := database.UpdateProject(proj.ID, models.UpdateProjectRequest{SpecFramework: &framework}); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	database.SetAgentOperations(localSpecReader(repo))
	writeSpecDir(t, repo, "openspec/changes", "pe-440-cloudprober", map[string]string{"tasks.md": tasksFile})
	seedMacro(t, database, proj.ID, "PE-440")

	if _, _, err := database.TodosFromSDD(context.Background(), "ada", proj.ID, "PE-440", SlicingFromTasks); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the copy after the import", func() bool { return len(fake.written()) == 1 })
	if fake.actors[0] != "ada" {
		t.Errorf("the copy is signed by the person who imported, got %q", fake.actors[0])
	}
}

func TestStoryCreationCopiesTheKeyOnTheLine(t *testing.T) {
	database, proj, fake := jiraMirrorProject(t)
	todos := []models.MacroTodo{{ID: "a", Text: "First"}, {ID: "b", Text: "Second"}, {ID: "c", Text: "Third"}}
	if _, err := database.SaveMacroMeta(proj.ID, "PE-12", nil, nil, nil, &todos); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := database.CreateStoryFromMacroTodo(as("ada"), proj.ID, "PE-12", "a"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the copy after one story", func() bool { return len(fake.written()) == 1 })
	if body := fake.written()[0].Body; !strings.Contains(body, "1. ☐ First - PE-100") {
		t.Fatalf("the key shows on the line:\n%s", body)
	}
	batch, err := database.CreateStoriesFromMacroTodos(as("ada"), proj.ID, "PE-12", []string{"b", "c"})
	if err != nil || batch.Created != 2 {
		t.Fatalf("batch %+v %v", batch, err)
	}
	waitFor(t, "the copy after the batch", func() bool { return len(fake.written()) == 2 })
	time.Sleep(100 * time.Millisecond)
	writes := fake.written()
	if len(writes) != 2 || !strings.Contains(writes[1].Body, "3. ☐ Third - PE-102") {
		t.Fatalf("one copy for the whole batch, showing every key: %d writes\n%s", len(writes), writes[len(writes)-1].Body)
	}
}

func TestReplaceMacroTodosMergesTheFullList(t *testing.T) {
	database, proj, _ := jiraMirrorProject(t)
	stored := []models.MacroTodo{
		{ID: "a", Text: "Attached", StoryKey: "PE-41", SourceKind: models.MacroTodoFromTasks, SourceEntry: "1. Attached"},
		{ID: "b", Text: "Dropped"},
		{ID: "c", Text: "Kept"},
	}
	if _, err := database.SaveMacroMeta(proj.ID, "PE-12", nil, nil, nil, &stored); err != nil {
		t.Fatal(err)
	}
	saved, err := database.ReplaceMacroTodos(as("ada"), proj.ID, "PE-12", []MacroTodoInput{
		{ID: "c", Text: " Kept, reworded ", Done: true},
		{Text: "New one", TargetTrackerProject: "ds"},
		{ID: "a", Text: "Attached, reworded"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := saved.Todos
	if len(got) != 3 || got[0].ID != "c" || got[2].ID != "a" || got[1].ID == "" {
		t.Fatalf("order and ids %+v", got)
	}
	if got[0].Text != "Kept, reworded" || !got[0].Done {
		t.Errorf("a known line takes the given text and done: %+v", got[0])
	}
	if got[2].StoryKey != "PE-41" || got[2].SourceKind != models.MacroTodoFromTasks || got[2].SourceEntry != "1. Attached" {
		t.Errorf("a known line keeps its story key and origin: %+v", got[2])
	}
	if got[1].TargetTrackerProject != "DS" {
		t.Errorf("the new line takes its target: %+v", got[1])
	}
	if saved.TodosMirror == nil || saved.TodosMirror.Kind != models.MacroTodosMirrorJiraComment {
		t.Errorf("the answer carries the copy status: %+v", saved.TodosMirror)
	}

	for name, items := range map[string][]MacroTodoInput{
		"blank text":   {{ID: "c", Text: "  "}},
		"unknown id":   {{ID: "zzz", Text: "x"}},
		"repeated id":  {{ID: "c", Text: "x"}, {ID: "c", Text: "y"}},
		"blank second": {{Text: "fine"}, {Text: ""}},
	} {
		if _, err := database.ReplaceMacroTodos(as("ada"), proj.ID, "PE-12", items); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	after, _ := database.GetMacro(proj.ID, "PE-12")
	if len(after.Todos) != 3 || after.Todos[0].Text != "Kept, reworded" {
		t.Fatalf("a refused call saves nothing: %+v", after.Todos)
	}
	if _, err := database.ReplaceMacroTodos(as("ada"), proj.ID, "PE-999", nil); err == nil || !strings.Contains(err.Error(), "introuvable") {
		t.Errorf("an unknown macro is named: %v", err)
	}
	if _, err := database.ReplaceMacroTodos(as("ada"), "nope", "PE-12", nil); err == nil || !strings.Contains(err.Error(), "projet") {
		t.Errorf("an unknown project is named: %v", err)
	}
}

// githubMilestoneSite is a repository with one milestone, whose description
// the test reads back.
type githubMilestoneSite struct {
	mu          sync.Mutex
	description string
	patches     int
}

func (s *githubMilestoneSite) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		milestone := func() string {
			raw, _ := json.Marshal(map[string]any{"number": 3, "title": "Q4", "description": s.description, "state": "open", "html_url": "https://github.com/acme/app/milestone/3"})
			return string(raw)
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /repos/acme/app/milestones":
			fmt.Fprint(w, "["+milestone()+"]")
		case "GET /repos/acme/app/milestones/3":
			fmt.Fprint(w, milestone())
		case "PATCH /repos/acme/app/milestones/3":
			raw, _ := io.ReadAll(r.Body)
			var payload map[string]any
			_ = json.Unmarshal(raw, &payload)
			if desc, ok := payload["description"].(string); ok {
				s.description = desc
			}
			s.patches++
			fmt.Fprint(w, milestone())
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func (s *githubMilestoneSite) current() (string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.description, s.patches
}

func githubMirrorProject(t *testing.T, description string) (*DB, *models.Project, *githubMilestoneSite) {
	t.Helper()
	fastTodosMirror(t)
	database := mirrorDB(t)
	proj, err := database.CreateProject(models.CreateProjectRequest{Name: "App", Slug: "app", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	site := &githubMilestoneSite{description: description}
	server := httptest.NewServer(site.handler(t))
	t.Cleanup(server.Close)
	database.trackers = &trackerapi.Client{GithubURL: server.URL, GithubToken: "test", HTTP: server.Client()}
	return database, proj, site
}

func TestGithubTodosBlockLivesBesideTheDescription(t *testing.T) {
	database, proj, site := githubMirrorProject(t, "Written on GitHub")
	ctx := tracker.WithUnattended(context.Background())

	// The first read imports the description without any block.
	macros, err := database.GetProjectMacros(proj.ID)
	if err != nil || len(macros) != 1 || macros[0].Description != "Written on GitHub" {
		t.Fatalf("macros %+v %v", macros, err)
	}
	list := todosOf("a", "b")
	if _, err := database.SaveMacroMeta(proj.ID, "M-3", nil, nil, nil, &list); err != nil {
		t.Fatal(err)
	}
	if _, err := database.PushMacroTodosMirror(ctx, proj.ID, "M-3", false); err != nil {
		t.Fatal(err)
	}
	desc, _ := site.current()
	outside, block := splitTodosBlock(desc)
	if outside != "Written on GitHub" || !strings.Contains(block, "1. [ ] a\n2. [ ] b") {
		t.Fatalf("the block is appended after the text:\n%s", desc)
	}

	// A description edit in Sectile keeps the block of the current list.
	newDesc := "Edited in Sectile"
	if _, err := database.UpdateMacro(ctx, proj.ID, "M-3", nil, nil, &newDesc, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	desc, _ = site.current()
	if outside, b := splitTodosBlock(desc); outside != newDesc || b != block {
		t.Fatalf("the description edit erased the block:\n%s", desc)
	}
	// The block is not read back into the macro's description.
	macros, _ = database.GetProjectMacros(proj.ID)
	if macros[0].Description != newDesc || !macros[0].TodosMirror.UpToDate {
		t.Fatalf("macro after the edit %+v", macros[0])
	}

	// An emptied list removes the block and keeps the text.
	empty := []models.MacroTodo{}
	if _, err := database.SaveMacroMeta(proj.ID, "M-3", nil, nil, nil, &empty); err != nil {
		t.Fatal(err)
	}
	if _, err := database.PushMacroTodosMirror(ctx, proj.ID, "M-3", false); err != nil {
		t.Fatal(err)
	}
	if desc, _ = site.current(); desc != newDesc {
		t.Fatalf("an empty list removes the block, got:\n%s", desc)
	}
}

func TestGithubTodosBlockAloneUnderAnEmptyDescription(t *testing.T) {
	database, proj, site := githubMirrorProject(t, "")
	ctx := tracker.WithUnattended(context.Background())
	list := todosOf("a")
	if _, err := database.SaveMacroMeta(proj.ID, "M-3", nil, nil, nil, &list); err != nil {
		t.Fatal(err)
	}
	empty := ""
	if _, err := database.UpdateMacro(ctx, proj.ID, "M-3", nil, nil, &empty, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	desc, patches := site.current()
	if !strings.HasPrefix(desc, todosBlockOpen) || patches != 1 {
		t.Fatalf("the milestone holds the block only:\n%s (%d patches)", desc, patches)
	}
	// The description push recorded the block: the queued copy writes nothing.
	if note, err := database.PushMacroTodosMirror(ctx, proj.ID, "M-3", false); err != nil || !strings.Contains(note, "déjà à jour") {
		t.Fatalf("note %q %v", note, err)
	}
	if _, patches = site.current(); patches != 1 {
		t.Fatalf("no redundant write, got %d patches", patches)
	}
}
