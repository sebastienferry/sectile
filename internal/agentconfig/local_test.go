package agentconfig

import (
	"os"
	"path/filepath"
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
	t.Setenv("HOME", home)
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
	t.Setenv("HOME", home)
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
	if err := os.Symlink(outside, filepath.Join(home, ".agy")); err != nil {
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
	t.Setenv("HOME", home)
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
	t.Setenv("HOME", home)
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
	t.Setenv("HOME", home)
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
	if ExecutionLimit("b", true, o, 2) != 2 {
		t.Fatal("server default ignored")
	}
	if ExecutionLimit("a", true, o, 2) != 3 {
		t.Fatal("local override ignored")
	}
	if ExecutionLimit("a", false, o, 2) != 1 {
		t.Fatal("shared checkout must be serialized")
	}
}

func TestProjectCommandOverrideAndServerReset(t *testing.T) {
	config := Config{ProjectID: "p", AICommandTemplate: "server {prompt}"}
	overrides := Overrides{Commands: map[string]string{"p": "local {prompt}"}}
	if got := ApplyOverrides(config, overrides).AICommandTemplate; got != "local {prompt}" {
		t.Fatal(got)
	}
	config.ProjectID = "other"
	if got := ApplyOverrides(config, overrides).AICommandTemplate; got != "server {prompt}" {
		t.Fatal("override leaked", got)
	}
	config.ProjectID = "p"
	overrides.Commands["p"] = ""
	overrides.AICommandTemplate = "legacy {prompt}"
	if got := ApplyOverrides(config, overrides).AICommandTemplate; got != "server {prompt}" {
		t.Fatal("server reset failed", got)
	}
}

func TestAdjustmentScaffoldPreservesLegacyEdits(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
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
	t.Setenv("HOME", home)
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
