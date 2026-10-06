package runner_test

import (
	"os"
	"testing"

	"tasks/internal/runner"
)

func TestGetCwdGitStatus(t *testing.T) {
	r := runner.NewRunner()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	status, err := r.GetCwdGitStatus(cwd)
	if err != nil {
		t.Fatalf("GetCwdGitStatus failed: %v", err)
	}

	if status == nil {
		t.Fatal("Expected status to not be nil")
	}

	if !status.IsGitRepo {
		t.Errorf("Expected current workspace to be detected as a git repo, got isGitRepo=false")
	}

	if status.Branch == "" {
		t.Errorf("Expected branch name to not be empty")
	}

	t.Logf("Detected git status: branch=%s, clean=%v, modified=%d, untracked=%d",
		status.Branch, status.IsClean, status.ModifiedCount, status.UntrackedCount)
}

func TestOpenInEditor(t *testing.T) {
	r := runner.NewRunner()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Use 'echo' or 'true' as mock editor command
	if err := r.OpenInEditor("echo", cwd); err != nil {
		t.Fatalf("OpenInEditor failed with echo: %v", err)
	}

	// Test FindEditorBinary for code and zed
	bin, _ := runner.FindEditorBinary("code")
	if bin == "" {
		t.Errorf("Expected FindEditorBinary(code) to return a binary or command")
	}
	t.Logf("FindEditorBinary(code) resolved to: %s", bin)
}
