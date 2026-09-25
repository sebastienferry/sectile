package db

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

// multiRepoProject is a multi-repo project whose code remote is o/a and which
// declares o/b, with one ticket on feat/12.
func multiRepoProject(t *testing.T, d *DB) (*models.Project, *models.Task) {
	t.Helper()
	no := false
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Multi", IssueTracker: "local", MonoRepo: &no, UseWorktrees: &no,
		GitRemoteUrl: "git@github.com:o/a.git", Repositories: []string{"https://github.com/o/b"}})
	if err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "two repositories"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.conn.Exec("UPDATE tasks SET branch_name='feat/12' WHERE id=?", task.ID); err != nil {
		t.Fatal(err)
	}
	if task, err = d.GetTaskByID(task.ID); err != nil {
		t.Fatal(err)
	}
	return p, task
}

func TestProjectRepositoriesKeepTheCodeRemoteFirst(t *testing.T) {
	testProjectRepositories(t, testDB(t))
}

func TestPostgresProjectRepositories(t *testing.T) {
	testProjectRepositories(t, openPostgres(t))
}

func testProjectRepositories(t *testing.T, d *DB) {
	p, _ := multiRepoProject(t, d)
	if len(p.Repositories) != 2 || p.Repositories[0].Identity != "github.com/o/a" || p.Repositories[1].Identity != "github.com/o/b" {
		t.Fatalf("repositories = %+v", p.Repositories)
	}
	var stored string
	if err := d.conn.QueryRow("SELECT repositories FROM projects WHERE id = ?", p.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != `["https://github.com/o/b"]` {
		t.Errorf("stored = %s, the code remote must not be stored twice", stored)
	}
	respelled := []string{"https://github.com/o/b", "git@github.com:o/b.git"}
	if _, err := d.UpdateProjectAs("", p.ID, models.UpdateProjectRequest{Repositories: &respelled}); !errors.Is(err, ErrDuplicateRepository) {
		t.Errorf("SSH and HTTPS of one remote: err = %v, want ErrDuplicateRepository", err)
	}
	// The project as the client read it lists the code remote first: sending
	// it back is not declaring it twice.
	asRead := []string{"https://github.com/o/a", "https://github.com/o/b"}
	if updated, err := d.UpdateProjectAs("", p.ID, models.UpdateProjectRequest{Repositories: &asRead}); err != nil || len(updated.Repositories) != 2 {
		t.Errorf("code remote sent back: %+v, %v", updated, err)
	}
	three := []string{"https://github.com/o/b", "git@gitlab.com:g/c.git"}
	updated, err := d.UpdateProjectAs("", p.ID, models.UpdateProjectRequest{Repositories: &three})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Repositories) != 3 || updated.Repositories[2].Identity != "gitlab.com/g/c" {
		t.Errorf("after update = %+v", updated.Repositories)
	}
	unrelated := "renamed"
	if updated, err = d.UpdateProjectAs("", p.ID, models.UpdateProjectRequest{Name: &unrelated}); err != nil || len(updated.Repositories) != 3 {
		t.Errorf("an unrelated update kept %+v, %v", updated.Repositories, err)
	}
}

func TestTaskRepositoryPin(t *testing.T) {
	d := testDB(t)
	_, task := multiRepoProject(t, d)
	pin := "git@github.com:o/b.git"
	updated, err := d.UpdateTask(task.ID, models.UpdateTaskRequest{Repository: &pin})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Repository != "github.com/o/b" {
		t.Errorf("pinned = %q, want the identity", updated.Repository)
	}
	outside := "https://github.com/o/elsewhere"
	if _, err := d.UpdateTask(task.ID, models.UpdateTaskRequest{Repository: &outside}); !errors.Is(err, ErrRepositoryNotInProject) {
		t.Errorf("pin outside the list: err = %v", err)
	}
	if reread, _ := d.GetTaskByID(task.ID); reread.Repository != "github.com/o/b" {
		t.Errorf("a refused pin changed the task: %q", reread.Repository)
	}
	empty := ""
	if updated, err = d.UpdateTask(task.ID, models.UpdateTaskRequest{Repository: &empty}); err != nil || updated.Repository != "" {
		t.Errorf("unpin = %q, %v", updated.Repository, err)
	}
}

func TestRepositoryConversionAppliesOnce(t *testing.T) {
	testRepositoryConversion(t, testDB(t))
}

func TestPostgresRepositoryConversion(t *testing.T) {
	testRepositoryConversion(t, openPostgres(t))
}

