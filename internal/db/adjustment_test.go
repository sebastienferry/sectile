package db

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/trackerapi"
	"testing"
	"time"
)

func TestAdjustmentAliasesAndHumanBoundary(t *testing.T) {
	for _, id := range []string{"adjust", "adjust-issue", "review"} {
		s, ok := StageSkillByID(id)
		if !ok || s.ID != "adjust" || s.ToStage != "reviewed" {
			t.Fatalf("alias %s: %+v", id, s)
		}
	}
	next, _ := NextStep("reviewed")
	if next.SkillID != "handoff" {
		t.Fatal(next)
	}
}
func TestAdjustmentEvidenceRejectsInvalidPR(t *testing.T) {
	good := trackerapi.PullRequest{URL: "https://forge/pull/1", Branch: "ticket", Open: true}
	recorded := []models.TaskPullRequest{{URL: good.URL, Branch: "ticket"}}
	// A follow-up PR on the branch the task already used is the task's own work,
	// so "follow-up" passes; only a PR on an unrelated branch is a substitution.
	accepted := map[string]bool{"valid": true, "merged": true, "follow-up": true, "first-pr": true}
	for _, kind := range []string{"valid", "draft", "closed", "branch", "follow-up", "merged", "merged-branch", "unrelated-branch", "first-pr"} {
		t.Run(kind, func(t *testing.T) {
			p, links := good, recorded
			switch kind {
			case "draft":
				p.Draft = true
			case "closed":
				p.Open = false
			case "branch":
				p.Branch = "other"
			case "follow-up":
				p.URL = "https://forge/pull/2"
			case "merged":
				p.Open, p.Merged = false, true
			case "merged-branch":
				p.Open, p.Merged, p.Branch = false, true, "other"
			case "unrelated-branch":
				p.URL, links = "https://forge/pull/2", []models.TaskPullRequest{{URL: good.URL, Branch: "another-ticket"}}
			case "first-pr":
				links = nil
			}
			err := validatePullRequestEvidence(p, "ticket", p.URL, links, true)
			if (err == nil) != accepted[kind] {
				t.Fatalf("%s: %v", kind, err)
			}
		})
	}
}
func TestMergedPRCompletesReview(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	no := false
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Merged", RepoPath: "/not-mounted-on-server", IssueTracker: "local", UseWorktrees: &no})
	if err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "merged", Labels: []string{"#implemented"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.conn.Exec("UPDATE tasks SET branch_name='ticket',status='to_test',labels='[\"#implemented\"]' WHERE id=?", task.ID); err != nil {
		t.Fatal(err)
	}
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		return json.RawMessage(`{"sha":"agent-commit","branch":"ticket","clean":true}`), nil
	})
	// The human merged the PR before the review stage was recorded; that must not strand the task.
	pr := trackerapi.PullRequest{URL: "https://forge/pull/1", Branch: "ticket", SHA: "agent-commit", Merged: true}
	d.prEvidenceLookup = func(string, string) (trackerapi.PullRequest, error) { return pr, nil }
	got, _, err := d.TransitionTaskStage(task.ID, "reviewed", "review complete on a merged PR", pr.URL, "ticket")
	if err != nil || d.StageOfTask(got) != "reviewed" || got.PrURL == nil || *got.PrURL != pr.URL {
		t.Fatalf("merged PR rejected: %+v %v", got, err)
	}
	// A closed-unmerged PR is abandoned work and still blocks the transition.
	closed := trackerapi.PullRequest{URL: "https://forge/pull/1", Branch: "ticket", SHA: "agent-commit"}
	d.prEvidenceLookup = func(string, string) (trackerapi.PullRequest, error) { return closed, nil }
	if _, _, err = d.TransitionTaskStage(task.ID, "reviewed", "closed PR", closed.URL, "ticket"); err == nil {
		t.Fatal("closed-unmerged PR accepted")
	}
	// A merged PR whose head is not the agent checkout is still rejected.
	stale := pr
	stale.SHA = "other-commit"
	d.prEvidenceLookup = func(string, string) (trackerapi.PullRequest, error) { return stale, nil }
	if _, _, err = d.TransitionTaskStage(task.ID, "reviewed", "stale merged PR", stale.URL, "ticket"); err == nil {
		t.Fatal("merged PR without the checkout commit accepted")
	}
}

