package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/testhome"
)

func TestLocalSpecRepoPrefersTheWorkstationMapping(t *testing.T) {
	root := t.TempDir()
	if got, err := localSpecRepo(agentconfig.Settings{}, "p1", root, true); err != nil || got != root {
		t.Fatalf("without an override a mono-repo checkout carries the specifications: %q %v", got, err)
	}
	wiki := t.TempDir()
	overrides := agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p1": {SpecPath: wiki}}}
	for _, mono := range []bool{true, false} {
		if got, err := localSpecRepo(overrides, "p1", root, mono); err != nil || got != wiki {
			t.Fatalf("the override must win (monoRepo %v): %q %v", mono, got, err)
		}
	}
	if got, _ := localSpecRepo(overrides, "p2", root, true); got != root {
		t.Fatalf("another project's override must not apply, got %q", got)
	}
	missing := filepath.Join(t.TempDir(), "gone")
	overrides.ProjectSettings["p1"] = agentconfig.ProjectSettings{SpecPath: missing}
	if _, err := localSpecRepo(overrides, "p1", root, true); err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("an override to a missing directory must be refused by name, got %v", err)
	}
}

// A multi-repo project never falls back on the code checkout: the refusal
// names the desktop setting instead.
func TestLocalSpecRepoRequiresTheFolderOnAMultiRepoProject(t *testing.T) {
	got, err := localSpecRepo(agentconfig.Settings{}, "p1", t.TempDir(), false)
	if err == nil || got != "" || !strings.Contains(err.Error(), "Specifications folder") {
		t.Fatalf("a multi-repo project without a folder must be refused naming the setting, got %q %v", got, err)
	}
}

// A macro launch prepares the skills and the macro worktree from the project
// alone: it never asks the server for a task.
func TestMacroWorkspacePreparesWithoutATask(t *testing.T) {
	ctx := context.Background()
	testhome.Temp(t)
	root, _ := specRepoWithRemote(t)
	config := agentconfig.Config{SchemaVersion: 1, ProjectID: "remote-project", UseWorktrees: true, AIProvider: "claude",
		Skills: []agentconfig.Skill{{ID: "realign_macro", Directory: "realign-macro", Command: "/realign-macro", Content: "realign instructions"}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/execution-seed" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path != "/api/v1/agent/config" {
			t.Errorf("a macro launch must not read %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("taskKey") != "" {
			t.Errorf("a macro launch must not name a task: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(config)
	}))
	defer srv.Close()
	d := &agentDaemon{repoRoot: root, loopback: loopbackServer{url: "http://127.0.0.1:8091"}, link: serverLink{serverURL: srv.URL, token: "token", projectID: "remote-project"}}
	// The provider is the workstation's (#305).
	if err := agentconfig.WriteSettings(agentconfig.Settings{Defaults: agentconfig.Defaults{Execution: agentconfig.Execution{AIProvider: "claude"}}}); err != nil {
		t.Fatal(err)
	}

	fetched, err := d.fetchConfig(ctx, "remote-project", "")
	if err != nil {
		t.Fatal(err)
	}
	effective, cwd, workspace, err := d.prepareMacroWorkspace(ctx, fetched, "M-7", "Ux improvements")
	if err != nil {
		t.Fatalf("prepareMacroWorkspace: %v", err)
	}
	if !samePath(t, cwd, root) {
		t.Fatalf("the skill must run in the project checkout, got %s", cwd)
	}
	if !workspace.Worktree || workspace.Branch != "M-7-ux-improvements" || !samePath(t, workspace.Path, filepath.Join(root, ".tasks", "worktrees", "M-7")) {
		t.Fatalf("unexpected macro workspace %+v", workspace)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".claude/skills/realign-macro/SKILL.md")); err != nil {
		t.Fatalf("the macro skill must be installed: %v", err)
	}
	line, err := dispatchCommand(effective, "M-7", "realign_macro", "realign_macro", "", "", models.SkillModeInteractive, "",
		agentCommandContext{Branch: workspace.Branch, Directory: cwd})
	if err != nil || !strings.Contains(line, "/realign-macro M-7") {
		t.Fatalf("the command must name the macro: %q %v", line, err)
	}
}

