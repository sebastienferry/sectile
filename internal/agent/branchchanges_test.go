package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

// clonedOrigin creates a bare origin on disk with one commit on main and a
// clone of it, whose origin/HEAD names main as a clone sets it.
func clonedOrigin(t *testing.T) (origin, clone string) {
	t.Helper()
	origin = filepath.Join(t.TempDir(), "tools.git")
	gitTest(t, filepath.Dir(origin), "init", "-q", "--bare", "-b", "main", origin)
	seed := checkoutOf(t, origin)
	gitTest(t, seed, "push", "-q", "origin", "main")
	clone = filepath.Join(t.TempDir(), "tools")
	gitTest(t, filepath.Dir(clone), "clone", "-q", origin, clone)
	return origin, clone
}

// pushFromElsewhere pushes branch with one commit of its own to origin from
// another clone, as another workstation would.
func pushFromElsewhere(t *testing.T, origin, branch string) {
	t.Helper()
	other := filepath.Join(t.TempDir(), "other")
	gitTest(t, filepath.Dir(other), "clone", "-q", origin, other)
	gitTest(t, other, "checkout", "-q", "-b", branch)
	gitTest(t, other, "commit", "-q", "--allow-empty", "-m", "work elsewhere")
	gitTest(t, other, "push", "-q", "origin", branch)
}

func TestBranchChangesCountsTheCommitsOfEveryRef(t *testing.T) {
	ctx := context.Background()
	const branch = "feat/12"
	for name, tc := range map[string]struct {
		setup  func(t *testing.T, origin, clone string)
		exists bool
		ahead  int
	}{
		"local branch with no commit of its own": {
			setup:  func(t *testing.T, _, clone string) { gitTest(t, clone, "branch", branch) },
			exists: true,
		},
		"pushed branch with no commit of its own": {
			setup: func(t *testing.T, _, clone string) {
				gitTest(t, clone, "branch", branch)
				gitTest(t, clone, "push", "-q", "origin", branch)
			},
			exists: true,
		},
		"local branch with a commit": {
			setup: func(t *testing.T, _, clone string) {
				gitTest(t, clone, "checkout", "-q", "-b", branch)
				gitTest(t, clone, "commit", "-q", "--allow-empty", "-m", "work")
			},
			exists: true, ahead: 1,
		},
		// The local branch has nothing, origin has a commit: the branch changed.
		"local branch empty, fetched origin branch with a commit": {
			setup: func(t *testing.T, origin, clone string) {
				gitTest(t, clone, "branch", branch)
				pushFromElsewhere(t, origin, branch)
				gitTest(t, clone, "fetch", "-q", "origin")
			},
			exists: true, ahead: 1,
		},
		"branch cut from an older default branch": {
			setup: func(t *testing.T, _, clone string) {
				gitTest(t, clone, "branch", branch)
				gitTest(t, clone, "commit", "-q", "--allow-empty", "-m", "main moves on")
				gitTest(t, clone, "push", "-q", "origin", "main")
			},
			exists: true,
		},
		"branch nowhere": {setup: func(*testing.T, string, string) {}},
	} {
		t.Run(name, func(t *testing.T) {
			origin, clone := clonedOrigin(t)
			tc.setup(t, origin, clone)
			defaultBranch, exists, ahead, err := branchChanges(ctx, clone, branch)
			if err != nil {
				t.Fatal(err)
			}
			if defaultBranch != "main" || exists != tc.exists || ahead != tc.ahead {
				t.Fatalf("default=%q exists=%v ahead=%d, want main %v %d", defaultBranch, exists, ahead, tc.exists, tc.ahead)
			}
		})
	}
}

// Every answer the agent cannot give for sure is an error, so the server
// never reads it as a repository left unchanged.
func TestBranchChangesFailsRatherThanGuess(t *testing.T) {
	ctx := context.Background()
	const branch = "feat/12"
	for name, setup := range map[string]func(t *testing.T, origin, clone string){
		"origin holds the branch, not fetched here": func(t *testing.T, origin, _ string) {
			pushFromElsewhere(t, origin, branch)
		},
		"origin/HEAD unset": func(t *testing.T, _, clone string) {
			gitTest(t, clone, "remote", "set-head", "origin", "-d")
		},
		"origin unreachable": func(t *testing.T, origin, clone string) {
			gitTest(t, clone, "branch", branch)
			if err := os.RemoveAll(origin); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			origin, clone := clonedOrigin(t)
			setup(t, origin, clone)
			if _, exists, ahead, err := branchChanges(ctx, clone, branch); err == nil {
				t.Fatalf("exists=%v ahead=%d, want an error", exists, ahead)
			}
		})
	}
	if _, _, _, err := branchChanges(ctx, t.TempDir(), ""); err == nil {
		t.Fatal("an empty branch must be refused")
	}
}

func TestRepositoryCheckoutMatchesTheOriginWhateverIsCheckedOut(t *testing.T) {
	ctx := context.Background()
	origin, clone := clonedOrigin(t)
	repository := models.RepositoryIdentity(origin)
	other := checkoutOf(t, "git@github.com:acme/app.git")
	got, found := repositoryCheckout(ctx, repository, []string{"", "relative", "/does/not/exist", t.TempDir(), other, clone})
	if !found || got != filepath.Clean(clone) {
		t.Fatalf("checkout=%q found=%v, want %s", got, found, clone)
	}
	if _, found := repositoryCheckout(ctx, repository, []string{other}); found {
		t.Fatal("a checkout of another repository must not match")
	}
}

func TestBranchChangesOperationEchoesTheRepository(t *testing.T) {
	ctx := context.Background()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	const branch = "feat/12"
	root := checkoutOf(t, "git@github.com:acme/app.git")
	origin, clone := clonedOrigin(t)
	gitTest(t, clone, "branch", branch)
	repository := models.RepositoryIdentity(origin)

	project := "app"
	task := models.Task{ID: "task", Key: "#12", ProjectID: project, BranchName: func() *string { b := branch; return &b }(), RepoPath: &clone}
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

	op := agentprotocol.Operation{ProjectID: project, TaskID: task.ID, Action: "branch_changes", Branch: branch, Repository: repository}
	value, err := daemon.executeOperation(ctx, op)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.(branchChangesAnswer); got != (branchChangesAnswer{Repository: repository, Found: true, DefaultBranch: "main", Exists: true}) {
		t.Fatalf("answer = %+v", got)
	}

	op.Repository = "gitlab.com/group/elsewhere"
	if value, err = daemon.executeOperation(ctx, op); err != nil || value.(branchChangesAnswer) != (branchChangesAnswer{Repository: op.Repository}) {
		t.Fatalf("an unknown repository must be reported as not found: %#v, %v", value, err)
	}
}