func TestAdjustmentOverridePrecedence(t *testing.T) {
	overrides := map[string]projectSkillOverride{"review": {content: "review"}, "create_pr": {content: "create"}}
	origin, conflicts := adjustmentOverrideOrigin(overrides)
	if origin != "review" || len(conflicts) != 0 {
		t.Fatal(origin, conflicts)
	}
	overrides["adjust"] = projectSkillOverride{content: "adjust"}
	origin, conflicts = adjustmentOverrideOrigin(overrides)
	if origin != "adjust" || len(conflicts) != 1 || overrides["review"].content != "review" {
		t.Fatal(origin, conflicts)
	}
}
func TestEarlierPRRecoveryPreservesImplemented(t *testing.T) {
	for _, timing := range []string{"specified", "implemented"} {
		t.Run(timing, func(t *testing.T) {
			repo := "/not-mounted-on-server"
			d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			no := false
			p, err := d.CreateProject(models.CreateProjectRequest{Name: "Recovery", RepoPath: repo, IssueTracker: "local", UseWorktrees: &no})
			if err != nil {
				t.Fatal(err)
			}
			_, err = d.UpdateProject(p.ID, models.UpdateProjectRequest{PRCreationStage: &timing})
			if err != nil {
				t.Fatal(err)
			}
			task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "recover", Labels: []string{"#implemented"}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = d.conn.Exec("UPDATE tasks SET branch_name='ticket',status='to_test',labels='[\"#implemented\"]' WHERE id=?", task.ID)
			if err != nil {
				t.Fatal(err)
			}
			pr := trackerapi.PullRequest{URL: "https://forge/pull/1", Branch: "ticket", SHA: "agent-commit", Open: true, Draft: true}
			d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
				if op.Action != "git_evidence" || op.TaskID != task.ID || op.ProjectID != p.ID {
					t.Fatalf("wrong evidence request: %#v", op)
				}
				return json.RawMessage(`{"sha":"agent-commit","branch":"ticket","clean":true}`), nil
			})
			d.prEvidenceLookup = func(string, string) (trackerapi.PullRequest, error) { return pr, nil }
			got, _, err := d.TransitionTaskStage(task.ID, timing, "PR recovery complete", pr.URL, "ticket")
			if err != nil || d.StageOfTask(got) != "implemented" || got.PrURL == nil || *got.PrURL != pr.URL {
				t.Fatalf("%+v %v", got, err)
			}
			if _, _, err = d.TransitionTaskStage(task.ID, "reviewed", "draft cannot complete", pr.URL, "ticket"); err == nil {
				t.Fatal("draft accepted")
			}
			pr.Draft = false
			got, _, err = d.TransitionTaskStage(task.ID, "reviewed", "checks and adjustment complete", pr.URL, "ticket")
			if err != nil || d.StageOfTask(got) != "reviewed" {
				t.Fatalf("%+v %v", got, err)
			}
			d.prEvidenceLookup = func(string, string) (trackerapi.PullRequest, error) { return pr, fmt.Errorf("forge unavailable") }
			if _, _, err = d.TransitionTaskStage(task.ID, "reviewed", "retry", pr.URL, "ticket"); err == nil {
				t.Fatal("lookup failure accepted")
			}
		})
	}
}
func TestAdjustmentReconciliationRetainsHistoryAndReset(t *testing.T) {
	root := t.TempDir()
	d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	no := false
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Custom", RepoPath: root, IssueTracker: "local", UseWorktrees: &no})
	if err != nil {
		t.Fatal(err)
	}
	d.ensureProjectSkillsTable()
	for _, id := range []string{"create_pr", "review"} {
		if _, err := d.conn.Exec("INSERT INTO project_skills(project_id,skill_id,content,updated_at) VALUES (?,?,?,?)", p.ID, id, "legacy "+id, time.Now().Format(time.RFC3339)); err != nil {
			t.Fatal(err)
		}
	}
	entry, err := d.projectSkillEntry(p.ID, "adjust")
	if err != nil || !entry.RequiresReconciliation || entry.OverrideOrigin != "review" || entry.Content != "legacy review" {
		t.Fatalf("%+v %v", entry, err)
	}
	entry, err = d.SaveProjectSkillContent(p.ID, "adjust", "Reviewed custom instructions")
	if err != nil || entry.RequiresReconciliation || entry.OverrideOrigin != "adjust" {
		t.Fatalf("%+v %v", entry, err)
	}
	for _, skill := range d.EffectiveProjectSkills(p.ID, "") {
		if skill.ID == "adjust" && (!strings.Contains(skill.Content, "Reviewed custom instructions") || !strings.Contains(skill.Content, "Never create a PR during adjustment")) {
			t.Fatal("custom contract weakened")
		}
	}
	entry, err = d.ResetProjectSkillContent(p.ID, "adjust")
	if err != nil || entry.RequiresReconciliation || entry.IsCustom {
		t.Fatalf("reset did not select default: %+v %v", entry, err)
	}
	saved := d.projectSkillOverrides(p.ID)
	if saved["create_pr"].content != "legacy create_pr" || saved["review"].content != "legacy review" {
		t.Fatal("legacy history deleted")
	}
}
func TestCompositePRPoliciesCreateBeforeAdjustment(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Composite"})
	if err != nil {
		t.Fatal(err)
	}
	for _, timing := range []string{"specified", "implemented"} {
		_, err = d.UpdateProject(p.ID, models.UpdateProjectRequest{PRCreationStage: &timing})
		if err != nil {
			t.Fatal(err)
		}
		for _, framework := range []string{"openspec", "speckit"} {
			for _, skill := range d.EffectiveProjectSkills(p.ID, framework) {
				if skill.ID != "pickup" && skill.ID != "pickup_issues" && skill.ID != "implement" && skill.ID != "specify" {
					continue
				}
				if !strings.Contains(skill.Content, "PR creation stage: "+timing) {
					t.Fatal("policy missing")
				}
				if timing == "implemented" && !strings.Contains(skill.Content, "After successful implementation checks") {
					t.Fatal("creation moved after adjustment")
				}
				if skill.ID == "pickup" || skill.ID == "pickup_issues" {
					if !strings.Contains(skill.Content, "Never create a PR during adjustment") || !strings.Contains(skill.Content, "Stop before merge") {
						t.Fatal("composite boundary missing")
					}
				}
			}
		}
	}
}

