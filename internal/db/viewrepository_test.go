package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/trackerapi"
	"tasks/internal/tracker"
)

func viewTestUser(t *testing.T, d *DB) string {
	t.Helper()
	if err := d.EnsureUser(ImplicitUserID); err != nil {
		t.Fatal(err)
	}
	return ImplicitUserID
}

// US3: a view keeps an optional repository, kept by an edit that leaves it,
// cleared by an empty value, and refused when it names no host and path.
func TestBoardViewRepository(t *testing.T) {
	d, task := crossRepoTask(t, models.CreateProjectRequest{Name: "Views"})
	user := viewTestUser(t, d)
	name, remote := "Arch", "git@gitlab.com:smartadserver/private/arch/argocd-arch.git"
	view, err := d.CreateBoardView(user, models.BoardViewRequest{Name: &name, ProjectIDs: &[]string{task.ProjectID}, Repository: &remote})
	if err != nil {
		t.Fatal(err)
	}
	if view.Repository != remote {
		t.Fatalf("created repository = %q", view.Repository)
	}
	renamed := "Arch work"
	if view, err = d.UpdateBoardView(user, view.ID, models.BoardViewRequest{Name: &renamed}); err != nil || view.Repository != remote {
		t.Fatalf("an edit without the field changed the repository: %q (%v)", view.Repository, err)
	}
	listed, err := d.ListBoardViews(user)
	if err != nil || len(listed) != 1 || listed[0].Repository != remote {
		t.Fatalf("listed views = %+v (%v)", listed, err)
	}
	for _, bad := range []string{"not a remote", "argocd-arch", "https://gitlab.com", "https://gitlab.com/"} {
		if _, err := d.UpdateBoardView(user, view.ID, models.BoardViewRequest{Repository: &bad}); !errors.Is(err, ErrBoardViewRepositoryInvalid) {
			t.Errorf("%q accepted as a repository: %v", bad, err)
		}
	}
	empty := " "
	if view, err = d.UpdateBoardView(user, view.ID, models.BoardViewRequest{Repository: &empty}); err != nil || view.Repository != "" {
		t.Fatalf("clearing the repository: %q (%v)", view.Repository, err)
	}
}

// US4: a launch checks the view and records its repository and launcher.
func TestViewLaunchRecordsTheRepository(t *testing.T) {
	d, task := crossRepoTask(t, models.CreateProjectRequest{Name: "Views"})
	user := viewTestUser(t, d)
	name, remote := "Arch", "https://gitlab.com/smartadserver/private/arch/argocd-arch.git"
	view, err := d.CreateBoardView(user, models.BoardViewRequest{Name: &name, ProjectIDs: &[]string{task.ProjectID}, Repository: &remote})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.ViewLaunchRepository("somebody-else", view.ID, task.ProjectID); !errors.Is(err, ErrBoardViewNotFound) {
		t.Fatalf("another user's view: %v", err)
	}
	if _, err := d.ViewLaunchRepository(user, view.ID, "other-project"); !errors.Is(err, ErrViewDoesNotSelectProject) {
		t.Fatalf("a view without the project: %v", err)
	}
	identity, err := d.ViewLaunchRepository(user, view.ID, task.ProjectID)
	if err != nil || identity != archRepository {
		t.Fatalf("identity = %q (%v)", identity, err)
	}
	if err := d.RecordTaskViewRepository(task.ID, identity, user); err != nil {
		t.Fatal(err)
	}
	// A view without a repository says nothing, and leaves the record alone.
	if err := d.RecordTaskViewRepository(task.ID, "", "somebody-else"); err != nil {
		t.Fatal(err)
	}
	stored, err := d.GetTaskByID(task.ID)
	if err != nil || stored.ViewRepository != archRepository || d.taskViewRepositoryUser(task.ID) != user {
		t.Fatalf("recorded %q by %q (%v)", stored.ViewRepository, d.taskViewRepositoryUser(task.ID), err)
	}
	listed, err := d.GetTasks("", "", "", "", task.ProjectID, "", "", "", "", nil, nil, false)
	if err != nil || len(listed) != 1 || listed[0].ViewRepository != archRepository {
		t.Fatalf("the list does not carry the view repository: %+v (%v)", listed, err)
	}
}