func testRepositoryConversion(t *testing.T, d *DB) {
	no := false
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Legacy", IssueTracker: "local", MonoRepo: &no, UseWorktrees: &no,
		GitRemoteUrl: "git@github.com:o/a.git", RepoPath: "/src/a", RepoPaths: []string{"/src/b", "/gone"}})
	if err != nil {
		t.Fatal(err)
	}
	pinned, _ := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "pinned"})
	lost, _ := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "lost"})
	if _, err := d.conn.Exec("UPDATE tasks SET repo_path='/src/b' WHERE id=?", pinned.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("UPDATE tasks SET repo_path='/gone' WHERE id=?", lost.ID); err != nil {
		t.Fatal(err)
	}

	legacy, err := d.LegacyRepoPaths(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, path := range legacy.Paths {
		got[path.Path] = path.TaskIDs
	}
	if legacy.Migrated || len(got) != 3 || len(got["/src/b"]) != 1 || got["/src/b"][0] != pinned.ID || len(got["/gone"]) != 1 {
		t.Fatalf("legacy = %+v", legacy)
	}

	report := models.RepositoryConversion{
		Converted: []models.ConvertedRepoPath{{Path: "/src/a", URL: "git@github.com:o/a.git"}, {Path: "/src/b", URL: "git@github.com:o/b.git", TaskIDs: []string{pinned.ID}}},
		Dropped:   []models.DroppedRepoPath{{Path: "/gone", Reason: "not found", TaskIDs: []string{lost.ID}}},
	}
	converted, err := d.ApplyRepositoryConversion("usr_1", p.ID, report)
	if err != nil {
		t.Fatal(err)
	}
	if len(converted.Repositories) != 2 || converted.Repositories[1].Identity != "github.com/o/b" || len(converted.RepoPaths) != 0 {
		t.Errorf("project after conversion = %+v / %v", converted.Repositories, converted.RepoPaths)
	}
	var stored models.RepositoryConversion
	if err := json.Unmarshal([]byte(converted.RepositoriesMigration), &stored); err != nil || stored.UserID != "usr_1" || len(stored.Dropped) != 1 || stored.ConvertedAt == "" {
		t.Errorf("report = %q, %v", converted.RepositoriesMigration, err)
	}
	if task, _ := d.GetTaskByID(pinned.ID); task.Repository != "github.com/o/b" || task.RepoPath != nil {
		t.Errorf("pinned task = %q, %v", task.Repository, task.RepoPath)
	}
	if task, _ := d.GetTaskByID(lost.ID); task.Repository != "" || task.RepoPath != nil {
		t.Errorf("dropped task = %q, %v", task.Repository, task.RepoPath)
	}

	if _, err := d.ApplyRepositoryConversion("usr_2", p.ID, report); !errors.Is(err, ErrRepositoriesConverted) {
		t.Errorf("second conversion: err = %v, want ErrRepositoriesConverted", err)
	}
	if again, _ := d.LegacyRepoPaths(p.ID); !again.Migrated || len(again.Paths) != 0 {
		t.Errorf("legacy after conversion = %+v", again)
	}
}

func TestRunAwaitingRepositoryMarksAnAutonomousRun(t *testing.T) {
	d := testDB(t)
	_, task := multiRepoProject(t, d)
	run, err := d.StartRemoteRunBy("usr_1", task.ID, "code-issue", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("UPDATE task_activities SET run_mode = 'autonomous' WHERE id = ?", run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.MarkRunAwaitingRepository(Actor{ID: "usr_2"}, false, run.ID, true); !errors.Is(err, ErrRunNotYours) {
		t.Errorf("another user: err = %v", err)
	}
	waiting, err := d.MarkRunAwaitingRepository(Actor{ID: "usr_1"}, false, run.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if waiting.WaitingSince == nil || waiting.WaitingReason != "repository" {
		t.Errorf("waiting = %v, %q", waiting.WaitingSince, waiting.WaitingReason)
	}
	if err := d.SetRemoteRunWaiting(run.ID, true); err != nil {
		t.Fatal(err)
	}
	if again, _ := d.GetActivityByID(run.ID); again.WaitingReason != "" || again.WaitingSince == nil {
		t.Errorf("a session wait must drop the repository reason: %v, %q", again.WaitingSince, again.WaitingReason)
	}
	released, err := d.MarkRunAwaitingRepository(Actor{ID: "usr_1"}, false, run.ID, false)
	if err != nil || released.WaitingSince != nil || released.WaitingReason != "" {
		t.Errorf("released = %v, %q, %v", released.WaitingSince, released.WaitingReason, err)
	}
}

// fakeWorktreeAgent answers repository_worktree like a current agent.
type fakeWorktreeAgent struct {
	calls  []agentprotocol.Operation
	noEcho bool
}

func (f *fakeWorktreeAgent) call(_ context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
	f.calls = append(f.calls, op)
	repository := op.Repository
	if f.noEcho {
		repository = ""
	}
	return json.Marshal(models.RepositoryWorktree{Repository: repository, Path: "/src/" + op.Repository + "/.tasks/worktrees/x", Branch: op.Branch})
}

func TestPrepareRepositoryWorktree(t *testing.T) {
	d := testDB(t)
	p, task := multiRepoProject(t, d)
	agent := &fakeWorktreeAgent{}
	d.SetAgentOperations(agent.call)

	worktree, err := d.PrepareRepositoryWorktree(context.Background(), "", task.Key, "git@github.com:o/b.git")
	if err != nil {
		t.Fatal(err)
	}
	if worktree.Branch != "feat/12" || len(agent.calls) != 1 || agent.calls[0].Action != "repository_worktree" || agent.calls[0].Repository != "github.com/o/b" || agent.calls[0].TaskID != task.ID {
		t.Errorf("worktree = %+v, calls = %+v", worktree, agent.calls)
	}
	if _, err := d.PrepareRepositoryWorktree(context.Background(), "", task.Key, "github.com/o/b"); err != nil {
		t.Fatal(err)
	}
	if reread, _ := d.GetTaskByID(task.ID); len(reread.ChangedRepositories) != 1 || reread.ChangedRepositories[0] != "github.com/o/b" {
		t.Errorf("changed = %v, want github.com/o/b once", reread.ChangedRepositories)
	}

	for name, repository := range map[string]string{
		"outside the project": "https://github.com/o/elsewhere",
		"primary repository":  "git@github.com:o/a.git",
	} {
		if _, err := d.PrepareRepositoryWorktree(context.Background(), "", task.Key, repository); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	agent.noEcho = true
	if _, err := d.PrepareRepositoryWorktree(context.Background(), "", task.Key, "github.com/o/b"); err == nil || !strings.Contains(err.Error(), "too old") {
		t.Errorf("no echo: err = %v", err)
	}
	agent.noEcho = false

	yes := true
	if _, err := d.UpdateProjectAs("", p.ID, models.UpdateProjectRequest{MonoRepo: &yes}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.PrepareRepositoryWorktree(context.Background(), "", task.Key, "github.com/o/b"); err == nil || !strings.Contains(err.Error(), "mono-repo") {
		t.Errorf("mono-repo: err = %v", err)
	}
}

func TestAProjectKnownByItsGitHubRepositorySavesItsOwnList(t *testing.T) {
	d := testDB(t)
	no := false
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "GitHub only", IssueTracker: "local", MonoRepo: &no, GithubRepo: "o/a", Repositories: []string{"git@github.com:o/b.git"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Repositories) != 2 || p.Repositories[0].Identity != "github.com/o/a" {
		t.Fatalf("repositories = %+v", p.Repositories)
	}
	asRead := []string{p.Repositories[0].URL, p.Repositories[1].URL}
	if _, err := d.UpdateProjectAs("", p.ID, models.UpdateProjectRequest{Repositories: &asRead}); err != nil {
		t.Errorf("saving the list as read: %v", err)
	}
}

func TestChangingTheCodeRemoteDoesNotDeclareTheOldOne(t *testing.T) {
	d := testDB(t)
	p, _ := multiRepoProject(t, d)
	remote := "git@github.com:o/z.git"
	updated, err := d.UpdateProjectAs("", p.ID, models.UpdateProjectRequest{GitRemoteUrl: &remote})
	if err != nil {
		t.Fatal(err)
	}
	var identities []string
	for _, repository := range updated.Repositories {
		identities = append(identities, repository.Identity)
	}
	if strings.Join(identities, " ") != "github.com/o/z github.com/o/b" {
		t.Errorf("repositories = %v, the old code remote must not stay declared", identities)
	}
}

func TestAMovedTicketKeepsOnlyAPinItsNewProjectDeclares(t *testing.T) {
	d := testDB(t)
	_, task := multiRepoProject(t, d)
	pin := "github.com/o/b"
	if _, err := d.UpdateTask(task.ID, models.UpdateTaskRequest{Repository: &pin}); err != nil {
		t.Fatal(err)
	}
	other, err := d.CreateProject(models.CreateProjectRequest{Name: "Other", IssueTracker: "local", GitRemoteUrl: "git@github.com:o/x.git"})
	if err != nil {
		t.Fatal(err)
	}
	moved, err := d.UpdateTask(task.ID, models.UpdateTaskRequest{ProjectID: &other.ID})
	if err != nil {
		t.Fatal(err)
	}
	if moved.Repository != "" {
		t.Errorf("pin after the move = %q, want none", moved.Repository)
	}
}

func TestAPinTheProjectNoLongerDeclaresIsNoPin(t *testing.T) {
	d := testDB(t)
	p, task := multiRepoProject(t, d)
	pin := "github.com/o/b"
	if _, err := d.UpdateTask(task.ID, models.UpdateTaskRequest{Repository: &pin}); err != nil {
		t.Fatal(err)
	}
	none := []string{}
	if _, err := d.UpdateProjectAs("", p.ID, models.UpdateProjectRequest{Repositories: &none}); err != nil {
		t.Fatal(err)
	}
	project, _ := d.GetProjectByID(p.ID)
	reread, _ := d.GetTaskByID(task.ID)
	if multiRepoTask(project, reread) || TaskPrimaryRepository(project, reread) != "github.com/o/a" {
		t.Errorf("a removed repository still decides: multi=%v primary=%q", multiRepoTask(project, reread), TaskPrimaryRepository(project, reread))
	}
}