func TestWebWorkflowSkillNamesAndTransitions(t *testing.T) {
	expected := []struct{ id, name, from, to string }{
		{"clarify", "Clarify", "new", "clarified"},
		{"specify", "Specify", "clarified", "specified"},
		{"implement", "Implement", "specified", "implemented"},
		{"adjust", "Adjust", "implemented", "reviewed"},
		{"handoff", "Handoff", "reviewed", "finished"},
	}
	for _, framework := range []string{"openspec", "speckit"} {
		templates := ProjectSkillTemplates(framework)
		for i, want := range expected {
			stage := StageSkills[i]
			if stage.ID != want.id || stage.Name != want.name || stage.FromStage != want.from || stage.ToStage != want.to || templates[i].Name != want.name {
				t.Fatalf("%s: stage %+v, template name %q, want %+v", framework, stage, templates[i].Name, want)
			}
		}
	}
}

func TestCreatePRIsIndependentOfWorkflow(t *testing.T) {
	for _, alias := range []string{"create_pr", "create-pr"} {
		s, ok := StageSkillByID(alias)
		if !ok || s.ID != "create_pr" || s.Command != "/create-pr" || s.FromStage != "" || s.ToStage != "" || skillStageLabel[s.ID] != "" {
			t.Fatalf("standalone creation was mapped to a workflow stage: %+v", s)
		}
	}
	overrides := map[string]projectSkillOverride{"create_pr": {content: "Custom PR creation"}}
	if origin, _ := adjustmentOverrideOrigin(overrides); origin != "" {
		t.Fatalf("creation customization leaked into Adjust: %s", origin)
	}
}
