package agent

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
)

func codexGitWorktree(t *testing.T, root string) (string, string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	main := filepath.Join(root, "main")
	worktree := filepath.Join(root, "task")
	if err := os.MkdirAll(main, 0755); err != nil {
		t.Fatal(err)
	}
	gitTest(t, main, "init", "-q", "-b", "main")
	gitTest(t, main, "commit", "-q", "--allow-empty", "-m", "seed")
	gitTest(t, main, "worktree", "add", "-q", "-b", "feat/task", worktree)
	common, err := filepath.EvalSymlinks(filepath.Join(main, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	return worktree, common
}

func TestCodexGitScopeIncludesAbsentPackedReferenceFiles(t *testing.T) {
	worktree, common := codexGitWorktree(t, t.TempDir())
	roots := worktreeGitWritableRoots(worktree)
	for _, name := range []string{"packed-refs", "packed-refs.lock", "packed-refs.new"} {
		path := filepath.Join(common, name)
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("fixture metadata unexpectedly exists: %s: %v", path, err)
		}
		if !slices.Contains(roots, path) {
			t.Errorf("missing creation grant for %s: %v", name, roots)
		}
	}
	if len(roots) != 7 {
		t.Fatalf("unexpected Git scope: %v", roots)
	}
	if slices.Contains(roots, common) {
		t.Fatal("common Git directory must remain protected")
	}
	for _, directory := range []string{"", filepath.Join(t.TempDir(), "missing"), filepath.Dir(common)} {
		if got := worktreeGitWritableRoots(directory); len(got) != 0 {
			t.Errorf("non-worktree %q received scope %v", directory, got)
		}
	}
}

func TestCodexGitScopeRejectsFileSymlinksAndDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symbolic links requires workstation privileges")
	}
	worktree, common := codexGitWorktree(t, t.TempDir())
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("protected"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(common, "packed-refs.new")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(common, "packed-refs.lock"), 0700); err != nil {
		t.Fatal(err)
	}
	roots := worktreeGitWritableRoots(worktree)
	for _, path := range []string{outside, filepath.Join(common, "packed-refs.new"), filepath.Join(common, "packed-refs.lock")} {
		if slices.Contains(roots, path) {
			t.Errorf("unsafe file grant: %s", path)
		}
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(worktree, alias); err != nil {
		t.Fatal(err)
	}
	if got := worktreeGitWritableRoots(alias); !slices.Equal(got, roots) {
		t.Fatalf("worktree alias changed canonical scope: %v != %v", got, roots)
	}
}

func TestCodexAutonomousLaunchSharesGitScopeWithoutChangingTemplates(t *testing.T) {
	t.Setenv("SECTILE_COMPANION_MCP_URL", "")
	t.Setenv("SECTILE_DESKTOP_MCP_URL", "")
	worktree, _ := codexGitWorktree(t, t.TempDir())
	launch := agentCommandContext{Directory: worktree, AddDirs: []string{"/attached folder"}}
	line, err := modeCommandLine("codex", "", "", "run", models.SkillModeAutonomous, launch)
	if err != nil {
		t.Fatal(err)
	}
	args, err := shellCommandWords(line)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range append([]string{"/attached folder"}, worktreeGitWritableRoots(worktree)...) {
		if !slices.Contains(args, "--add-dir="+root) {
			t.Errorf("missing scoped CLI root %s: %s", root, line)
		}
	}
	if !slices.Contains(args, "--approve-for-me") || !slices.Contains(args, "--json") {
		t.Fatalf("sandbox/reviewer or trace changed: %s", line)
	}
	if len(launch.AddDirs) != 1 {
		t.Fatal("launch folders mutated")
	}
	custom, err := modeCommandLine("codex", "codex {mode:exec|} {prompt}", "", "run", models.SkillModeAutonomous, launch)
	if err != nil || custom != "codex exec 'run'" {
		t.Fatalf("custom template changed: %s: %v", custom, err)
	}
	interactive, err := modeCommandLine("codex", "", "", "run", models.SkillModeInteractive, launch)
	if err != nil || strings.Contains(interactive, "packed-refs") {
		t.Fatalf("interactive CLI scope changed: %s: %v", interactive, err)
	}
}

func TestCodexReadOnlyTurnDoesNotGrantGitMetadata(t *testing.T) {
	d, id, log := codexConversationFixture(t)
	worktree, _ := codexGitWorktree(t, t.TempDir())
	d.queue.mu.Lock()
	d.queue.runs[id].desktop.Directory = worktree
	d.queue.mu.Unlock()
	codexMessage(t, d, id, `{"message":"inspect","mode":"plan"}`)
	waitCodex(t, d, id, func(c *providerConversation) bool { return !c.busy })
	for _, line := range strings.Split(codexLog(t, log), "\n") {
		var frame struct {
			Method string `json:"method"`
			Params struct {
				SandboxPolicy map[string]any `json:"sandboxPolicy"`
			} `json:"params"`
		}
		if json.Unmarshal([]byte(line), &frame) != nil || frame.Method != "turn/start" {
			continue
		}
		policy := frame.Params.SandboxPolicy
		if policy["type"] != "readOnly" || policy["writableRoots"] != nil {
			t.Fatalf("read-only turn received write grants: %v", policy)
		}
		return
	}
	t.Fatal("no turn/start recorded")
}

