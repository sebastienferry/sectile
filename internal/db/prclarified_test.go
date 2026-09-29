package db

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/trackerapi"
)

// earlyOwnedTask is a task at new in a project that opens its pull request at
// timing, with an agent answering spec_artifacts with mode and a forge that
// answers with *pr, or finds nothing while it is nil.
func earlyOwnedTask(t *testing.T, timing, mode string, pr **trackerapi.PullRequest) (*DB, *models.Task, func() []string) {
	t.Helper()
	d := testDB(t)
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Early", IssueTracker: "local", PRCreationStage: timing})
	if err != nil {
		t.Fatal(err)
	}
	setLegacyProject(t, d, p.ID, map[string]any{"repo_path": "/not-mounted-on-server"})
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "early", Labels: []string{"#new"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.conn.Exec("UPDATE tasks SET branch_name='ticket' WHERE id=?", task.ID); err != nil {
		t.Fatal(err)
	}
	var asked []string
	var askedMu sync.Mutex
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		askedMu.Lock()
		asked = append(asked, op.Action)
		askedMu.Unlock()
		switch op.Action {
		case "spec_artifacts":
			return json.RawMessage(`{"mode":"` + mode + `"}`), nil
		case "git_evidence":
			return json.RawMessage(`{"sha":"agent-commit","branch":"ticket","clean":true}`), nil
		}
		return nil, fmt.Errorf("unknown local operation %q", op.Action)
	})
	d.prEvidenceLookup = func(string, string, string) (trackerapi.PullRequest, error) {
		if *pr == nil {
			return trackerapi.PullRequest{}, fmt.Errorf("no pull request for this branch")
		}
		return **pr, nil
	}
	return d, task, func() []string {
		askedMu.Lock()
		defer askedMu.Unlock()
		return slices.Clone(asked)
	}
}

// A project that opens its pull request at clarification refuses a clarified
// transition without one, then records the draft once it exists; the
// specification that follows needs that same pull request (#580).
func TestClarifiedTransitionNeedsThePullRequestOfAnEarlyProject(t *testing.T) {
	var pr *trackerapi.PullRequest
	d, task, _ := earlyOwnedTask(t, "clarified", models.SpecArtifactsKeep, &pr)
	if _, _, err := d.TransitionTaskStage(task.ID, "clarified", "clarification confirmed", "", "ticket"); err == nil {
		t.Fatal("a clarified transition without its PR must be refused")
	}
	if got, _ := d.GetTaskByID(task.ID); d.StageOfTask(got) != "new" {
		t.Fatalf("a refused transition must leave the task at new, got %q", d.StageOfTask(got))
	}
	if _, _, err := d.TransitionTaskStage(task.ID, "specified", "spec written", "", "ticket"); err == nil {
		t.Fatal("the specification of such a project must need the PR too")
	}

	pr = &trackerapi.PullRequest{URL: "https://forge/pull/7", Branch: "ticket", SHA: "agent-commit", Open: true, Draft: true}
	got, _, err := d.TransitionTaskStage(task.ID, "clarified", "clarification confirmed", pr.URL, "ticket")
	if err != nil || d.StageOfTask(got) != "clarified" || got.PrURL == nil || *got.PrURL != pr.URL {
		t.Fatalf("the draft must be accepted and recorded: %+v %v", got, err)
	}
	got, _, err = d.TransitionTaskStage(task.ID, "specified", "spec written", pr.URL, "ticket")
	if err != nil || d.StageOfTask(got) != "specified" || len(got.PrLinks) != 1 {
		t.Fatalf("the specification must update the same draft: %+v %v", got, err)
	}
}

// A managed run reports its stage through the post-back, which holds the
// clarified stage to the same pull request.
func TestClarifiedPostBackNeedsThePullRequestOfAnEarlyProject(t *testing.T) {
	var pr *trackerapi.PullRequest
	d, task, _ := earlyOwnedTask(t, "clarified", models.SpecArtifactsKeep, &pr)
	stage, branch := "clarified", "ticket"
	if _, _, err := d.PostBackTask(models.TaskPostBackPayload{TaskID: task.ID, Stage: &stage, BranchName: &branch}); err == nil {
		t.Fatal("a clarified post-back without its PR must be refused")
	}
	pr = &trackerapi.PullRequest{URL: "https://forge/pull/7", Branch: "ticket", SHA: "agent-commit", Open: true, Draft: true}
	url := pr.URL
	if _, _, err := d.PostBackTask(models.TaskPostBackPayload{TaskID: task.ID, Stage: &stage, BranchName: &branch, PrURL: &url}); err != nil {
		t.Fatalf("a clarified post-back with its draft must be accepted: %v", err)
	}
}

// Projects that open their pull request later keep clarifying as before: no
// pull request is needed, and neither the forge nor the agent is asked.
func TestClarifiedTransitionOfALaterProjectNeedsNoPullRequest(t *testing.T) {
	for _, timing := range []string{"specified", "implemented"} {
		t.Run(timing, func(t *testing.T) {
			var pr *trackerapi.PullRequest
			d, task, asked := earlyOwnedTask(t, timing, models.SpecArtifactsDrop, &pr)
			looked := false
			d.prEvidenceLookup = func(string, string, string) (trackerapi.PullRequest, error) {
				looked = true
				return trackerapi.PullRequest{}, fmt.Errorf("no pull request for this branch")
			}
			got, _, err := d.TransitionTaskStage(task.ID, "clarified", "clarification confirmed", "", "ticket")
			if err != nil || d.StageOfTask(got) != "clarified" {
				t.Fatalf("%+v %v", got, err)
			}
			if looked || len(asked()) != 0 {
				t.Fatalf("no lookup expected: forge %v, agent %v", looked, asked())
			}
		})
	}
}

// A workstation that drops the artefacts has nothing to show at clarification
// nor at specification: both go without a pull request and say it is deferred;
// implementation still needs it.
func TestEarlyProjectDefersThePullRequestWhenTheWorkstationDrops(t *testing.T) {
	var pr *trackerapi.PullRequest
	d, task, _ := earlyOwnedTask(t, "clarified", models.SpecArtifactsDrop, &pr)
	for _, step := range []struct{ stage, skill string }{{"clarified", "clarify"}, {"specified", "specify"}} {
		set, err := d.validateStagePRs(task, "", step.skill, "", "ticket", []string{""})
		if err != nil || set.notice != prDeferredNotice || len(set.urls) != 0 {
			t.Fatalf("%s must say the PR is deferred: %+v %v", step.stage, set, err)
		}
		got, _, err := d.TransitionTaskStage(task.ID, step.stage, "stage done", "", "ticket")
		if err != nil || d.StageOfTask(got) != step.stage {
			t.Fatalf("%s must pass without a PR: %+v %v", step.stage, got, err)
		}
	}
	if _, _, err := d.TransitionTaskStage(task.ID, "implemented", "code written", "", "ticket"); err == nil {
		t.Fatal("the implemented stage must still require its PR")
	}
}
