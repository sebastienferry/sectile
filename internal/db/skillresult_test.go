package db

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
)

func successfulChecks() []skillCheck {
	zero := 0
	return []skillCheck{
		{Kind: "build", Command: "go build ./...", ExitCode: &zero, Output: "exit 0"},
		{Kind: "lint", Command: "go vet ./...", ExitCode: &zero, Output: "exit 0"},
		{Kind: "test", Command: "go test ./...", ExitCode: &zero, Output: "ok"},
	}
}

func TestSkillReceiptMustBelongToCurrentRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.json")
	for _, tc := range []struct {
		name, body string
		ok         bool
	}{
		{"empty", "", false},
		{"prose", "I am blocked", false},
		{"old run", `{"runId":"old","outcome":"completed","summary":"done"}`, false},
		{"blocked", `{"runId":"current","outcome":"blocked","summary":"decision needed"}`, false},
		{"transient", `{"runId":"current","outcome":"retryable","summary":"forge unavailable"}`, false},
		{"no summary", `{"runId":"current","outcome":"completed"}`, false},
		{"complete", `{"runId":"current","outcome":"completed","summary":"scope settled"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := readSkillResult(path, "current")
			if (err == nil) != tc.ok {
				t.Fatalf("read result: %v", err)
			}
		})
	}
}

func TestSkillChecksRejectMissingAndFailedEvidence(t *testing.T) {
	for _, name := range []string{"passed", "missing test", "failed test", "no exit code", "no output", "failed and skipped", "not applicable"} {
		t.Run(name, func(t *testing.T) {
			checks := successfulChecks()
			wantOK := false
			switch name {
			case "passed":
				wantOK = true
			case "missing test":
				checks = checks[:2]
			case "failed test":
				one := 1
				checks[2].ExitCode = &one
			case "no exit code":
				checks[2].ExitCode = nil
			case "no output":
				checks[2].Output = ""
			case "failed and skipped":
				checks[2].SkipReason = "pre-existing failure"
			case "not applicable":
				checks[1] = skillCheck{Kind: "lint", SkipReason: "repository defines no static analyzer"}
				wantOK = true
			}
			if err := validateSkillChecks(checks, "build", "lint", "test"); (err == nil) != wantOK {
				t.Fatalf("check validation: %v", err)
			}
		})
	}
	if err := validateSkillChecks([]skillCheck{{Kind: "merge", SkipReason: "no remote"}}, "merge"); err == nil {
		t.Fatal("handoff must never skip merge confirmation")
	}
}

func testSkillRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "main"}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "initial"}, {"switch", "-c", "ticket"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v: %s", err, out)
		}
	}
	return repo
}

func TestSkillResultRequiresArtifactsBranchAndForgePR(t *testing.T) {
	repo := testSkillRepo(t)
	result := &skillResult{Branch: "ticket", Checks: successfulChecks()}
	noPR := func(string, string) string { return "" }
	if err := validateSkillResult(result, "specify", repo, noPR); err == nil {
		t.Fatal("missing specification was accepted")
	}
	for _, name := range []string{"spec.md", "tasks.md", "plan.md"} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte("Acceptance criteria"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	result.Artifacts = []string{"spec.md", "tasks.md", "plan.md"}
	if err := validateSkillResult(result, "specify", repo, noPR); err != nil {
		t.Fatal(err)
	}
	result.Artifacts = []string{"../outside.md"}
	if err := validateSkillResult(result, "specify", repo, noPR); err == nil {
		t.Fatal("out-of-repo artifact accepted")
	}
	result.Branch = "main"
	if err := validateSkillResult(result, "implement", repo, noPR); err == nil {
		t.Fatal("wrong branch accepted")
	}
	result.Branch = "ticket"
	for _, args := range [][]string{{"add", "spec.md", "tasks.md", "plan.md"}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "spec"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v: %s", err, out)
		}
	}
	if err := validateSkillResult(result, "create_pr", repo, noPR); err == nil {
		t.Fatal("missing PR accepted")
	}
	result.PRURL = "https://github.com/example/repo/pull/42"
	if err := validateSkillResult(result, "create_pr", repo, noPR); err == nil {
		t.Fatal("unconfirmed PR accepted")
	}
	lookup := func(_, branch string) string {
		if branch != "ticket" {
			t.Fatalf("looked up wrong branch: %s", branch)
		}
		return result.PRURL
	}
	if err := validateSkillResult(result, "create_pr", repo, lookup); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "unfinished.go"), []byte("unfinished"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateSkillResult(result, "create_pr", repo, lookup); err == nil {
		t.Fatal("PR accepted with uncommitted changes")
	}
}

// An already-running agent reports a successful turn even when its answer is a
// blocker. Exercise the real worker and the PTY path, not just the JSON parser.
type receiptTerminal struct {
	TerminalSessionRunner
	respond func(string)
}

func (f receiptTerminal) AgentLaunched(string) bool { return true }
func (f receiptTerminal) RunInAgentSession(_ context.Context, _, line string, _ time.Duration) (string, int, bool, error) {
	f.respond(line)
	return "Agent turn finished", 0, false, nil
}

func TestWorkerAdvancesOnlyAfterVerifiedCompletion(t *testing.T) {
	for _, outcome := range []string{"missing", "blocked", "retryable", "completed", "missing-pr", "persistence-failure"} {
		t.Run(outcome, func(t *testing.T) {
			repo := testSkillRepo(t)
			d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			no := false
			project, err := d.CreateProject(models.CreateProjectRequest{Name: "Result", Slug: "result", RepoPath: repo, IssueTracker: "local", UseWorktrees: &no})
			if err != nil {
				t.Fatal(err)
			}
			stage, skillID := "new", "clarify"
			if outcome == "missing-pr" {
				stage, skillID = "implemented", "create_pr"
			}
			task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Receipt test", Labels: []string{"#" + stage}})
			if err != nil {
				t.Fatal(err)
			}
			branch := "ticket"
			initialStatus, _ := InternalStatusForStage(stage)
			labels, _ := json.Marshal([]string{"#" + stage})
			if _, err := d.conn.Exec("UPDATE tasks SET branch_name = ?, status = ?, labels = ? WHERE id = ?", branch, initialStatus, string(labels), task.ID); err != nil {
				t.Fatal(err)
			}
			activity := models.TaskActivity{ID: "receipt-run", TaskID: task.ID, SkillID: skillID, Status: "queued", CreatedAt: time.Now()}
			if err := d.addTaskActivityDirect(activity); err != nil {
				t.Fatal(err)
			}
			if outcome == "persistence-failure" {
				if _, err := d.conn.Exec(`CREATE TRIGGER reject_completion
					BEFORE UPDATE ON task_activities WHEN NEW.status = 'completed'
					BEGIN SELECT RAISE(ABORT, 'test persistence failure'); END`); err != nil {
					t.Fatal(err)
				}
			}
			d.SetTerminalRunner(receiptTerminal{respond: func(line string) {
				if !strings.Contains(line, "user context preserved") {
					t.Fatal("PTY lost additional instructions")
				}
				match := regexp.MustCompile(`absolute file ("[^"]+")`).FindStringSubmatch(line)
				if len(match) != 2 {
					t.Fatalf("PTY lost result-file contract: %s", line)
				}
				if _, _, err := d.TransitionTaskStage(task.ID, "finished", "premature", "", ""); err == nil {
					t.Fatal("standalone transition bypassed worker result gate")
				}
				finished := "finished"
				if _, _, err := d.PostBackTask(models.TaskPostBackPayload{TaskID: task.ID, Stage: &finished}); err == nil {
					t.Fatal("post-back bypassed worker result gate")
				}
				if outcome == "missing" {
					return
				}
				var path string
				if err := json.Unmarshal([]byte(match[1]), &path); err != nil {
					t.Fatal(err)
				}
				r := skillResult{RunID: activity.ID, Outcome: outcome, Summary: "Scope or blocker recorded", Branch: branch, Checks: successfulChecks()}
				if outcome == "missing-pr" || outcome == "persistence-failure" {
					r.Outcome = "completed"
				}
				raw, _ := json.Marshal(r)
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}})
			// Failed runs must not enqueue another autonomous step.
			d.processSkillJob(SkillJob{TaskID: task.ID, ActivityID: activity.ID, SkillID: skillID, Prompt: "user context preserved", AutoChain: outcome != "completed"})
			updated, err := d.GetTaskByID(task.ID)
			if err != nil {
				t.Fatal(err)
			}
			wantStage, wantStatus := stage, "failed"
			if outcome == "completed" {
				wantStage, wantStatus = "clarified", "completed"
			}
			if got := d.StageOfTask(updated); got != wantStage {
				t.Fatalf("stage = %s; want %s", got, wantStage)
			}
			var status string
			if err := d.conn.QueryRow("SELECT status FROM task_activities WHERE id = ?", activity.ID).Scan(&status); err != nil || status != wantStatus {
				t.Fatalf("activity = %s; want %s; error %v", status, wantStatus, err)
			}
			if out, err := exec.Command("git", "-C", repo, "branch", "--show-current").Output(); err != nil || strings.TrimSpace(string(out)) != branch {
				t.Fatalf("worker switched/merged the checkout: %s, %v", out, err)
			}
		})
	}
}
