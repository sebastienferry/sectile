package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

// A GitLab project with two repositories: g/a is the code remote, g/b a second
// repository. Every lookup goes through the agent, as GitLab evidence does.
const (
	mrA = "https://gitlab.com/g/a/-/merge_requests/1"
	mrB = "https://gitlab.com/g/b/-/merge_requests/2"
)

// fakeRepoAgent answers the agent operations of a two-repository project. A
// stage transition also queues a postback that the queue worker runs on its own
// goroutine and that calls the agent again, so every field is behind mu: the
// test reads what the agent was asked while the worker may still be asking.
type fakeRepoAgent struct {
	mu        sync.Mutex
	t         *testing.T
	prs       map[string]string // repository ("" for the code remote) → merge request URL
	heads     map[string]string // repository → checkout head
	removed   []string
	failRemov string
	evidence  []string // repositories git_evidence was asked about ("" for the task checkout)
}

func (f *fakeRepoAgent) operate(_ context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch op.Action {
	case "pr_evidence":
		url, ok := f.prs[op.Repository]
		if !ok {
			return json.RawMessage(fmt.Sprintf(`{"repository":%q,"forge":"gitlab","refusal":"no merge request from %s"}`, op.Repository, op.Branch)), nil
		}
		return json.RawMessage(fmt.Sprintf(`{"repository":%q,"forge":"gitlab","url":%q,"branch":%q,"sha":"head-%s","open":true}`, op.Repository, url, op.Branch, url[len(url)-1:])), nil
	case "git_evidence":
		f.evidence = append(f.evidence, op.Repository)
		key := op.Repository
		head, ok := f.heads[key]
		if !ok {
			return json.RawMessage(fmt.Sprintf(`{"repository":%q,"found":false}`, key)), nil
		}
		// The project checkout is asked without a branch and answers the one
		// checked out, as the agent does.
		return json.RawMessage(fmt.Sprintf(`{"repository":%q,"found":true,"path":"/work","sha":%q,"branch":"feat/12","clean":true}`, key, head)), nil
	case "remove_workspace":
		result := models.WorktreeRemoval{}
		for _, repository := range op.Repositories {
			if repository == f.failRemov {
				result.Failed = append(result.Failed, models.WorktreeRemovalFailed{Repository: repository, Error: "locked"})
				continue
			}
			result.Removed = append(result.Removed, repository)
		}
		f.removed = op.Repositories
		return json.Marshal(result)
	}
	f.t.Errorf("unexpected operation %#v", op)
	return nil, fmt.Errorf("unexpected operation %q", op.Action)
}

// asked returns the repositories git_evidence was asked about so far.
func (f *fakeRepoAgent) asked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.evidence...)
}

// set changes the fake's answers under its lock.
func (f *fakeRepoAgent) set(change func(*fakeRepoAgent)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change(f)
}

func twoRepoTask(t *testing.T) (*DB, *models.Task, *fakeRepoAgent) {
	t.Helper()
	d := testDB(t)
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Multi", IssueTracker: "local",
		GitRemoteUrl: "git@gitlab.com:g/a.git", Repositories: []string{"git@gitlab.com:g/b.git"}})
	if err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "two repositories", Labels: []string{"#specified"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.conn.Exec(`UPDATE tasks SET branch_name='feat/12', status='to_implement', labels='["#specified"]' WHERE id=?`, task.ID); err != nil {
		t.Fatal(err)
	}
	if err := d.AddChangedRepository(task.ID, "gitlab.com/g/b"); err != nil {
		t.Fatal(err)
	}
	if task, err = d.GetTaskByID(task.ID); err != nil {
		t.Fatal(err)
	}
	agent := &fakeRepoAgent{t: t,
		prs:   map[string]string{"": mrA, "gitlab.com/g/a": mrA, "gitlab.com/g/b": mrB},
		heads: map[string]string{"": "head-1", "gitlab.com/g/b": "head-2"}}
	d.SetAgentOperations(agent.operate)
	return d, task, agent
}

func TestImplementationNeedsAPullRequestPerChangedRepository(t *testing.T) {
	d, task, _ := twoRepoTask(t)
	got, _, err := d.TransitionTaskStageWithPRs("", task.ID, "implemented", "done", []string{mrA, mrB}, "feat/12")
	if err != nil {
		t.Fatal(err)
	}
	if got.PrURL == nil || *got.PrURL != mrA {
		t.Errorf("current pull request = %v, want the primary repository's", got.PrURL)
	}
	var urls []string
	for _, link := range got.PrLinks {
		urls = append(urls, link.URL)
	}
	if strings.Join(urls, " ") != mrB+" "+mrA {
		t.Errorf("recorded = %v", urls)
	}
}

