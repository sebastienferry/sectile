package db

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/trackerapi"
)

// The SFE-360 shape of #392: a coordination project with no code remote whose
// work lands in argocd-arch, on GitLab.
const (
	archRepository = "gitlab.com/smartadserver/private/arch/argocd-arch"
	archMR         = "https://gitlab.com/smartadserver/private/arch/argocd-arch/-/merge_requests/97"
	sfeBranch      = "feature/SFE-360-remove-arch-api"
)

func crossRepoTask(t *testing.T, req models.CreateProjectRequest) (*DB, *models.Task) {
	t.Helper()
	d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	no := false
	req.RepoPath, req.IssueTracker, req.UseWorktrees = "/not-mounted-on-server", "local", &no
	p, err := d.CreateProject(req)
	if err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "cross repository", Labels: []string{"#specified"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.conn.Exec("UPDATE tasks SET branch_name=?,status='to_implement',labels='[\"#specified\"]' WHERE id=?", sfeBranch, task.ID); err != nil {
		t.Fatal(err)
	}
	if task, err = d.GetTaskByID(task.ID); err != nil {
		t.Fatal(err)
	}
	return d, task
}

// fakeCrossRepoAgent answers pr_evidence and git_evidence for another
// repository the way a current agent does, echoing the repository asked.
type fakeCrossRepoAgent struct {
	t           *testing.T
	mr          string // pr_evidence answer, without the echo
	checkout    string // git_evidence answer, without the echo
	noEcho      bool
	prCalls     int
	lookupError error
}

func (f *fakeCrossRepoAgent) operate(_ context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
	echo := func(body string) json.RawMessage {
		if f.noEcho || op.Repository == "" {
			return json.RawMessage(body)
		}
		return json.RawMessage(fmt.Sprintf(`{"repository":%q,%s`, op.Repository, strings.TrimPrefix(body, "{")))
	}
	switch op.Action {
	case "pr_evidence":
		f.prCalls++
		if f.lookupError != nil {
			return nil, f.lookupError
		}
		if op.Repository != archRepository || op.Branch != sfeBranch {
			f.t.Fatalf("merge request asked in the wrong place: %#v", op)
		}
		return echo(f.mr), nil
	case "git_evidence":
		if op.Repository != archRepository || op.Branch != sfeBranch {
			f.t.Fatalf("head asked in the wrong place: %#v", op)
		}
		return echo(f.checkout), nil
	}
	f.t.Fatalf("unexpected operation %q", op.Action)
	return nil, nil
}

func archAnswer(draft, merged bool) string {
	return fmt.Sprintf(`{"forge":"gitlab","url":%q,"branch":%q,"sha":"mr-head","open":%t,"draft":%t,"merged":%t}`, archMR, sfeBranch, !merged, draft, merged)
}

const notFound = `{"found":false}`

func foundCheckout(sha string, clean bool) string {
	return fmt.Sprintf(`{"found":true,"path":"/work/argocd-arch","sha":%q,"branch":%q,"clean":%t}`, sha, sfeBranch, clean)
}

func TestForeignMergeRequestCompletesImplementationWithoutRemote(t *testing.T) {
	d, task := crossRepoTask(t, models.CreateProjectRequest{Name: "SFE"})
	agent := &fakeCrossRepoAgent{t: t, mr: archAnswer(false, false), checkout: notFound}
	d.SetAgentOperations(agent.operate)

	// No verified checkout: the forge's word stands, and the report says so.
	url, notice, err := d.validateStagePR(task, "", "implement", d.adjustmentCheckout(task), sfeBranch, archMR)
	if err != nil || url != archMR || !strings.Contains(notice, "not verified on a local checkout") || !strings.Contains(notice, archRepository) {
		t.Fatalf("url=%q notice=%q err=%v", url, notice, err)
	}
	got, _, err := d.TransitionTaskStage(task.ID, "implemented", "implementation complete", archMR, sfeBranch)
	if err != nil || got.PrURL == nil || *got.PrURL != archMR || d.StageOfTask(got) != "implemented" {
		t.Fatalf("foreign merge request not accepted: %+v %v", got, err)
	}
}

func TestForeignHeadIsCheckedOnAVerifiedCheckout(t *testing.T) {
	d, task := crossRepoTask(t, models.CreateProjectRequest{Name: "SFE"})
	agent := &fakeCrossRepoAgent{t: t, mr: archAnswer(false, false), checkout: foundCheckout("mr-head", true)}
	d.SetAgentOperations(agent.operate)

	if _, notice, err := d.validateStagePR(task, "", "implement", "", sfeBranch, archMR); err != nil || notice != "" {
		t.Fatalf("verified head: notice=%q err=%v", notice, err)
	}
	agent.checkout = foundCheckout("other-commit", true)
	if _, _, err := d.validateStagePR(task, "", "implement", "", sfeBranch, archMR); err == nil || !strings.Contains(err.Error(), "merge request does not contain the agent checkout commit") {
		t.Fatalf("stale checkout accepted or misworded: %v", err)
	}
	agent.checkout = foundCheckout("mr-head", false)
	if _, _, err := d.validateStagePR(task, "", "adjust", "", sfeBranch, archMR); err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("dirty checkout accepted for adjustment: %v", err)
	}
}

