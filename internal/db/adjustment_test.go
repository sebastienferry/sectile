package db

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"tasks/internal/models"
	"tasks/internal/runner"
	"testing"
	"time"
)

func TestAdjustmentAliasesAndHumanBoundary(t *testing.T) {
	for _, id := range []string{"adjust", "adjust-issue", "create_pr", "create-pr", "review"} {
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
	good := runner.PullRequestEvidence{URL: "https://forge/pull/1", Branch: "ticket", Open: true}
	for _, kind := range []string{"valid", "draft", "closed", "branch", "replacement"} {
		t.Run(kind, func(t *testing.T) {
			p := good
			switch kind {
			case "draft":
				p.Draft = true
			case "closed":
				p.Open = false
			case "branch":
				p.Branch = "other"
			case "replacement":
				p.URL = "https://forge/pull/2"
			}
			err := validatePullRequestEvidence(p, "ticket", p.URL, good.URL, true)
			if (err == nil) != (kind == "valid") {
				t.Fatalf("%s: %v", kind, err)
			}
		})
	}
}
func TestAdjustmentOverridePrecedence(t *testing.T) {
	overrides := map[string]projectSkillOverride{"review": {content: "review"}, "create_pr": {content: "create"}}
	origin, conflicts := adjustmentOverrideOrigin(overrides)
	if origin != "create_pr" || len(conflicts) != 1 {
		t.Fatal(origin, conflicts)
	}
	overrides["adjust"] = projectSkillOverride{content: "adjust"}
	origin, conflicts = adjustmentOverrideOrigin(overrides)
	if origin != "adjust" || len(conflicts) != 2 || overrides["review"].content != "review" {
		t.Fatal(origin, conflicts)
	}
}
func TestEarlierPRRecoveryPreservesImplemented(t *testing.T) {
	for _, timing := range []string{"specified", "implemented"} {
		t.Run(timing, func(t *testing.T) {
			repo := testSkillRepo(t)
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
			for _, args := range [][]string{{"add", "."}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "test: preserve generated context"}} {
				if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
					t.Fatalf("%s %v", out, err)
				}
			}
			head, _ := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
			pr := runner.PullRequestEvidence{URL: "https://forge/pull/1", Branch: "ticket", SHA: strings.TrimSpace(string(head)), Open: true, Draft: true}
			d.prEvidenceLookup = func(string, string) (runner.PullRequestEvidence, error) { return pr, nil }
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
			d.prEvidenceLookup = func(string, string) (runner.PullRequestEvidence, error) { return pr, fmt.Errorf("forge unavailable") }
			if _, _, err = d.TransitionTaskStage(task.ID, "reviewed", "retry", pr.URL, "ticket"); err == nil {
				t.Fatal("lookup failure accepted")
			}
		})
	}
}
func TestLegacyForwarderPreservesDivergence(t *testing.T) {
	root := t.TempDir()
	if err := installAdjustmentForwarders(root); err != nil {
		t.Fatal(err)
	}
	if err := installAdjustmentForwarders(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(SkillDirsFor(root, "create-pr")[0], "SKILL.md")
	if err := os.WriteFile(path, []byte("personal work"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := installAdjustmentForwarders(root); err == nil {
		t.Fatal("divergence unreported")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "personal work" {
		t.Fatal("overwritten")
	}
}

func TestManagedAdjustmentPinsPRAndStopsAtReview(t *testing.T) {
	for _, outcome := range []string{"ready", "draft", "replacement", "failed-check", "feedback-failure"} {
		t.Run(outcome, func(t *testing.T) {
			repo := testSkillRepo(t)
			d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			no := false
			p, err := d.CreateProject(models.CreateProjectRequest{Name: "Adjust", RepoPath: repo, IssueTracker: "local", UseWorktrees: &no})
			if err != nil {
				t.Fatal(err)
			}
			task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "adjust", Labels: []string{"#implemented"}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = d.conn.Exec("UPDATE tasks SET branch_name='ticket',status='to_test',labels='[\"#implemented\"]' WHERE id=?", task.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"add", "."}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "test: context"}} {
				if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
					t.Fatalf("%s %v", out, err)
				}
			}
			head, _ := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
			pr := runner.PullRequestEvidence{URL: "https://forge/pull/1", Branch: "ticket", SHA: strings.TrimSpace(string(head)), Open: true, Draft: true}
			d.prEvidenceLookup = func(string, string) (runner.PullRequestEvidence, error) { return pr, nil }
			activity := models.TaskActivity{ID: "adjust-run", TaskID: task.ID, SkillID: "create_pr", Status: "queued", CreatedAt: time.Now()}
			if err := d.addTaskActivityDirect(activity); err != nil {
				t.Fatal(err)
			}
			d.SetTerminalRunner(receiptTerminal{respond: func(line string) {
				match := regexp.MustCompile(`absolute file ("[^"]+")`).FindStringSubmatch(line)
				if len(match) != 2 {
					t.Fatal("missing receipt contract")
				}
				var path string
				if err := json.Unmarshal([]byte(match[1]), &path); err != nil {
					t.Fatal(err)
				}
				pr.Draft = outcome == "draft"
				if outcome == "replacement" {
					pr.URL = "https://forge/pull/2"
				}
				receipt := skillResult{RunID: activity.ID, Outcome: "completed", Summary: "Complete diff reviewed; no human feedback; checks passed", Branch: "ticket", PRURL: pr.URL, Checks: successfulChecks()}
				if outcome == "failed-check" {
					code := 1
					receipt.Checks[0].ExitCode = &code
				}
				if outcome == "feedback-failure" {
					receipt.Outcome = "retryable"
					receipt.Summary = "Feedback retrieval failed; retry required"
				}
				raw, _ := json.Marshal(receipt)
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}})
			d.processSkillJob(SkillJob{TaskID: task.ID, ActivityID: activity.ID, SkillID: "create_pr", AutoChain: true})
			got, _ := d.GetTaskByID(task.ID)
			want := "implemented"
			if outcome == "ready" {
				want = "reviewed"
			}
			if d.StageOfTask(got) != want {
				t.Fatalf("stage %s want %s", d.StageOfTask(got), want)
			}
			if len(d.jobQueue) != 0 {
				t.Fatal("autonomous adjustment continued beyond review")
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
	if err != nil || !entry.RequiresReconciliation || entry.OverrideOrigin != "create_pr" || entry.LegacyContents["review"] != "legacy review" {
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
