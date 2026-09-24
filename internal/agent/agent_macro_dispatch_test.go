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
	"tasks/internal/models"
	"tasks/internal/testhome"
)

func TestLocalSpecRepoPrefersTheWorkstationMapping(t *testing.T) {
	root := t.TempDir()
	if got, err := localSpecRepo(agentconfig.Overrides{}, "p1", root); err != nil || got != root {
		t.Fatalf("without a mapping the project checkout carries the specifications: %q %v", got, err)
	}
	wiki := t.TempDir()
	overrides := agentconfig.Overrides{SpecRepos: map[string]string{"p1": wiki}}
	if got, err := localSpecRepo(overrides, "p1", root); err != nil || got != wiki {
		t.Fatalf("the mapped checkout must be used: %q %v", got, err)
	}
	if got, _ := localSpecRepo(overrides, "p2", root); got != root {
		t.Fatalf("another project's mapping must not apply, got %q", got)
	}
	missing := filepath.Join(t.TempDir(), "gone")
	overrides.SpecRepos["p1"] = missing
	if _, err := localSpecRepo(overrides, "p1", root); err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("a mapping to a missing directory must be refused by name, got %v", err)
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
