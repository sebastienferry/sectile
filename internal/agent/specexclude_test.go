package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/testhome"
)

// ignored reports whether Git ignores path, relative to dir.
func ignored(t *testing.T, dir, path string) bool {
	t.Helper()
	err := exec.Command("git", "-C", dir, "check-ignore", "-q", "--no-index", path).Run()
	if err == nil {
		return true
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return false
	}
	t.Fatalf("git check-ignore %s: %v", path, err)
	return false
}

func readExclude(t *testing.T, repo string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(raw)
}

func writeExclude(t *testing.T, repo, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, ".git", "info", "exclude"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestArtefactPatterns(t *testing.T) {
	if got := artefactPatterns("#487"); !slices.Equal(got, []string{"/docs/clarifications/487.md", "/openspec/changes/487-*/", "/specs/487-*/"}) {
		t.Errorf("#487: %v", got)
	}
	got := artefactPatterns("SFE-12")
	for _, want := range []string{"/specs/SFE-12-*/", "/specs/sfe-12-*/", "/openspec/changes/sfe-12-*/", "/docs/clarifications/SFE-12.md"} {
		if !slices.Contains(got, want) {
			t.Errorf("SFE-12 lacks %s: %v", want, got)
		}
	}
	for _, key := range []string{"", "#", "a b", "x*", "a\nb", "../x", "a/b", "-1", "[1]", "!1"} {
		if got := artefactPatterns(key); got != nil {
			t.Errorf("%q must yield no pattern: %v", key, got)
		}
	}
}

// The block lives in the common directory, so a linked worktree sees it; it is
// written once, grows task by task in order, and ignores only each task's own
// artefacts.
func TestEnsureSpecExclusionsInTheCommonDirectory(t *testing.T) {
	ctx := context.Background()
	repo := checkoutOf(t, "git@github.com:o/a.git")
	worktree := filepath.Join(t.TempDir(), "487")
	gitTest(t, repo, "worktree", "add", "-q", "-b", "feat/487", worktree)

	covered, err := ensureSpecExclusions(ctx, worktree, "p1", "#487")
	if err != nil || !covered {
		t.Fatalf("ensure: %v %v", covered, err)
	}
	for _, path := range []string{"specs/487-drop/spec.md", "openspec/changes/487-drop/proposal.md", "docs/clarifications/487.md"} {
		if !ignored(t, worktree, path) {
			t.Errorf("%s must be ignored in the worktree", path)
		}
	}
	for _, path := range []string{"specs/488-x/spec.md", "specs/4870-x/spec.md", "docs/clarifications/488.md", "src/specs/487-x/a.md"} {
		if ignored(t, worktree, path) {
			t.Errorf("%s must not be ignored", path)
		}
	}

	path := filepath.Join(repo, ".git", "info", "exclude")
	before, _ := os.Stat(path)
	time.Sleep(10 * time.Millisecond)
	if _, err := ensureSpecExclusions(ctx, worktree, "p1", "487"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("an unchanged block must not be rewritten")
	}

	if _, err := ensureSpecExclusions(ctx, repo, "p1", "#12"); err != nil {
		t.Fatal(err)
	}
	want := "# >>> sectile: dropped specification artefacts, project p1 >>>\n" +
		"/docs/clarifications/12.md\n/docs/clarifications/487.md\n" +
		"/openspec/changes/12-*/\n/openspec/changes/487-*/\n" +
		"/specs/12-*/\n/specs/487-*/\n" +
		"# <<< sectile: dropped specification artefacts, project p1 <<<\n"
	if got := readExclude(t, repo); !strings.HasSuffix(got, want) || strings.Count(got, ">>> sectile") != 1 {
		t.Errorf("exclude file:\n%s", got)
	}
}

// Every line outside the block is the user's and is kept byte for byte, when
// the block is added, grown and removed.
func TestSpecExclusionsLeaveTheUserLinesAlone(t *testing.T) {
	ctx := context.Background()
	repo := checkoutOf(t, "git@github.com:o/a.git")
	above := "# git ls-files --others --exclude-from=.git/info/exclude\n*.log\n"
	writeExclude(t, repo, above)

	if _, err := ensureSpecExclusions(ctx, repo, "p1", "#1"); err != nil {
		t.Fatal(err)
	}
	below := "\n/tmp/\n  spaced  \n"
	writeExclude(t, repo, readExclude(t, repo)+below)
	if _, err := ensureSpecExclusions(ctx, repo, "p1", "#2"); err != nil {
		t.Fatal(err)
	}
	got := readExclude(t, repo)
	if !strings.HasPrefix(got, above) || !strings.HasSuffix(got, below) || !strings.Contains(got, "/specs/2-*/") {
		t.Fatalf("growing the block moved the user's lines:\n%q", got)
	}
	if err := removeSpecExclusions(ctx, repo, "p1"); err != nil {
		t.Fatal(err)
	}
	if got := readExclude(t, repo); got != above+below {
		t.Fatalf("removing the block must leave exactly the user's lines:\n%q\nwant\n%q", got, above+below)
	}
	if err := removeSpecExclusions(ctx, repo, "p1"); err != nil {
		t.Fatalf("removing an absent block: %v", err)
	}
}

// Two projects sharing a checkout each own their block.
func TestSpecExclusionsOfTwoProjectsAreIndependent(t *testing.T) {
	ctx := context.Background()
	repo := checkoutOf(t, "git@github.com:o/a.git")
	for _, call := range []struct{ project, key string }{{"p1", "#1"}, {"p2", "PX-2"}, {"p1", "#3"}} {
		if _, err := ensureSpecExclusions(ctx, repo, call.project, call.key); err != nil {
			t.Fatal(err)
		}
	}
	if err := removeSpecExclusions(ctx, repo, "p1"); err != nil {
		t.Fatal(err)
	}
	got := readExclude(t, repo)
	if strings.Contains(got, "project p1") || strings.Contains(got, "/specs/1-*/") || !strings.Contains(got, "project p2") || !strings.Contains(got, "/specs/PX-2-*/") {
		t.Fatalf("removing p1 must leave p2's block alone:\n%s", got)
	}
	if !ignored(t, repo, "specs/px-2-thing/spec.md") || ignored(t, repo, "specs/3-thing/spec.md") {
		t.Error("p2's rules must still apply, p1's must be gone")
	}
}

// An opening marker without its closing line is dropped on its own: the lines
// after it cannot be told from the user's and are kept.
func TestSpecExclusionsKeepTheLinesAfterAnUnclosedMarker(t *testing.T) {
	ctx := context.Background()
	repo := checkoutOf(t, "git@github.com:o/a.git")
	open, _ := specExcludeMarkers("p1")
	writeExclude(t, repo, "*.log\n"+open+"\n/mine/\n")
	if err := removeSpecExclusions(ctx, repo, "p1"); err != nil {
		t.Fatal(err)
	}
	if got := readExclude(t, repo); got != "*.log\n/mine/\n" {
		t.Fatalf("got %q", got)
	}
}

// An unsafe key gets no rule and writes nothing; a tracked file under an
// excluded path stays tracked.
func TestSpecExclusionsUnsafeKeyAndTrackedFiles(t *testing.T) {
	ctx := context.Background()
	repo := checkoutOf(t, "git@github.com:o/a.git")
	if covered, err := ensureSpecExclusions(ctx, repo, "p1", "a b"); err != nil || covered {
		t.Fatalf("an unsafe key must be reported uncovered: %v %v", covered, err)
	}
	if got := readExclude(t, repo); strings.Contains(got, "sectile") {
		t.Fatalf("an unsafe key must write nothing: %q", got)
	}

	if err := os.MkdirAll(filepath.Join(repo, "specs", "5-old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "specs", "5-old", "spec.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repo, "add", "specs")
	gitTest(t, repo, "commit", "-q", "-m", "spec")
	if _, err := ensureSpecExclusions(ctx, repo, "p1", "#5"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "specs", "5-old", "spec.md"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if status := gitTest(t, repo, "status", "--porcelain"); !strings.Contains(status, "specs/5-old/spec.md") {
		t.Fatalf("a tracked specification must stay tracked: %q", status)
	}
}

// specDispatchServer answers the agent configuration and the task of a
// dispatch, with the project's server setting.
func specDispatchServer(t *testing.T, config *agentconfig.Config, task models.Task) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/agent/config" {
			_ = json.NewEncoder(w).Encode(config)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/tasks/") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(task)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// A dispatch writes the task's rules before the session starts when the
// project drops its artefacts, and removes the block once it keeps them.
func TestDispatchAppliesTheSpecArtifactsSetting(t *testing.T) {
	ctx := context.Background()
	testhome.Temp(t)
	root := checkoutOf(t, "git@github.com:o/a.git")
	config := agentconfig.Config{SchemaVersion: 1, ProjectID: "p", GitRemoteURL: "git@github.com:o/a.git", UseWorktrees: true, AIProvider: "claude", SpecArtifacts: "drop"}
	srv := specDispatchServer(t, &config, models.Task{ID: "task", Key: "#46", ProjectID: "p"})
	d := &agentDaemon{repoRoot: root, loopback: loopbackServer{url: "http://127.0.0.1:8091"}, link: serverLink{serverURL: srv.URL, token: "token", projectID: "p"}}

	effective, workDir, _, _, err := d.prepareDispatch(ctx, "#46")
	if err != nil {
		t.Fatal(err)
	}
	if !effective.DropsSpecArtifacts() || !ignored(t, workDir, "specs/46-x/spec.md") || !ignored(t, workDir, "docs/clarifications/46.md") {
		t.Fatalf("a drop dispatch must exclude the task's artefacts: %q\n%s", effective.SpecArtifacts, readExclude(t, root))
	}

	config.SpecArtifacts = "keep"
	if _, workDir, _, _, err = d.prepareDispatch(ctx, "#46"); err != nil {
		t.Fatal(err)
	}
	if ignored(t, workDir, "specs/46-x/spec.md") || strings.Contains(readExclude(t, root), "sectile") {
		t.Fatalf("a keep dispatch must remove the block:\n%s", readExclude(t, root))
	}
}

// On a project with several repositories the rules go to the checkout of the
// task's primary repository, where the stages write, and nowhere else.
func TestDispatchExcludesInThePrimaryRepositoryOnly(t *testing.T) {
	ctx := context.Background()
	testhome.Temp(t)
	projectRoot := checkoutOf(t, "git@github.com:o/a.git")
	b := checkoutOf(t, "git@github.com:o/b.git")
	if err := agentconfig.WriteSettings(agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {Path: projectRoot}}, Repositories: map[string]string{"github.com/o/b": b}}); err != nil {
		t.Fatal(err)
	}
	config := multiRepoConfig()
	config.SchemaVersion, config.AIProvider, config.UseWorktrees, config.SpecArtifacts = 1, "claude", true, "drop"
	srv := specDispatchServer(t, &config, models.Task{ID: "task", Key: "#7", ProjectID: "p", Repository: "github.com/o/b"})
	d := &agentDaemon{repoRoot: projectRoot, loopback: loopbackServer{url: "http://127.0.0.1:8091"}, link: serverLink{serverURL: srv.URL, token: "token", projectID: "p"}}

	if _, _, _, _, err := d.prepareDispatch(ctx, "#7"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readExclude(t, b), "/specs/7-*/") {
		t.Fatalf("the primary repository must carry the rules:\n%s", readExclude(t, b))
	}
	if strings.Contains(readExclude(t, projectRoot), "sectile") {
		t.Fatalf("the project root is a context repository here and gets no rule:\n%s", readExclude(t, projectRoot))
	}
}

// Only the stages that write or read the artefacts hear about a drop.
func TestSpecArtifactsNotice(t *testing.T) {
	drop := agentconfig.Config{SpecArtifacts: "drop"}
	for _, skill := range []string{"clarify", "specify", "implement", "adjust", "pickup", "pickup_issues"} {
		if got := specArtifactsNotice(drop, skill); !strings.Contains(got, "Specification artefacts are dropped on this workstation") {
			t.Errorf("%s: %q", skill, got)
		}
	}
	if got := specArtifactsNotice(drop, "handoff"); got != "" {
		t.Errorf("a stage outside the list must hear nothing: %q", got)
	}
	if got := specArtifactsNotice(agentconfig.Config{SpecArtifacts: "keep"}, "specify"); got != "" {
		t.Errorf("keep must add nothing: %q", got)
	}
}

// The server asks the actor's agent whether this workstation drops a task's
// artefacts; the answer is the effective value, keep for a key without rules.
func TestSpecArtifactsOperationAnswersTheEffectiveValue(t *testing.T) {
	ctx := context.Background()
	testhome.Temp(t)
	root := checkoutOf(t, "git@github.com:o/a.git")
	task := models.Task{ID: "task", Key: "#54", ProjectID: "p"}
	config := agentconfig.Config{SchemaVersion: 1, ProjectID: "p", GitRemoteURL: "git@github.com:o/a.git", UseWorktrees: true, AIProvider: "claude", SpecArtifacts: "drop"}
	srv := specDispatchServer(t, &config, task)
	d := &agentDaemon{repoRoot: root, link: serverLink{serverURL: srv.URL, token: "token", projectID: "p"}}
	mode := func(taskID string) string {
		t.Helper()
		value, err := d.executeOperation(ctx, agentprotocol.Operation{ProjectID: "p", TaskID: taskID, Action: "spec_artifacts"})
		if err != nil {
			t.Fatal(err)
		}
		return value.(map[string]string)["mode"]
	}
	if got := mode(task.ID); got != "drop" {
		t.Fatalf("drop: %q", got)
	}
	if got := mode(""); got != "drop" {
		t.Fatalf("drop without a task: %q", got)
	}
	if err := agentconfig.WriteSettings(agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {Path: root, SpecArtifacts: "keep"}}}); err != nil {
		t.Fatal(err)
	}
	if got := mode(task.ID); got != "keep" {
		t.Fatalf("the workstation override must win: %q", got)
	}
	if got := specArtifactsMode(agentconfig.Config{SpecArtifacts: "drop"}, "a b")["mode"]; got != "keep" {
		t.Fatalf("a key without rules keeps: %q", got)
	}
}