// monoRepoViewTask is a mono-repo project with a code remote whose ticket was
// launched from a view on argocd-arch.
func monoRepoViewTask(t *testing.T) (*DB, *models.Task) {
	t.Helper()
	d, task := crossRepoTask(t, models.CreateProjectRequest{Name: "Mono", GitRemoteUrl: "git@gitlab.com:group/app.git"})
	if err := d.RecordTaskViewRepository(task.ID, archRepository, ImplicitUserID); err != nil {
		t.Fatal(err)
	}
	task, err := d.GetTaskByID(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	return d, task
}

// US5: a mono-repo project accepts the pull request of the view repository,
// checked with the foreign rules of #392, and still refuses any other.
func TestMonoRepoProjectAcceptsTheViewRepositoryPullRequest(t *testing.T) {
	d, task := monoRepoViewTask(t)
	agent := &fakeCrossRepoAgent{t: t, mr: archAnswer(false, false), checkout: foundCheckout("mr-head", true)}
	d.SetAgentOperations(agent.operate)
	if _, _, err := d.TransitionTaskStage(task.ID, "implemented", "from the view", archMR, sfeBranch); err != nil {
		t.Fatalf("the view repository's merge request was refused: %v", err)
	}
	stored, _ := d.GetTaskByID(task.ID)
	if stored.PrURL == nil || *stored.PrURL != archMR {
		t.Fatalf("recorded pull request = %v", stored.PrURL)
	}

	d, task = monoRepoViewTask(t)
	other := "https://gitlab.com/group/unrelated/-/merge_requests/4"
	if _, _, err := d.TransitionTaskStage(task.ID, "implemented", "elsewhere", other, sfeBranch); err == nil || !strings.Contains(err.Error(), "is not in the project repository gitlab.com/group/app") {
		t.Fatalf("a foreign pull request outside the view repository: %v", err)
	}
}

// US5: a stage that names no pull request looks it up by branch in the view
// repository, and the adjust prerequisite reads it there.
func TestViewRepositoryIsWhereAnUnnamedPullRequestIsLookedUp(t *testing.T) {
	d, task := monoRepoViewTask(t)
	agent := &fakeCrossRepoAgent{t: t, mr: archAnswer(false, false), checkout: foundCheckout("mr-head", true)}
	d.SetAgentOperations(agent.operate)
	if _, _, err := d.TransitionTaskStage(task.ID, "implemented", "by branch", "", sfeBranch); err != nil {
		t.Fatalf("lookup by branch in the view repository: %v", err)
	}
	if agent.prCalls != 1 {
		t.Fatalf("pull request lookups = %d, want one in the view repository", agent.prCalls)
	}
	project, _ := d.GetProjectByID(task.ProjectID)
	target, err := d.resolveStagePRTarget(project, task, "")
	if err != nil || !target.foreign || target.link.Identity() != archRepository {
		t.Fatalf("adjust target = %+v (%v)", target, err)
	}
	// The project's own repository is no view repository: its rules stand.
	own := &models.Task{ViewRepository: "gitlab.com/group/app"}
	if taskViewRepository(project, own) != "" {
		t.Fatal("the project repository was taken for a view repository")
	}
}

// US6: a full synchronisation finds the pull request of the branch in the view
// repository, whatever the tracker, and reports a failed lookup as a warning.
func TestDiscoverySearchesTheViewRepository(t *testing.T) {
	d, proj, task := discoveryTestDB(t, "#implemented")
	branch := "feat/42"
	if _, err := d.UpdateTask(task.ID, models.UpdateTaskRequest{BranchName: &branch}); err != nil {
		t.Fatal(err)
	}
	if err := d.RecordTaskViewRepository(task.ID, "github.com/acme/platform", "launcher"); err != nil {
		t.Fatal(err)
	}
	task, _ = d.GetTaskByID(task.ID)
	prURL := "https://github.com/acme/platform/pull/7"
	var asked []string
	d.prEvidenceLookup = func(repo, br, _ string) (trackerapi.PullRequest, error) {
		asked = append(asked, repo+"@"+br)
		return trackerapi.PullRequest{URL: prURL, Branch: br, Open: true}, nil
	}
	local := tracker.NewLocalAdapter()
	steps, halt := d.rediscoverPullRequests(context.Background(), proj, local, task, false)
	if halt || len(asked) != 1 || asked[0] != "github.com/acme/platform@feat/42" {
		t.Fatalf("lookup %v, halt %v, steps %v", asked, halt, steps)
	}
	stored, _ := d.GetTaskByID(task.ID)
	if stored.PrURL == nil || *stored.PrURL != prURL {
		t.Fatalf("attached = %v, steps %v", stored.PrURL, steps)
	}

	// The gate still bounds it: a ticket holding a link is not looked up again.
	if steps, _ = d.rediscoverPullRequests(context.Background(), proj, local, stored, false); len(asked) != 1 {
		t.Fatalf("a linked ticket was looked up again: %v", steps)
	}
	if got := d.rediscoverProjectPullRequests(context.Background(), proj, local, []models.Task{*stored}); len(got) != 0 || len(asked) != 1 {
		t.Fatalf("the full pass looked a linked ticket up: %v", got)
	}
}

func TestDiscoveryReportsAFailedViewRepositoryLookup(t *testing.T) {
	d, proj, task := discoveryTestDB(t, "#implemented")
	branch := "feat/42"
	if _, err := d.UpdateTask(task.ID, models.UpdateTaskRequest{BranchName: &branch}); err != nil {
		t.Fatal(err)
	}
	if err := d.RecordTaskViewRepository(task.ID, "gitlab.example.com/group/platform", "launcher"); err != nil {
		t.Fatal(err)
	}
	task, _ = d.GetTaskByID(task.ID)
	d.prEvidenceLookup = func(string, string, string) (trackerapi.PullRequest, error) {
		return trackerapi.PullRequest{}, fmt.Errorf("agent offline")
	}
	steps := d.rediscoverProjectPullRequests(context.Background(), proj, tracker.NewLocalAdapter(), []models.Task{*task})
	if len(steps) != 1 || !strings.Contains(steps[0], "gitlab.example.com/group/platform") || !strings.Contains(steps[0], "agent offline") {
		t.Fatalf("steps = %v", steps)
	}
	stored, _ := d.GetTaskByID(task.ID)
	if stored.PrURL != nil || len(stored.PrLinks) != 0 {
		t.Fatalf("a failed lookup attached %v", stored.PrLinks)
	}
}