// Opt in only from a host that can start Codex's native sandbox. The fixture
// root must be outside system temp: workspace-write also grants temp writes.
// No model, credentials or network request is needed for command/exec.
func TestCodexNativeSandboxLinkedWorktree(t *testing.T) {
	base := os.Getenv("SECTILE_TEST_CODEX_SANDBOX_ROOT")
	if base == "" {
		t.Skip("set SECTILE_TEST_CODEX_SANDBOX_ROOT to qualify the installed native Codex sandbox")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("live enforcement qualified on macOS only")
	}
	root, err := os.MkdirTemp(base, "codex-git-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	worktree, common := codexGitWorktree(t, root)
	cmd := codexConversationCommand(worktree, nil)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	rpc := &codexRPC{input: &conversationInput{w: stdin}, pending: map[string]chan codexFrame{}, done: make(chan struct{}), notify: func(codexFrame) {}}
	cmd.Stdout = &conversationOutput{line: rpc.read}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = cmd.Wait(); close(rpc.done) }()
	t.Cleanup(func() { rpc.input.close(); _ = cmd.Process.Kill(); <-rpc.done })
	if err := rpc.request("initialize", map[string]any{"clientInfo": map[string]string{"name": "sectile", "version": "test-921"}, "capabilities": map[string]bool{"experimentalApi": true}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := rpc.input.send(map[string]any{"method": "initialized"}); err != nil {
		t.Fatal(err)
	}
	policy := map[string]any{"type": "workspaceWrite", "networkAccess": false, "writableRoots": append([]string{worktree}, worktreeGitWritableRoots(worktree)...)}
	run := func(policy map[string]any, args ...string) (int, string) {
		t.Helper()
		var result struct {
			ExitCode int    `json:"exitCode"`
			Stdout   string `json:"stdout"`
			Stderr   string `json:"stderr"`
		}
		ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
		defer cancel()
		err := rpc.call(ctx, "command/exec", map[string]any{"command": args, "cwd": worktree, "sandboxPolicy": policy, "timeoutMs": 30000, "env": map[string]string{"GIT_AUTHOR_NAME": "Fixture", "GIT_AUTHOR_EMAIL": "fixture@example.invalid", "GIT_COMMITTER_NAME": "Fixture", "GIT_COMMITTER_EMAIL": "fixture@example.invalid", "GIT_TRACE_REFS": "1"}}, &result)
		if err != nil {
			t.Fatal(err)
		}
		return result.ExitCode, result.Stdout + result.Stderr
	}
	for _, name := range []string{"first", "after-packing"} {
		if err := os.WriteFile(filepath.Join(worktree, name), []byte(name+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"git", "add", name}, {"git", "commit", "-m", name}, {"git", "pack-refs", "--all"}} {
			code, output := run(policy, args...)
			if code != 0 || strings.Contains(output, "Operation not permitted") || strings.Contains(output, "Permission denied") {
				t.Fatalf("%v: exit=%d: %s", args, code, output)
			}
			if len(args) > 1 && args[1] == "commit" && !strings.Contains(output, "AUTO_MERGE") {
				t.Fatalf("commit cleanup was not exercised: %s", output)
			}
		}
		if got := gitTest(t, worktree, "show", "HEAD:"+name); got != name {
			t.Fatalf("committed content = %q", got)
		}
		gitTest(t, worktree, "diff", "--cached", "--exit-code")
		head := gitTest(t, worktree, "rev-parse", "HEAD")
		if got := gitTest(t, worktree, "rev-parse", "refs/heads/feat/task"); got != head {
			t.Fatal("branch does not point to committed HEAD")
		}
	}
	for _, path := range []string{filepath.Join(common, "hooks", "sentinel"), filepath.Join(common, "config"), filepath.Join(common, "packed-refs.lock.unrelated"), filepath.Join(root, "main", "unrelated"), filepath.Join(root, "outside")} {
		code, output := run(policy, "/bin/sh", "-c", "printf sentinel > "+quoteShell(path))
		if code == 0 || (!strings.Contains(output, "Operation not permitted") && !strings.Contains(output, "Permission denied")) {
			t.Fatalf("protected boundary %s: exit=%d: %s", path, code, output)
		}
	}
	code, output := run(map[string]any{"type": "readOnly", "networkAccess": false}, "/bin/sh", "-c", "printf sentinel > readonly-sentinel")
	if code == 0 {
		t.Fatalf("read-only boundary writable: %s", output)
	}
}