// A multi-repo project whose specifications folder is a plain folder: the
// launch writes in that folder, with no branch, and the command line carries
// no dangling branch text. Without the folder the launch is refused.
func TestMacroWorkspaceOnAPlainFolderOfAMultiRepoProject(t *testing.T) {
	ctx := context.Background()
	testhome.Temp(t)
	root, _ := specRepoWithRemote(t)
	plain := t.TempDir()
	mono := false
	config := agentconfig.Config{SchemaVersion: 1, ProjectID: "remote-project", UseWorktrees: true, AIProvider: "claude", MonoRepo: &mono,
		Skills: []agentconfig.Skill{{ID: "realign_macro", Directory: "realign-macro", Command: "/realign-macro", Content: "realign instructions"}}}
	d := &agentDaemon{repoRoot: root, loopback: loopbackServer{url: "http://127.0.0.1:8091"}, link: serverLink{serverURL: "http://127.0.0.1:9", token: "token", projectID: "remote-project"}}

	if _, _, _, err := d.prepareMacroWorkspace(ctx, config, "M-7", "Ux"); err == nil || !strings.Contains(err.Error(), "Specifications folder") {
		t.Fatalf("a multi-repo project without a folder must be refused naming the setting, got %v", err)
	}

	if err := agentconfig.WriteSettings(agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"remote-project": {SpecPath: plain}}}); err != nil {
		t.Fatal(err)
	}
	effective, cwd, workspace, err := d.prepareMacroWorkspace(ctx, config, "M-7", "Ux")
	if err != nil {
		t.Fatalf("prepareMacroWorkspace: %v", err)
	}
	if workspace.Path != plain || workspace.Branch != "" || workspace.Worktree || workspace.Warning == "" {
		t.Fatalf("unexpected macro workspace %+v", workspace)
	}
	if entries, _ := os.ReadDir(plain); len(entries) != 0 {
		t.Fatalf("nothing must be created in the plain folder, found %v", entries)
	}
	line, err := dispatchCommand(effective, "M-7", "realign_macro", "realign_macro", "", "", models.SkillModeInteractive, "",
		agentCommandContext{Branch: workspace.Branch, Directory: cwd})
	if err != nil || !strings.Contains(line, "/realign-macro M-7") || strings.Contains(strings.ToLower(line), "branch") {
		t.Fatalf("the command must name the macro and no branch: %q %v", line, err)
	}
}

// Two macros each have their own worktree: their runs must not queue behind
// each other, while two runs of one macro still do.
func TestMacroRunsShareACheckoutOnlyForTheSameMacro(t *testing.T) {
	macroRun := func(key string) *controlledRun {
		return &controlledRun{root: "/repo", isolated: true, desktop: desktopRun{ProjectID: "p1", MacroKey: key}}
	}
	if sharesCheckout(macroRun("M-7"), macroRun("M-8")) {
		t.Fatal("two macros must not share a checkout")
	}
	if !sharesCheckout(macroRun("M-7"), macroRun("M-7")) {
		t.Fatal("two runs of one macro must share its worktree")
	}
	task := &controlledRun{root: "/repo", isolated: true, taskID: ""}
	if sharesCheckout(macroRun("M-7"), task) {
		t.Fatal("a macro run and a task run in their own worktrees must not share")
	}
}

// The server's slicing import reads the macro's specification through the
// agent, in the workstation's specifications folder: the code checkout on a
// mono-repo project, the override otherwise, and a plain folder is enough.
func TestMacroSpecFileReadsTheWorkstationFolder(t *testing.T) {
	ctx := context.Background()
	testhome.Temp(t)
	root, _ := specRepoWithRemote(t)
	mono := true
	config := agentconfig.Config{SchemaVersion: 1, ProjectID: "remote-project", AIProvider: "claude", SpecFramework: "speckit", MonoRepo: &mono}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(config)
	}))
	defer srv.Close()
	d := &agentDaemon{repoRoot: root, link: serverLink{serverURL: srv.URL, token: "token", projectID: "remote-project"}}
	write := func(folder, content string) string {
		dir := filepath.Join(folder, "specs", "M-1-slicing")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "tasks.md")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	read := func() (agentprotocol.MacroSpecFile, error) {
		value, err := d.executeOperation(ctx, agentprotocol.Operation{ProjectID: "remote-project", Action: "macro_spec_file", MacroKey: "M-1", Framework: "speckit", SpecFile: "tasks.md"})
		if err != nil {
			return agentprotocol.MacroSpecFile{}, err
		}
		return value.(agentprotocol.MacroSpecFile), nil
	}

	inCode := write(root, "## 1. From the code checkout\n")
	got, err := read()
	if err != nil || got.Content != "## 1. From the code checkout\n" || !samePath(t, got.Origin, inCode) {
		t.Fatalf("a mono-repo project reads its code checkout: %+v %v", got, err)
	}

	mono = false
	if _, err := read(); err == nil || !strings.Contains(err.Error(), "Specifications folder") {
		t.Fatalf("a multi-repo project without a folder must be refused naming the setting, got %v", err)
	}

	plain := t.TempDir()
	inPlain := write(plain, "## 1. From the plain folder\n")
	if err := agentconfig.WriteSettings(agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"remote-project": {SpecPath: plain}}}); err != nil {
		t.Fatal(err)
	}
	got, err = read()
	if err != nil || got.Content != "## 1. From the plain folder\n" || got.Origin != inPlain {
		t.Fatalf("the override is read, plain folder or not: %+v %v", got, err)
	}

	if _, err := d.executeOperation(ctx, agentprotocol.Operation{ProjectID: "remote-project", Action: "macro_spec_file"}); err == nil {
		t.Fatal("a macro key is required")
	}
}
