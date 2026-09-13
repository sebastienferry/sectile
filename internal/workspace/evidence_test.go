package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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
