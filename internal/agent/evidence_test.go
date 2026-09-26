package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// checkoutOf creates a repository whose origin is remote, with one commit.
func checkoutOf(t *testing.T, remote string) string {
	t.Helper()
	dir := t.TempDir()
	gitTest(t, dir, "init", "-q", "-b", "main")
	gitTest(t, dir, "remote", "add", "origin", remote)
	gitTest(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	return dir
}

func TestVerifiedCheckoutFindsTheBranchInALinkedWorktree(t *testing.T) {
	ctx := context.Background()
	const repository = "gitlab.com/smartadserver/private/arch/argocd-arch"
	const branch = "feature/SFE-360-remove-arch-api"
	other := checkoutOf(t, "git@gitlab.com:smartadserver/private/sfe.git")
	gitTest(t, other, "branch", branch)
	arch := checkoutOf(t, "git@gitlab.com:smartadserver/private/arch/argocd-arch.git")
	worktree := filepath.Join(t.TempDir(), "SFE-360")
	gitTest(t, arch, "worktree", "add", "-q", "-b", branch, worktree)
	gitTest(t, worktree, "commit", "-q", "--allow-empty", "-m", "work")
	head := gitTest(t, worktree, "rev-parse", "HEAD")

	// The first candidate carries the branch but another origin: a server hint
	// is never trusted over the checkout's own remote.
	got, found, err := verifiedCheckout(ctx, repository, branch, []string{"", "relative/path", "/does/not/exist", other, arch})
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	gotPath, _ := filepath.EvalSymlinks(got.Path)
	wantPath, _ := filepath.EvalSymlinks(worktree)
	if gotPath != wantPath || got.SHA != head || got.Branch != branch || !got.Clean {
		t.Fatalf("checkout = %+v, want %s at %s", got, worktree, head)
	}

	if err := os.WriteFile(filepath.Join(worktree, "dirty"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _, _ = verifiedCheckout(ctx, repository, branch, []string{arch}); got.Clean {
		t.Fatal("an untracked file must make the checkout dirty")
	}
}

func TestVerifiedCheckoutReportsNoneWithoutFailing(t *testing.T) {
	ctx := context.Background()
	arch := checkoutOf(t, "https://gitlab.com/smartadserver/private/arch/argocd-arch.git")
	for name, candidates := range map[string][]string{
		"no candidate":            nil,
		"branch not checked out":  {arch},
		"only another repository": {checkoutOf(t, "git@github.com:acme/app.git")},
		"not a repository":        {t.TempDir()},
	} {
		if _, found, err := verifiedCheckout(ctx, "gitlab.com/smartadserver/private/arch/argocd-arch", "topic", candidates); found || err != nil {
			t.Errorf("%s: found=%v err=%v", name, found, err)
		}
	}
}

func TestWorktreeOnBranch(t *testing.T) {
	porcelain := "worktree /repo\nHEAD 1\nbranch refs/heads/main\n\nworktree /repo/.tasks/worktrees/x\nHEAD 2\nbranch refs/heads/topic\n\nworktree /detached\nHEAD 3\ndetached\n"
	if got := worktreeOnBranch(porcelain, "topic"); got != "/repo/.tasks/worktrees/x" {
		t.Fatalf("topic worktree = %q", got)
	}
	if got := worktreeOnBranch(porcelain, "main"); got != "/repo" {
		t.Fatalf("main worktree = %q", got)
	}
	if got := worktreeOnBranch(porcelain, "top"); got != "" {
		t.Fatalf("prefix must not match, got %q", got)
	}
	// A worktree whose directory was deleted is listed as prunable, not usable.
	stale := "worktree /repo\nHEAD 1\nbranch refs/heads/main\n\nworktree /gone\nHEAD 2\nbranch refs/heads/topic\nprunable gitdir file points to non-existent location\n"
	if got := worktreeOnBranch(stale, "topic"); got != "" {
		t.Fatalf("prunable worktree used: %q", got)
	}
}

func TestEvidenceOperationsForAProjectWithoutRemote(t *testing.T) {
	ctx := context.Background()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	const branch = "feature/SFE-360-remove-arch-api"
	// The SFE shape: a coordination checkout with no remote at all.
	root := t.TempDir()
	gitTest(t, root, "init", "-q", "-b", branch)
	gitTest(t, root, "commit", "-q", "--allow-empty", "-m", "init")
	arch := checkoutOf(t, "git@gitlab.com:smartadserver/private/arch/argocd-arch.git")
	gitTest(t, arch, "checkout", "-q", "-b", branch)
	head := gitTest(t, arch, "rev-parse", "HEAD")

	project := "sfe"
	task := models.Task{ID: "task", Key: "SFE-360", ProjectID: project, BranchName: func() *string { b := branch; return &b }(), RepoPath: &arch}
	config := agentconfig.Config{SchemaVersion: 1, ProjectID: project, AIProvider: "claude"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/agent/config":
			_ = json.NewEncoder(w).Encode(config)
		case "/api/projects/" + project:
			_ = json.NewEncoder(w).Encode(models.Project{ID: project, RepoPath: root})
		default:
			_ = json.NewEncoder(w).Encode(task)
		}
	}))
	defer srv.Close()
	daemon := &agentDaemon{repoRoot: root, link: serverLink{serverURL: srv.URL, token: "token", projectID: project}}

	op := agentprotocol.Operation{ProjectID: project, TaskID: task.ID, Action: "git_evidence", Branch: branch, Repository: "gitlab.com/smartadserver/private/arch/argocd-arch"}
	value, err := daemon.executeOperation(ctx, op)
	if err != nil {
		t.Fatal(err)
	}
	evidence := value.(map[string]any)
	if evidence["repository"] != op.Repository || evidence["found"] != true || evidence["sha"] != head || evidence["clean"] != true {
		t.Fatalf("evidence: %#v", evidence)
	}

	op.Repository = "gitlab.com/group/elsewhere"
	if value, err = daemon.executeOperation(ctx, op); err != nil || value.(map[string]any)["found"] != false {
		t.Fatalf("an unknown repository must be reported as not found: %#v, %v", value, err)
	}

	// Without a repository, the lookup on the remote-less checkout names the
	// missing origin instead of a git exit status.
	op = agentprotocol.Operation{ProjectID: project, TaskID: task.ID, Action: "pr_evidence", Branch: branch}
	if _, err = daemon.executeOperation(ctx, op); err == nil || !strings.Contains(err.Error(), "no origin remote") {
		t.Fatalf("expected the missing origin to be named, got %v", err)
	}
}

func TestForeignWorkDirFallsBackToTheProjectRoot(t *testing.T) {
	root, existing := t.TempDir(), t.TempDir()
	missing := filepath.Join(root, ".tasks", "worktrees", "SFE-360")
	for name, tc := range map[string]struct {
		target string
		err    error
		want   string
	}{
		"existing checkout":   {existing, nil, existing},
		"checkout to create":  {missing, nil, root},
		"mismatched checkout": {"", os.ErrInvalid, root},
	} {
		if got := foreignWorkDir(tc.target, root, tc.err); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}