func TestForeignMergeRequestKeepsTheEvidenceRules(t *testing.T) {
	d, task := crossRepoTask(t, models.CreateProjectRequest{Name: "SFE"})
	agent := &fakeCrossRepoAgent{t: t, mr: archAnswer(true, false), checkout: notFound}
	d.SetAgentOperations(agent.operate)

	// A draft completes implementation but not adjustment.
	if _, _, err := d.TransitionTaskStage(task.ID, "implemented", "draft", archMR, sfeBranch); err != nil {
		t.Fatalf("draft refused for implementation: %v", err)
	}
	if _, _, err := d.TransitionTaskStage(task.ID, "reviewed", "draft", archMR, sfeBranch); err == nil || !strings.Contains(err.Error(), "still a draft") {
		t.Fatalf("draft accepted for adjustment: %v", err)
	}
	// Another source branch is refused.
	agent.mr = strings.Replace(archAnswer(false, false), sfeBranch, "feature/other", 1)
	if _, _, err := d.TransitionTaskStage(task.ID, "reviewed", "other branch", archMR, sfeBranch); err == nil || !strings.Contains(err.Error(), "GitLab does not confirm") {
		t.Fatalf("merge request on another branch accepted: %v", err)
	}
	// The human merged it: adjustment completes.
	agent.mr = archAnswer(false, true)
	if got, _, err := d.TransitionTaskStage(task.ID, "reviewed", "merged", archMR, sfeBranch); err != nil || d.StageOfTask(got) != "reviewed" {
		t.Fatalf("merged merge request refused: %+v %v", got, err)
	}
}

func TestForeignGitHubPullRequestOnAMultiRepositoryProject(t *testing.T) {
	no := false
	d, task := crossRepoTask(t, models.CreateProjectRequest{Name: "Multi", GitRemoteUrl: "git@github.com:acme/app.git", MonoRepo: &no})
	const prURL = "https://github.com/acme/tools/pull/5"
	// The transition queues a postback job, and the queue worker replays the
	// same lookup from its own goroutine: the recorded call has to be guarded
	// or the read below races it.
	var mu sync.Mutex
	var asked []string
	d.prEvidenceLookup = func(repo, branch, url string) (trackerapi.PullRequest, error) {
		mu.Lock()
		asked = []string{repo, branch, url}
		mu.Unlock()
		return trackerapi.PullRequest{URL: prURL, Branch: sfeBranch, SHA: "pr-head", Open: true}, nil
	}
	d.SetAgentOperations(func(_ context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		if op.Action != "git_evidence" || op.Repository != "github.com/acme/tools" {
			t.Fatalf("unexpected operation %#v", op)
		}
		return json.RawMessage(fmt.Sprintf(`{"repository":%q,"found":true,"sha":"pr-head","branch":%q,"clean":true}`, op.Repository, sfeBranch)), nil
	})
	got, _, err := d.TransitionTaskStage(task.ID, "implemented", "done", prURL, sfeBranch)
	if err != nil || got.PrURL == nil || *got.PrURL != prURL {
		t.Fatalf("foreign GitHub pull request refused: %+v %v", got, err)
	}
	mu.Lock()
	recorded := append([]string(nil), asked...)
	mu.Unlock()
	if !slices.Equal(recorded, []string{"github.com/acme/tools", sfeBranch, prURL}) {
		t.Fatalf("lookup asked %v", recorded)
	}
}

