package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// storyTracker is a Jira that creates stories and records the parents it is
// asked to write, with a refusal to switch on.
type storyTracker struct {
	*fakeTracker
	mu        sync.Mutex
	created   []string // project key of each created story
	parents   map[string]string
	parentErr error
	next      int
}

func newStoryTracker() *storyTracker {
	fake := newFakeTracker()
	fake.Capabilities = append(fake.Capabilities, tracker.CapCreate)
	return &storyTracker{fakeTracker: fake, parents: map[string]string{}}
}

func (s *storyTracker) CreateIssue(ctx context.Context, req tracker.CreateIssueRequest) (*models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	s.created = append(s.created, req.Project.JiraProject)
	key := fmt.Sprintf("%s-%d", req.Project.JiraProject, s.next)
	return &models.Task{ID: key, Key: key, Title: req.Title, Source: "jira"}, nil
}

func (s *storyTracker) SetParent(ctx context.Context, key, parentKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.parentErr != nil {
		return s.parentErr
	}
	s.parents[key] = parentKey
	return nil
}

// jiraStoryDB is a database with two Jira projects on one site and one on
// another, and a macro PE-100 in the first carrying one line.
func jiraStoryDB(t *testing.T, targetID func(sameSite, otherSite *models.Project) string) (*DB, *storyTracker, *models.Project, string) {
	t.Helper()
	fake := newStoryTracker()
	database, macroProject := jiraTestDB(t, fake.fakeTracker)
	database.TrackerRegistry().Register("jira", fake)
	site := "https://equativ.atlassian.net"
	if _, err := database.UpdateProject(macroProject.ID, models.UpdateProjectRequest{TrackerUrl: &site}); err != nil {
		t.Fatal(err)
	}
	sameSite, err := database.CreateProject(models.CreateProjectRequest{Name: "Data", Slug: "data", IssueTracker: "jira", JiraProject: "DATA", TrackerUrl: "https://EQUATIV.atlassian.net/"})
	if err != nil {
		t.Fatal(err)
	}
	otherSite, err := database.CreateProject(models.CreateProjectRequest{Name: "Elsewhere", Slug: "elsewhere", IssueTracker: "jira", JiraProject: "EL", TrackerUrl: "https://other.atlassian.net"})
	if err != nil {
		t.Fatal(err)
	}
	todos := []models.MacroTodo{{ID: "line", Text: "Ship it", TargetProjectID: targetID(sameSite, otherSite)}}
	if _, err := database.SaveMacroMeta(macroProject.ID, "PE-100", nil, nil, nil, &todos); err != nil {
		t.Fatal(err)
	}
	return database, fake, macroProject, "line"
}

func TestSameTrackerInstanceFixtures(t *testing.T) {
	database, err := NewDB(t.TempDir() + "/tasks.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	jira := func(id, site string) *models.Project {
		return &models.Project{ID: id, Name: id, IssueTracker: "jira", TrackerUrl: site}
	}
	github := func(id, repo, api string) *models.Project {
		return &models.Project{ID: id, Name: id, IssueTracker: "github", GithubRepo: repo, GithubApiUrl: api}
	}
	local := func(id string) *models.Project { return &models.Project{ID: id, Name: id, IssueTracker: "local"} }
	// The same fixtures as web/tests/targetProject.test.mjs.
	cases := []struct {
		name         string
		macro, other *models.Project
		want         bool
	}{
		{"one Jira site", jira("a", "https://equativ.atlassian.net"), jira("b", "https://EQUATIV.atlassian.net/"), true},
		{"two Jira sites", jira("a", "https://equativ.atlassian.net"), jira("b", "https://other.atlassian.net"), false},
		{"one GitHub repository", github("a", "org/repo", ""), github("b", "ORG/repo", ""), true},
		{"two GitHub repositories", github("a", "org/repo", ""), github("b", "org/other", ""), false},
		{"another GitHub instance", github("a", "org/repo", ""), github("b", "org/repo", "https://ghe.example.com/api/v3"), false},
		{"two local boards", local("a"), local("b"), true},
		{"local and Jira", local("a"), jira("b", "https://x.atlassian.net"), false},
		{"GitHub and Jira", github("a", "org/repo", ""), jira("b", "https://x.atlassian.net"), false},
	}
	for _, c := range cases {
		got, reason := database.sameTrackerInstance(c.macro, c.other)
		if got != c.want {
			t.Errorf("%s: got %v (%s)", c.name, got, reason)
		}
		if !got && !strings.Contains(reason, c.other.Name) {
			t.Errorf("%s: the refusal must name the target project, got %q", c.name, reason)
		}
	}
}

