package agentconfig

import (
	"os"
	"path/filepath"
	"tasks/internal/testhome"
	"testing"
)

// installed resolves a managed destination inside the provider configuration home.
func installed(t *testing.T, home, provider, relative string) string {
	t.Helper()
	loc, err := ResolveLocations(provider)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(home, loc.SkillDir, relative)
}

func TestScaffoldRefreshBacksUpEdits(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	c := Config{SchemaVersion: Version, Skills: []Skill{{ID: "implement", Directory: "code-issue", Content: "remote v1", CommandContent: "command v1"}}}
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	path := installed(t, home, "agy", "code-issue/SKILL.md")
	if err := os.WriteFile(path, []byte("local edit"), 0644); err != nil {
		t.Fatal(err)
	}
	c.Skills[0].Content = "remote v2"
	preserved, err := Scaffold(root, c)
	if err != nil || len(preserved) != 1 {
		t.Fatalf("preserved %v, %v", preserved, err)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "remote v2" {
		t.Fatal("managed skill not updated")
	}
	backup, err := os.ReadFile(filepath.Join(root, preserved[0]))
	if err != nil || string(backup) != "local edit" {
		t.Fatal("local edit backup missing")
	}
	if _, err := os.Stat(filepath.Join(root, ".skills/code-issue/SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("the checkout must receive no managed skill")
	}
}

func TestScaffoldRejectsEscapeAndUnknownVersion(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	c := Config{SchemaVersion: 99}
	if _, err := Scaffold(root, c); err == nil {
		t.Fatal("unknown version accepted")
	}
	c.SchemaVersion = Version
	c.Skills = []Skill{{ID: "implement", Directory: "../escape"}}
	if _, err := Scaffold(root, c); err == nil {
		t.Fatal("path traversal accepted")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(home, ".gemini")); err != nil {
		t.Fatal(err)
	}
	c.Skills[0].Directory = "code-issue"
	if _, err := Scaffold(root, c); err == nil {
		t.Fatal("symlink escape accepted")
	}
}

func TestOverridesDoNotMutateContract(t *testing.T) {
	c := Config{AIProvider: "agy", AICommandTemplate: "agy {prompt}", Skills: []Skill{{ID: "implement", Content: "remote"}}}
	effective := ApplyOverrides(c, Overrides{AIProvider: "claude", Terminal: "ghostty", Skills: map[string]string{"implement": "local"}})
	if c.Skills[0].Content != "remote" || effective.Skills[0].Content != "local" || effective.AIProvider != "claude" || effective.AICommandTemplate != "" {
		t.Fatal("override precedence or isolation failed")
	}
}

func TestScaffoldValidatesAllSkillsBeforeWriting(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	c := Config{SchemaVersion: Version, Skills: []Skill{{ID: "valid", Directory: "valid", Content: "should not be written"}, {ID: "invalid", Directory: "../escape"}}}
	if _, err := Scaffold(root, c); err == nil {
		t.Fatal("invalid contract accepted")
	}
	if _, err := os.Stat(filepath.Join(home, ".agy")); !os.IsNotExist(err) {
		t.Fatal("partial skill install")
	}
}
func TestScaffoldRetiresOwnedFilesAndPreservesPersonalSkills(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	c := Config{SchemaVersion: Version, Skills: []Skill{{ID: "old", Directory: "old", Content: "old skill"}}}
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	personal := installed(t, home, "agy", "personal/SKILL.md")
	if err := os.MkdirAll(filepath.Dir(personal), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(personal, []byte("personal"), 0644); err != nil {
		t.Fatal(err)
	}
	changed := installed(t, home, "agy", "old/SKILL.md")
	if err := os.WriteFile(changed, []byte("customized"), 0644); err != nil {
		t.Fatal(err)
	}
	c.Skills = nil
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{personal, changed} {
		if _, err := os.Stat(p); err != nil {
			t.Fatal("personal file removed", err)
		}
	}
}
func TestScaffoldRejectsUnsafeManifest(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	path, err := ManifestPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"README.md":"somehash"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Scaffold(root, Config{SchemaVersion: Version}); err == nil {
		t.Fatal("manifest claimed unrelated file")
	}
	os.MkdirAll(filepath.Join(root, ".taskflow"), 0755)
	os.WriteFile(filepath.Join(root, ".taskflow/agent-manifest.json"), []byte(`{"README.md":"somehash"}`), 0644)
	os.Remove(path)
	if _, err := Scaffold(root, Config{SchemaVersion: Version}); err == nil {
		t.Fatal("checkout manifest claimed unrelated file")
	}
}

func TestWorktreeOverrideIsScopedToProject(t *testing.T) {
	overrides := Overrides{Worktrees: map[string]bool{"a": false}}
	if ApplyOverrides(Config{ProjectID: "a", UseWorktrees: true}, overrides).UseWorktrees {
		t.Fatal("local worktree setting ignored")
	}
	if !ApplyOverrides(Config{ProjectID: "b", UseWorktrees: true}, overrides).UseWorktrees {
		t.Fatal("override leaked to another project")
	}
}

func TestExecutionLimitDefaultsAndOverrides(t *testing.T) {
	o := Overrides{Parallelism: map[string]int{"a": 3}}
	if ExecutionLimit("b", true, o) != 1 {
		t.Fatal("a project without a local override must run a single execution")
	}
	if ExecutionLimit("a", true, o) != 3 {
		t.Fatal("local override ignored")
	}
	if ExecutionLimit("a", false, o) != 1 {
		t.Fatal("shared checkout must be serialized")
	}
	bounds := Overrides{Parallelism: map[string]int{"low": 0, "high": MaxParallelism + 4, "max": MaxParallelism}}
	if ExecutionLimit("low", true, bounds) != 1 {
		t.Fatal("limit below the range must clamp to one")
	}
	if ExecutionLimit("high", true, bounds) != MaxParallelism {
		t.Fatal("limit above the range must clamp to the ceiling")
	}
	if ExecutionLimit("max", true, bounds) != MaxParallelism {
		t.Fatal("ceiling must be selectable")
	}
}

func TestProjectCommandOverrideAndWorkstationReset(t *testing.T) {
	config := Config{ProjectID: "p", AICommandTemplate: "server {prompt}"}
	overrides := Overrides{
		AICommandTemplate:           "workstation {prompt}",
		AICommandTemplateAutonomous: "workstation -p {prompt}",
		Commands:                    map[string]string{"p": "local {prompt}"},
		CommandsAutonomous:          map[string]string{"p": "local -p {prompt}"},
	}
	applied := ApplyOverrides(config, overrides)
	if got := applied.AICommandTemplate; got != "local {prompt}" {
		t.Fatalf("expected project command override, got %q", got)
	}
	if got := applied.AICommandTemplateAutonomous; got != "local -p {prompt}" {
		t.Fatalf("expected project autonomous command override, got %q", got)
	}

	// Other project inherits workstation default
	config.ProjectID = "other"
	other := ApplyOverrides(config, overrides)
	if got := other.AICommandTemplate; got != "workstation {prompt}" {
		t.Fatalf("expected workstation default command, got %q", got)
	}
	if got := other.AICommandTemplateAutonomous; got != "workstation -p {prompt}" {
		t.Fatalf("expected workstation default autonomous command, got %q", got)
	}

	// Resetting project command to empty falls back to workstation default
	config.ProjectID = "p"
	overrides.Commands["p"] = ""
	overrides.CommandsAutonomous["p"] = ""
	reset := ApplyOverrides(config, overrides)
	if got := reset.AICommandTemplate; got != "workstation {prompt}" {
		t.Fatalf("workstation reset failed, got %q", got)
	}
	if got := reset.AICommandTemplateAutonomous; got != "workstation -p {prompt}" {
		t.Fatalf("workstation autonomous reset failed, got %q", got)
	}

	// When workstation has no command templates, falls back to server templates
	overrides.AICommandTemplate = ""
	overrides.AICommandTemplateAutonomous = ""
	serverFallback := ApplyOverrides(config, overrides)
	if got := serverFallback.AICommandTemplate; got != "server {prompt}" {
		t.Fatalf("server fallback failed, got %q", got)
	}
}

func TestProjectProviderChangeDropsInheritedCommands(t *testing.T) {
	config := Config{ProjectID: "p", AIProvider: "agy"}
	overrides := Overrides{
		AIProvider:                  "claude",
		AICommandTemplate:           "claude --expert {prompt}",
		AICommandTemplateAutonomous: "claude -p --expert {prompt}",
		AIProviders:                 map[string]string{"p": "gemini"},
	}
	// Project switches to gemini without explicit project commands: inherited claude commands dropped
	applied := ApplyOverrides(config, overrides)
	if applied.AIProvider != "gemini" {
		t.Fatalf("expected gemini, got %q", applied.AIProvider)
	}
	if applied.AICommandTemplate != "" {
		t.Fatalf("expected empty command template on provider switch, got %q", applied.AICommandTemplate)
	}
	if applied.AICommandTemplateAutonomous != "" {
		t.Fatalf("expected empty autonomous template on provider switch, got %q", applied.AICommandTemplateAutonomous)
	}

	// If explicit project command is provided for the new provider, it is kept
	overrides.Commands = map[string]string{"p": "gemini {prompt}"}
	overrides.CommandsAutonomous = map[string]string{"p": "gemini -p {prompt}"}
	appliedWithCmd := ApplyOverrides(config, overrides)
	if appliedWithCmd.AICommandTemplate != "gemini {prompt}" || appliedWithCmd.AICommandTemplateAutonomous != "gemini -p {prompt}" {
		t.Fatalf("custom project command lost on provider switch")
	}
}

func TestAdjustmentScaffoldPreservesLegacyEdits(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	c := Config{SchemaVersion: Version, Skills: []Skill{{ID: "adjust", Directory: "adjust-issue", Command: "/adjust-issue", Content: "adjustment contract", CommandContent: "adjustment contract"}}}
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	path := installed(t, home, "agy", "create-pr/SKILL.md")
	if err := os.WriteFile(path, []byte("personal legacy edits"), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		reports, err := Scaffold(root, c)
		if err != nil || len(reports) == 0 {
			t.Fatalf("missing divergence report: %v %v", reports, err)
		}
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "personal legacy edits" {
		t.Fatal("legacy edits overwritten")
	}
	c = ApplyOverrides(c, Overrides{Skills: map[string]string{"review": "old review"}})
	if !c.Skills[0].RequiresReconciliation {
		t.Fatal("legacy local override not flagged")
	}
}

func TestScaffoldInstallsSeparatePRSkills(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	c := Config{SchemaVersion: Version, Skills: []Skill{
		{ID: "adjust", Directory: "adjust-issue", Command: "/adjust-issue", Content: "Adjust the existing PR", CommandContent: "Adjust the existing PR"},
		{ID: "create_pr", Directory: "create-pr", Command: "/create-pr", Content: "Create a draft PR", CommandContent: "Create a draft PR"},
	}}
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	for _, skill := range c.Skills {
		raw, err := os.ReadFile(installed(t, home, "agy", filepath.Join(skill.Directory, "SKILL.md")))
		if err != nil || string(raw) != skill.Content {
			t.Fatalf("%s: %s %v", skill.ID, raw, err)
		}
	}
	got := ApplyOverrides(c, Overrides{Skills: map[string]string{"create_pr": "Custom creation"}})
	if got.Skills[0].RequiresReconciliation || got.Skills[0].Content != c.Skills[0].Content {
		t.Fatal("creation override changed Adjust")
	}
}

func TestProjectAIProviderAndModelOverrides(t *testing.T) {
	c1 := Config{
		ProjectID:         "p1",
		AIProvider:        "agy",
		AIModel:           "server-base-model",
		AICommandTemplate: "agy {prompt}",
	}
	c2 := Config{
		ProjectID:         "p2",
		AIProvider:        "agy",
		AIModel:           "server-base-model",
		AICommandTemplate: "agy {prompt}",
	}
	overrides := Overrides{
		AIProvider: "gemini",
		AIModel:    "gemini-pro",
		AIProviders: map[string]string{
			"p1": "claude",
		},
		AIModels: map[string]string{
			"p1": "claude-opus-5",
		},
	}

	// p1 should take the per-project overrides over global and server defaults
	effective1 := ApplyOverrides(c1, overrides)
	if effective1.AIProvider != "claude" {
		t.Fatalf("expected provider 'claude', got %q", effective1.AIProvider)
	}
	if effective1.AIModel != "claude-opus-5" {
		t.Fatalf("expected model 'claude-opus-5', got %q", effective1.AIModel)
	}
	// Provider changed from server ("agy") and no command provided for p1: commands dropped
	if effective1.AICommandTemplate != "" {
		t.Fatalf("expected empty command template, got %q", effective1.AICommandTemplate)
	}

	// p2 has no project override, so it should fall back to global overrides
	effective2 := ApplyOverrides(c2, overrides)
	if effective2.AIProvider != "gemini" {
		t.Fatalf("expected provider 'gemini', got %q", effective2.AIProvider)
	}
	if effective2.AIModel != "gemini-pro" {
		t.Fatalf("expected model 'gemini-pro', got %q", effective2.AIModel)
	}

	// When neither project nor global overrides are set, fall back to server defaults
	effectiveReset := ApplyOverrides(c1, Overrides{})
	if effectiveReset.AIProvider != "agy" {
		t.Fatalf("expected provider 'agy', got %q", effectiveReset.AIProvider)
	}
	if effectiveReset.AIModel != "server-base-model" {
		t.Fatalf("expected model 'server-base-model', got %q", effectiveReset.AIModel)
	}
	if effectiveReset.AICommandTemplate != "agy {prompt}" {
		t.Fatalf("expected 'agy {prompt}', got %q", effectiveReset.AICommandTemplate)
	}

	// Project command override is preserved when project provider changes
	overridesWithCmd := overrides
	overridesWithCmd.Commands = map[string]string{"p1": "my-claude {prompt}"}
	effectiveWithCmd := ApplyOverrides(c1, overridesWithCmd)
	if effectiveWithCmd.AIProvider != "claude" || effectiveWithCmd.AICommandTemplate != "my-claude {prompt}" {
		t.Fatalf("expected claude with custom command, got provider=%q cmd=%q", effectiveWithCmd.AIProvider, effectiveWithCmd.AICommandTemplate)
	}
}

func TestTerminalOverridesPrecedence(t *testing.T) {
	c1 := Config{ProjectID: "proj-1", ExternalTerminalCommand: "pty"}
	c2 := Config{ProjectID: "proj-2", ExternalTerminalCommand: "pty"}

	// Fallback to server setting when overrides are empty
	base := ApplyOverrides(c1, Overrides{})
	if base.ExternalTerminalCommand != "pty" {
		t.Fatalf("expected 'pty', got %q", base.ExternalTerminalCommand)
	}

	// Workstation terminal override takes effect
	withGlobal := ApplyOverrides(c1, Overrides{Terminal: "terminal"})
	if withGlobal.ExternalTerminalCommand != "terminal" {
		t.Fatalf("expected 'terminal', got %q", withGlobal.ExternalTerminalCommand)
	}

	// Project terminal override takes precedence over workstation terminal override
	withProject := ApplyOverrides(c1, Overrides{
		Terminal:  "terminal",
		Terminals: map[string]string{"proj-1": "ghostty"},
	})
	if withProject.ExternalTerminalCommand != "ghostty" {
		t.Fatalf("expected 'ghostty', got %q", withProject.ExternalTerminalCommand)
	}

	// Another project without override falls back to workstation terminal
	withOtherProject := ApplyOverrides(c2, Overrides{
		Terminal:  "terminal",
		Terminals: map[string]string{"proj-1": "ghostty"},
	})
	if withOtherProject.ExternalTerminalCommand != "terminal" {
		t.Fatalf("expected 'terminal', got %q", withOtherProject.ExternalTerminalCommand)
	}
}

// The workstation override wins over the server value, and a project without
// one follows the server (#487). A stored value nobody recognises is ignored.
func TestApplyOverridesSpecArtifacts(t *testing.T) {
	overrides := Overrides{SpecArtifacts: map[string]string{"a": "drop", "b": "keep", "c": "bogus"}}
	if got := ApplyOverrides(Config{ProjectID: "a", SpecArtifacts: "keep"}, overrides); !got.DropsSpecArtifacts() {
		t.Fatalf("the drop override must win: %q", got.SpecArtifacts)
	}
	if got := ApplyOverrides(Config{ProjectID: "b", SpecArtifacts: "drop"}, overrides); got.DropsSpecArtifacts() {
		t.Fatalf("the keep override must win: %q", got.SpecArtifacts)
	}
	if got := ApplyOverrides(Config{ProjectID: "c", SpecArtifacts: "drop"}, overrides); !got.DropsSpecArtifacts() {
		t.Fatalf("an unrecognised override must follow the server: %q", got.SpecArtifacts)
	}
	if got := ApplyOverrides(Config{ProjectID: "d", SpecArtifacts: "drop"}, overrides); !got.DropsSpecArtifacts() {
		t.Fatalf("no override must follow the server: %q", got.SpecArtifacts)
	}
	if (Config{}).DropsSpecArtifacts() {
		t.Fatal("an empty value must read as keep")
	}
}