func TestMonoRepoProjectRefusesAForeignPullRequest(t *testing.T) {
	d, task := crossRepoTask(t, models.CreateProjectRequest{Name: "Mono", GitRemoteUrl: "git@gitlab.com:group/app.git"})
	d.prEvidenceLookup = func(string, string, string) (trackerapi.PullRequest, error) {
		t.Fatal("a refused pull request must not be looked up")
		return trackerapi.PullRequest{}, nil
	}
	_, _, err := d.TransitionTaskStage(task.ID, "implemented", "elsewhere", archMR, sfeBranch)
	if err == nil || !strings.Contains(err.Error(), "is not in the project repository gitlab.com/group/app") {
		t.Fatalf("foreign pull request accepted on a mono-repo project: %v", err)
	}
	// The project's own repository keeps today's path, SSH remote or not.
	own := "https://gitlab.com/group/app/-/merge_requests/3"
	d.prEvidenceLookup = func(repo, _, url string) (trackerapi.PullRequest, error) {
		if url != "" {
			t.Fatalf("same-repository lookup routed as foreign: %s %s", repo, url)
		}
		return trackerapi.PullRequest{URL: own, Branch: sfeBranch, SHA: "agent-commit", Open: true, Forge: "gitlab"}, nil
	}
	d.SetAgentOperations(func(_ context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		if op.Repository != "" {
			t.Fatalf("same-repository head asked elsewhere: %#v", op)
		}
		return json.RawMessage(fmt.Sprintf(`{"sha":"agent-commit","branch":%q,"clean":true}`, sfeBranch)), nil
	})
	if _, _, err = d.TransitionTaskStage(task.ID, "implemented", "own", own, sfeBranch); err != nil {
		t.Fatalf("own merge request refused: %v", err)
	}
}

func TestCrossRepositoryLookupFailuresAreNeverAbsence(t *testing.T) {
	d, task := crossRepoTask(t, models.CreateProjectRequest{Name: "SFE"})
	agent := &fakeCrossRepoAgent{t: t, mr: archAnswer(false, false), checkout: notFound, noEcho: true}
	d.SetAgentOperations(agent.operate)
	// An agent that ignores the repository would have answered for the project
	// checkout: that is a failed lookup.
	if _, _, err := d.TransitionTaskStage(task.ID, "implemented", "old agent", archMR, sfeBranch); err == nil || !strings.Contains(err.Error(), "too old") {
		t.Fatalf("old agent answer accepted: %v", err)
	}
	// Its head answer is held to the same rule.
	d.prEvidenceLookup = func(string, string, string) (trackerapi.PullRequest, error) {
		return trackerapi.PullRequest{URL: archMR, Branch: sfeBranch, SHA: "mr-head", Open: true, Forge: "gitlab"}, nil
	}
	if _, _, err := d.TransitionTaskStage(task.ID, "implemented", "old agent", archMR, sfeBranch); err == nil || !strings.Contains(err.Error(), "too old") {
		t.Fatalf("old agent head accepted: %v", err)
	}
	d.prEvidenceLookup = nil
	// Without prUrl the project checkout is asked, and its missing origin is named.
	agent.noEcho = false
	agent.lookupError = fmt.Errorf("project checkout has no origin remote; configure the project repository or pass the prUrl of the repository that carries the pull request")
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		if op.Repository != "" {
			t.Fatalf("no prUrl, yet asked about %q", op.Repository)
		}
		return nil, agent.lookupError
	})
	_, _, err := d.TransitionTaskStage(task.ID, "implemented", "no prUrl", "", sfeBranch)
	if err == nil || !strings.Contains(err.Error(), "no origin remote") || strings.Contains(err.Error(), "no matching") {
		t.Fatalf("missing origin not named: %v", err)
	}
}