func TestStoryWithoutTargetLandsInTheMacroProjectUnderItsEpic(t *testing.T) {
	database, fake, macroProject, line := jiraStoryDB(t, func(_, _ *models.Project) string { return "" })
	_, story, notice, err := database.CreateStoryFromMacroTodo(macroProject.ID, "PE-100", line)
	key := storyKeyOf(story)
	if err != nil || notice != "" {
		t.Fatalf("create: %q %q %v", key, notice, err)
	}
	if len(fake.created) != 1 || fake.created[0] != "PE" {
		t.Fatalf("the story must be created in the macro's project, got %v", fake.created)
	}
	if fake.parents[key] != "PE-100" {
		t.Fatalf("the epic must be written as the story's parent on Jira, got %v", fake.parents)
	}
}

func TestStoryLandsInATargetOfTheSameSite(t *testing.T) {
	database, fake, macroProject, line := jiraStoryDB(t, func(sameSite, _ *models.Project) string { return sameSite.ID })
	meta, story, _, err := database.CreateStoryFromMacroTodo(macroProject.ID, "PE-100", line)
	key := storyKeyOf(story)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(fake.created) != 1 || fake.created[0] != "DATA" || !strings.HasPrefix(key, "DATA-") {
		t.Fatalf("the story must be created in the target project, got %v %q", fake.created, key)
	}
	if fake.parents[key] != "PE-100" || meta.Todos[0].StoryKey != key {
		t.Fatalf("parent %v, line %+v", fake.parents, meta.Todos[0])
	}
}

func TestTargetOnAnotherSiteIsRefusedBeforeAnyWrite(t *testing.T) {
	database, fake, macroProject, line := jiraStoryDB(t, func(_, otherSite *models.Project) string { return otherSite.ID })
	_, _, _, err := database.CreateStoryFromMacroTodo(macroProject.ID, "PE-100", line)
	if err == nil || !strings.Contains(err.Error(), "Elsewhere") || !strings.Contains(err.Error(), "instance Jira") {
		t.Fatalf("the refusal must name the target and the reason, got %v", err)
	}
	if len(fake.created) != 0 {
		t.Fatalf("nothing may be created, got %v", fake.created)
	}
	metas, _ := database.GetProjectMacros(macroProject.ID)
	if metas[0].Todos[0].StoryKey != "" {
		t.Fatalf("the line must stay unchanged, got %+v", metas[0].Todos[0])
	}
}

func TestMissingTargetIsRefused(t *testing.T) {
	database, fake, macroProject, line := jiraStoryDB(t, func(_, _ *models.Project) string { return "gone-project" })
	_, _, _, err := database.CreateStoryFromMacroTodo(macroProject.ID, "PE-100", line)
	if err == nil || !strings.Contains(err.Error(), "gone-project") || len(fake.created) != 0 {
		t.Fatalf("a missing target must be refused by name, got %v, created %v", err, fake.created)
	}
}

func TestARefusedParentKeepsTheStory(t *testing.T) {
	database, fake, macroProject, line := jiraStoryDB(t, func(_, _ *models.Project) string { return "" })
	fake.parentErr = errors.New("403 no permission to edit parent")
	meta, story, notice, err := database.CreateStoryFromMacroTodo(macroProject.ID, "PE-100", line)
	key := storyKeyOf(story)
	if err != nil {
		t.Fatalf("the story exists, the call must not fail: %v", err)
	}
	if meta.Todos[0].StoryKey != key || !strings.Contains(notice, "403") {
		t.Fatalf("key recorded and refusal said: %+v %q", meta.Todos[0], notice)
	}
}

func TestLocalStoryIsParentedLocallyOnly(t *testing.T) {
	database, err := NewDB(t.TempDir() + "/tasks.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Board", Slug: "board", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	todos := []models.MacroTodo{{ID: "line", Text: "Local work"}}
	if _, err := database.SaveMacroMeta(project.ID, "M-1", nil, nil, nil, &todos); err != nil {
		t.Fatal(err)
	}
	_, story, notice, err := database.CreateStoryFromMacroTodo(project.ID, "M-1", "line")
	key := storyKeyOf(story)
	if err != nil || notice != "" || key == "" {
		t.Fatalf("local story: %q %q %v", key, notice, err)
	}
	task, _ := database.GetTaskByID(key)
	if task == nil || task.ParentKey != "M-1" {
		t.Fatalf("the local parent must be written: %+v", task)
	}
}

func storyKeyOf(story *models.Task) string {
	if story == nil {
		return ""
	}
	return story.Key
}
