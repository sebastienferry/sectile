package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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
	// changes holds the branch_changes answer per repository; a repository
	// without one has a commit ahead, which keeps its pull request required.
	changes  map[string]string
	branches []string // repositories branch_changes was asked about
	lookups  []string // repositories pr_evidence was asked about
}

func (f *fakeRepoAgent) operate(_ context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch op.Action {
	case "branch_changes":
		f.branches = append(f.branches, op.Repository)
		if answer, ok := f.changes[op.Repository]; ok {
			if answer == "" {
				return nil, errors.New("git ls-remote: origin unreachable")
			}
			return json.RawMessage(answer), nil
		}
		return json.RawMessage(fmt.Sprintf(`{"repository":%q,"found":true,"defaultBranch":"main","exists":true,"ahead":1}`, op.Repository)), nil
	case "pr_evidence":
		f.lookups = append(f.lookups, op.Repository)
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
	if err == nil || !strings.Contains(err.Error(), "not in a repository") || !strings.Contains(err.Error(), "prepare_repository_worktree is called") {
		t.Fatalf("err = %v, want a refusal saying how a repository becomes changed", err)
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

// unchangedB is the agent's answer for a g/b checkout whose task branch has
// no commit of its own (#678).
const unchangedB = `{"repository":"gitlab.com/g/b","found":true,"defaultBranch":"main","exists":true,"ahead":0}`

func TestAPreparedRepositoryLeftUnchangedIsSkipped(t *testing.T) {
	for name, tc := range map[string]struct {
		answer string
		notice string
	}{
		"branch with no commit ahead": {unchangedB, "Prepared, unchanged: gitlab.com/g/b has no commit of feat/12 ahead of main, so no merge request is expected there."},
		"branch gone":                 {`{"repository":"gitlab.com/g/b","found":true,"defaultBranch":"main","exists":false,"ahead":0}`, "Prepared, unchanged: in gitlab.com/g/b, feat/12 exists neither locally nor on origin, so no merge request is expected there."},
	} {
		t.Run(name, func(t *testing.T) {
			d, task, agent := twoRepoTask(t)
			agent.set(func(f *fakeRepoAgent) {
				delete(f.prs, "gitlab.com/g/b")
				f.changes = map[string]string{"gitlab.com/g/b": tc.answer}
			})
			set, err := d.validateStagePRs(task, "", "implement", "", "feat/12", []string{mrA})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(set.urls, " ") != mrA || set.notice != tc.notice {
				t.Fatalf("set = %+v, want the primary merge request and the notice %q", set, tc.notice)
			}
			agent.set(func(f *fakeRepoAgent) {
				if slices.Contains(f.lookups, "gitlab.com/g/b") || slices.Contains(f.evidence, "gitlab.com/g/b") {
					t.Errorf("g/b was looked up on the forge: pr_evidence %v, git_evidence %v", f.lookups, f.evidence)
				}
			})

			got, _, err := d.TransitionTaskStageWithPRs("", task.ID, "implemented", "done", []string{mrA}, "feat/12")
			if err != nil {
				t.Fatal(err)
			}
			if len(got.PrLinks) != 1 || got.PrLinks[0].URL != mrA || got.PrURL == nil || *got.PrURL != mrA {
				t.Fatalf("recorded %+v, want the primary merge request only", got.PrLinks)
			}
			if !slices.Contains(got.ChangedRepositories, "gitlab.com/g/b") {
				t.Errorf("changed repositories = %v, g/b must stay recorded", got.ChangedRepositories)
			}
		})
	}
}

// Anything but a verified "no commit ahead" keeps the pull request required.
func TestAnUnprovenUnchangedRepositoryStillNeedsItsPullRequest(t *testing.T) {
	for name, answer := range map[string]string{
		"commit ahead":          `{"repository":"gitlab.com/g/b","found":true,"defaultBranch":"main","exists":true,"ahead":1}`,
		"agent error":           "",
		"no checkout":           `{"repository":"gitlab.com/g/b","found":false}`,
		"another repository":    `{"repository":"gitlab.com/g/a","found":true,"defaultBranch":"main","exists":true,"ahead":0}`,
		"no echo":               `{"found":true,"defaultBranch":"main","exists":true,"ahead":0}`,
		"no ahead count":        `{"repository":"gitlab.com/g/b","found":true,"defaultBranch":"main","exists":true}`,
		"no default branch":     `{"repository":"gitlab.com/g/b","found":true,"exists":true,"ahead":0}`,
		"agent predates this":   "unsupported",
		"branch is the default": `{"repository":"gitlab.com/g/b","found":true,"defaultBranch":"feat/12","exists":true,"ahead":0}`,
	} {
		t.Run(name, func(t *testing.T) {
			d, task, agent := twoRepoTask(t)
			agent.set(func(f *fakeRepoAgent) {
				delete(f.prs, "gitlab.com/g/b")
				f.changes = map[string]string{"gitlab.com/g/b": answer}
			})
			if answer == "unsupported" {
				d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
					if op.Action == "branch_changes" {
						return nil, &agentprotocol.UnsupportedOperationError{Device: "laptop", Operation: op.Action}
					}
					return agent.operate(ctx, op)
				})
			}
			_, _, err := d.TransitionTaskStageWithPRs("", task.ID, "implemented", "done", []string{mrA}, "feat/12")
			if err == nil || !strings.Contains(err.Error(), "gitlab.com/g/b") || !strings.Contains(err.Error(), "no merge request from feat/12") {
				t.Fatalf("err = %v, want the current refusal naming gitlab.com/g/b", err)
			}
		})
	}
}

func TestAGivenOrRecordedSecondaryPullRequestIsNotAskedAbout(t *testing.T) {
	d, task, agent := twoRepoTask(t)
	agent.set(func(f *fakeRepoAgent) { f.changes = map[string]string{"gitlab.com/g/b": unchangedB} })
	if _, _, err := d.TransitionTaskStageWithPRs("", task.ID, "implemented", "done", []string{mrA, mrB}, "feat/12"); err != nil {
		t.Fatal(err)
	}
	// Recorded on the branch now: the next check reads it, still without asking.
	task, _ = d.GetTaskByID(task.ID)
	if _, err := d.adjustmentPrerequisite(task, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := d.validateStagePRs(task, "", "implement", "", "feat/12", []string{mrA}); err != nil {
		t.Fatal(err)
	}
	agent.set(func(f *fakeRepoAgent) {
		if len(f.branches) != 0 {
			t.Errorf("branch_changes asked about %v for a repository with a merge request", f.branches)
		}
	})
}

func TestAdjustmentSkipsAnUnchangedSecondaryRepository(t *testing.T) {
	d, task, agent := twoRepoTask(t)
	agent.set(func(f *fakeRepoAgent) {
		delete(f.prs, "gitlab.com/g/b")
		f.changes = map[string]string{"gitlab.com/g/b": unchangedB}
	})
	if err := d.checkSecondaryPRs(mustProject(t, d, task), task, "", "feat/12"); err != nil {
		t.Fatalf("an unchanged repository must be skipped: %v", err)
	}
	// A later commit there requires its merge request again.
	agent.set(func(f *fakeRepoAgent) { f.changes = nil })
	if err := d.checkSecondaryPRs(mustProject(t, d, task), task, "", "feat/12"); err == nil || !strings.Contains(err.Error(), "gitlab.com/g/b") {
		t.Fatalf("err = %v, want the changed repository named", err)
	}
}

// A project that opens its pull request at specification applies the same
// rule at the specified stage, and a managed run's result does at any stage.
func TestEveryPathSkipsAnUnchangedSecondaryRepository(t *testing.T) {
	d, task, agent := twoRepoTask(t)
	agent.set(func(f *fakeRepoAgent) {
		delete(f.prs, "gitlab.com/g/b")
		f.changes = map[string]string{"gitlab.com/g/b": unchangedB}
	})
	if _, err := d.conn.Exec(`UPDATE projects SET pr_creation_stage='specified' WHERE id=?`, task.ProjectID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.validateStagePRs(task, "", "specify", "", "feat/12", []string{mrA}); err != nil {
		t.Fatalf("specified: %v", err)
	}

	run := models.TaskActivity{ID: "run-678", TaskID: task.ID, TaskKey: task.Key, SkillID: "implement", Status: string(models.ActivityStatusCompleted)}
	if err := d.addTaskActivityDirect(run); err != nil {
		t.Fatal(err)
	}
	stage, prURL, branch := "implemented", mrA, "feat/12"
	got, _, err := d.PostBackTask(models.TaskPostBackPayload{TaskID: task.ID, Stage: &stage, PrURL: &prURL, BranchName: &branch, ActivityID: run.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.PrLinks) != 1 || got.PrLinks[0].URL != mrA {
		t.Errorf("recorded %+v, want the primary merge request only", got.PrLinks)
	}
	d.mu.RLock()
	steps := d.getActivityByIDUnsafe(run.ID).Steps
	d.mu.RUnlock()
	if !slices.ContainsFunc(steps, func(s string) bool { return strings.Contains(s, "Prepared, unchanged: gitlab.com/g/b") }) {
		t.Fatalf("post-back run steps: %v", steps)
	}
}

func mustProject(t *testing.T, d *DB, task *models.Task) *models.Project {
	t.Helper()
	p, err := d.GetProjectByID(task.ProjectID)
	if err != nil || p == nil {
		t.Fatalf("project: %v", err)
	}
	return p
}