func TestAChangedRepositoryIsLookedUpByBranchWhenNoLinkNamesIt(t *testing.T) {
	d, task, _ := twoRepoTask(t)
	if _, _, err := d.TransitionTaskStageWithPRs("", task.ID, "implemented", "done", []string{mrA}, "feat/12"); err != nil {
		t.Fatalf("the second repository's merge request is found by branch: %v", err)
	}
}

func TestAChangedRepositoryWithoutPullRequestIsRefused(t *testing.T) {
	d, task, agent := twoRepoTask(t)
	agent.set(func(f *fakeRepoAgent) { delete(f.prs, "gitlab.com/g/b") })
	_, _, err := d.TransitionTaskStageWithPRs("", task.ID, "implemented", "done", []string{mrA}, "feat/12")
	if err == nil || !strings.Contains(err.Error(), "gitlab.com/g/b") {
		t.Fatalf("err = %v, want a refusal naming gitlab.com/g/b", err)
	}
}

func TestAPullRequestOutsideTheChangedRepositoriesIsRefused(t *testing.T) {
	d, task, _ := twoRepoTask(t)
	_, _, err := d.TransitionTaskStageWithPRs("", task.ID, "implemented", "done", []string{mrA, "https://gitlab.com/g/c/-/merge_requests/3"}, "feat/12")
	if err == nil || !strings.Contains(err.Error(), "not in a repository") {
		t.Fatalf("err = %v", err)
	}
}

func TestAStaleSecondaryHeadIsRefused(t *testing.T) {
	d, task, agent := twoRepoTask(t)
	agent.set(func(f *fakeRepoAgent) { f.heads["gitlab.com/g/b"] = "older" })
	_, _, err := d.TransitionTaskStageWithPRs("", task.ID, "implemented", "done", []string{mrA, mrB}, "feat/12")
	if err == nil || !strings.Contains(err.Error(), "gitlab.com/g/b") || !strings.Contains(err.Error(), "does not contain the agent checkout commit") {
		t.Fatalf("err = %v", err)
	}
}