func TestAdjustmentEntryPointsFollowTheForeignPullRequest(t *testing.T) {
	d, task := crossRepoTask(t, models.CreateProjectRequest{Name: "SFE"})
	agent := &fakeCrossRepoAgent{t: t, mr: archAnswer(false, false), checkout: notFound}
	d.SetAgentOperations(agent.operate)

	// The adjust prerequisite reads the task's current PR where it lives.
	if _, err := d.conn.Exec("UPDATE tasks SET pr_url=?, pr_links=? WHERE id=?", archMR, encodePullRequestLinks([]models.TaskPullRequest{{URL: archMR, Branch: sfeBranch}}), task.ID); err != nil {
		t.Fatal(err)
	}
	task, _ = d.GetTaskByID(task.ID)
	if pr, err := d.adjustmentPrerequisite(task, "", true); err != nil || pr.URL != archMR || agent.prCalls != 1 {
		t.Fatalf("adjustment prerequisite: %+v %v (calls %d)", pr, err, agent.prCalls)
	}

	// A post-back has no note: the weaker evidence lands on its run.
	run := models.TaskActivity{ID: "run-392", TaskID: task.ID, TaskKey: task.Key, SkillID: "implement", Status: string(models.ActivityStatusCompleted)}
	if err := d.addTaskActivityDirect(run); err != nil {
		t.Fatal(err)
	}
	stage, prURL, branch := "implemented", archMR, sfeBranch
	if _, _, err := d.PostBackTask(models.TaskPostBackPayload{TaskID: task.ID, Stage: &stage, PrURL: &prURL, BranchName: &branch, ActivityID: run.ID}); err != nil {
		t.Fatal(err)
	}
	d.mu.RLock()
	steps := d.getActivityByIDUnsafe(run.ID).Steps
	d.mu.RUnlock()
	if !slices.ContainsFunc(steps, func(s string) bool { return strings.Contains(s, "not verified on a local checkout") }) {
		t.Fatalf("post-back run steps: %v", steps)
	}
}

func TestResolveStagePRTargetKnowsTheProjectRepository(t *testing.T) {
	no := false
	for _, tc := range []struct {
		name    string
		p       *models.Project
		prURL   string
		foreign bool
		refused bool
	}{
		{"no prUrl", &models.Project{MonoRepo: true, GitRemoteUrl: "git@github.com:acme/app.git"}, "", false, false},
		{"own over ssh remote", &models.Project{MonoRepo: true, GitRemoteUrl: "git@github.com:Acme/App.git"}, "https://github.com/acme/app/pull/1", false, false},
		// A mirror remote on an unbranded host: the PRs live on the GitHub repository.
		{"own github repo behind a mirror remote", &models.Project{MonoRepo: true, GithubRepo: "acme/app", GitRemoteUrl: "git@code.acme.io:acme/app.git"}, "https://github.com/acme/app/pull/1", false, false},
		{"own self-hosted gitlab", &models.Project{MonoRepo: true, GitRemoteUrl: "ssh://git@gitlab.acme.io:2222/acme/app.git"}, "https://gitlab.acme.io/acme/app/-/merge_requests/4", false, false},
		{"unrecognized link keeps the project path", &models.Project{}, "https://forge/pull/2", false, false},
		{"no repository", &models.Project{MonoRepo: true}, archMR, true, false},
		{"multi-repository", &models.Project{MonoRepo: no, GitRemoteUrl: "git@github.com:acme/app.git"}, archMR, true, false},
		{"mono-repository", &models.Project{MonoRepo: true, GitRemoteUrl: "git@github.com:acme/app.git"}, archMR, false, true},
	} {
		target, err := (&DB{}).resolveStagePRTarget(tc.p, nil, tc.prURL)
		if (err != nil) != tc.refused || target.foreign != tc.foreign {
			t.Errorf("%s: target=%+v err=%v", tc.name, target, err)
		}
	}
}

func TestMonoRepoAdjustmentPrerequisiteKeepsTheProjectLookup(t *testing.T) {
	d, task := crossRepoTask(t, models.CreateProjectRequest{Name: "Renamed", GitRemoteUrl: "git@gitlab.com:group/app-renamed.git"})
	// The recorded link still names the repository's former path.
	old := "https://gitlab.com/group/app/-/merge_requests/3"
	if _, err := d.conn.Exec("UPDATE tasks SET pr_url=?, pr_links=? WHERE id=?", old, encodePullRequestLinks([]models.TaskPullRequest{{URL: old, Branch: sfeBranch}}), task.ID); err != nil {
		t.Fatal(err)
	}
	task, _ = d.GetTaskByID(task.ID)
	d.prEvidenceLookup = func(repo, _, url string) (trackerapi.PullRequest, error) {
		if url != "" {
			t.Fatalf("mono-repo prerequisite routed to %s", repo)
		}
		return trackerapi.PullRequest{URL: "https://gitlab.com/group/app-renamed/-/merge_requests/3", Branch: sfeBranch, Open: true, Forge: "gitlab"}, nil
	}
	if _, err := d.adjustmentPrerequisite(task, "", false); err != nil {
		t.Fatalf("renamed repository prerequisite refused: %v", err)
	}
}