func TestASingleRepositoryTaskKeepsOnePullRequest(t *testing.T) {
	d, task, _ := twoRepoTask(t)
	if _, err := d.conn.Exec(`UPDATE tasks SET changed_repositories='[]' WHERE id=?`, task.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.TransitionTaskStageWithPRs("", task.ID, "implemented", "done", []string{mrA, mrB}, "feat/12"); err == nil || !strings.Contains(err.Error(), "single repository") {
		t.Fatalf("two links on a single-repository task: %v", err)
	}
	if got, _, err := d.TransitionTaskStageWithPRs("", task.ID, "implemented", "done", []string{mrA}, "feat/12"); err != nil || len(got.PrLinks) != 1 {
		t.Fatalf("single repository: %+v, %v", got, err)
	}
}

func TestWorktreeRemovalCoversEveryChangedRepository(t *testing.T) {
	d, task, agent := twoRepoTask(t)
	if err := d.RemoveTaskWorktree("", task.ID); err != nil {
		t.Fatal(err)
	}
	var removed []string
	agent.set(func(f *fakeRepoAgent) { removed = f.removed })
	if strings.Join(removed, " ") != "gitlab.com/g/a gitlab.com/g/b" {
		t.Errorf("asked to remove %v", removed)
	}
	agent.set(func(f *fakeRepoAgent) { f.failRemov = "gitlab.com/g/b" })
	if err := d.RemoveTaskWorktree("", task.ID); err == nil || !strings.Contains(err.Error(), "gitlab.com/g/b: locked") {
		t.Errorf("err = %v, want the failed repository named", err)
	}
}

func TestAdjustmentChecksEverySecondaryRepository(t *testing.T) {
	d, task, agent := twoRepoTask(t)
	agent.set(func(f *fakeRepoAgent) { delete(f.prs, "gitlab.com/g/b") })
	if _, err := d.adjustmentPrerequisite(task, "", false); err == nil || !strings.Contains(err.Error(), "gitlab.com/g/b") {
		t.Fatalf("err = %v, want the secondary repository named", err)
	}
}

func TestThePrimaryPullRequestStaysCurrentWhenAlreadyRecorded(t *testing.T) {
	d, task, _ := twoRepoTask(t)
	if _, err := d.conn.Exec(`UPDATE tasks SET changed_repositories='[]' WHERE id=?`, task.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.TransitionTaskStageWithPRs("", task.ID, "implemented", "done", []string{mrA}, "feat/12"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddChangedRepository(task.ID, "gitlab.com/g/b"); err != nil {
		t.Fatal(err)
	}
	got, _, err := d.TransitionTaskStageWithPRs("", task.ID, "implemented", "again", []string{mrA, mrB}, "feat/12")
	if err != nil {
		t.Fatal(err)
	}
	if got.PrURL == nil || *got.PrURL != mrA {
		t.Errorf("current pull request = %v, want the primary repository's", got.PrURL)
	}
}

func TestTheCodeRepositoryChangedFromAPinnedTicketIsReadInItsOwnCheckout(t *testing.T) {
	d, task, agent := twoRepoTask(t)
	// Pinned to g/b, the ticket also changed the project's own g/a.
	if _, err := d.conn.Exec(`UPDATE tasks SET repository='gitlab.com/g/b', changed_repositories='["gitlab.com/g/a"]' WHERE id=?`, task.ID); err != nil {
		t.Fatal(err)
	}
	agent.set(func(f *fakeRepoAgent) { f.heads["gitlab.com/g/a"] = "head-1" })
	got, _, err := d.TransitionTaskStageWithPRs("", task.ID, "implemented", "done", []string{mrB, mrA}, "feat/12")
	if err != nil {
		t.Fatal(err)
	}
	// The transition asks first; the postback it queued may already be asking
	// again on the worker, so only the transition's own questions are checked.
	if asked := agent.asked(); len(asked) < 2 || strings.Join(asked[:2], " ") != "gitlab.com/g/a gitlab.com/g/b" {
		t.Errorf("heads asked in %q, want each repository named", asked)
	}
	if got.PrURL == nil || *got.PrURL != mrB {
		t.Errorf("current pull request = %v, want the pinned repository's", got.PrURL)
	}
}

// A repository the ticket changed through a folder attached on a workstation
// only is outside the project's list (#484): it still needs its pull request
// at every transition that asks for evidence, and its worktree goes at
// handoff.
func TestAnAttachedRepositoryNeedsItsPullRequest(t *testing.T) {
	const mrLib = "https://gitlab.com/g/lib/-/merge_requests/4"
	d, task, agent := twoRepoTask(t)
	if _, err := d.conn.Exec(`UPDATE tasks SET changed_repositories='["gitlab.com/g/lib"]' WHERE id=?`, task.ID); err != nil {
		t.Fatal(err)
	}
	task, _ = d.GetTaskByID(task.ID)
	agent.set(func(f *fakeRepoAgent) { f.heads["gitlab.com/g/lib"] = "head-4" })

	if _, _, err := d.TransitionTaskStageWithPRs("", task.ID, "implemented", "done", []string{mrA}, "feat/12"); err == nil || !strings.Contains(err.Error(), "gitlab.com/g/lib") {
		t.Fatalf("err = %v, want a refusal naming gitlab.com/g/lib", err)
	}
	if _, err := d.adjustmentPrerequisite(task, "", false); err == nil || !strings.Contains(err.Error(), "gitlab.com/g/lib") {
		t.Fatalf("adjust: err = %v, want the attached repository named", err)
	}

	agent.set(func(f *fakeRepoAgent) { f.prs["gitlab.com/g/lib"] = mrLib })
	got, _, err := d.TransitionTaskStageWithPRs("", task.ID, "implemented", "done", []string{mrA, mrLib}, "feat/12")
	if err != nil {
		t.Fatal(err)
	}
	if got.PrURL == nil || *got.PrURL != mrA {
		t.Errorf("current pull request = %v, want the primary repository's", got.PrURL)
	}
	if !strings.Contains(fmt.Sprint(got.PrLinks), mrLib) {
		t.Errorf("recorded = %+v, want the attached repository's merge request", got.PrLinks)
	}

	if err := d.RemoveTaskWorktree("", task.ID); err != nil {
		t.Fatal(err)
	}
	var removed []string
	agent.set(func(f *fakeRepoAgent) { removed = f.removed })
	if strings.Join(removed, " ") != "gitlab.com/g/a gitlab.com/g/lib" {
		t.Errorf("asked to remove %v", removed)
	}
}
