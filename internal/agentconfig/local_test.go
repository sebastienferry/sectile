package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"tasks/internal/models"
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
	c = Resolve(c, Settings{Skills: map[string]SkillOverride{"review": {Content: "old review"}}})
	if !c.Skills[0].RequiresReconciliation {
		t.Fatal("legacy local override not flagged")
	}
}

// A workstation set up before #608 holds the implementation skill under
// code-issue. The next setup installs implement-issue and turns code-issue into
// an alias forwarding to it, so /code-issue keeps running the stage.
func TestScaffoldKeepsCodeIssueAsAnAliasOfImplementIssue(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	before := Config{SchemaVersion: Version, Skills: []Skill{{ID: "implement", Directory: "code-issue", Command: "/code-issue", Content: "implementation contract", CommandContent: "implementation contract"}}}
	if _, err := Scaffold(root, before); err != nil {
		t.Fatal(err)
	}
	after := Config{SchemaVersion: Version, Skills: []Skill{{ID: "implement", Directory: "implement-issue", Command: "/implement-issue", Content: "implementation contract", CommandContent: "implementation contract"}}}
	if _, err := Scaffold(root, after); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(installed(t, home, "agy", "implement-issue/SKILL.md"))
	if err != nil || string(raw) != "implementation contract" {
		t.Fatalf("implement-issue: %q %v", raw, err)
	}
	raw, err = os.ReadFile(installed(t, home, "agy", "code-issue/SKILL.md"))
	if err != nil || !strings.Contains(string(raw), "name: code-issue") || !strings.Contains(string(raw), "Invoke implement-issue with the same arguments") {
		t.Fatalf("code-issue is not the alias: %q %v", raw, err)
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
	got := Resolve(c, Settings{Skills: map[string]SkillOverride{"create_pr": {Content: "Custom creation"}}})
	if got.Skills[0].RequiresReconciliation || got.Skills[0].Content != c.Skills[0].Content {
		t.Fatal("creation override changed Adjust")
	}
}

// The user-level folder is shared by every project of the workstation, so the
// direct setup installs the generic content when the server sends it, and the
// project's own content only from a server that predates it.
func TestScaffoldInstallsTheGenericSkill(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	c := Config{SchemaVersion: Version, AIProvider: "claude", Skills: []Skill{{ID: "specify", Directory: "specify-issue", Command: "/specify-issue",
		Content: "Spec Kit steps", CommandContent: "Spec Kit command", DirectContent: "generic steps", DirectCommandContent: "generic command"}}}
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(installed(t, home, "claude", "specify-issue/SKILL.md"))
	if err != nil || string(raw) != "generic command" {
		t.Fatalf("installed %q, %v", raw, err)
	}
	loc, _ := ResolveLocations("agy")
	if got := skillBody(c.Skills[0], loc); got != "generic steps" {
		t.Fatalf("a CLI that does not substitute arguments gets %q", got)
	}

	c.Skills[0].DirectContent, c.Skills[0].DirectCommandContent = "", ""
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(installed(t, home, "claude", "specify-issue/SKILL.md"))
	if string(raw) != "Spec Kit command" {
		t.Fatalf("a server without generic content installs %q", raw)
	}
}

// The providers with a direct setup are read from the manifest (#732): none
// without one, and a provider whose only entry is a compatibility alias does
// not count.
func TestManagedProvidersReadsTheManifest(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	testhome.Set(t, home)
	if providers, err := ManagedProviders(); err != nil || len(providers) != 0 {
		t.Fatalf("without a manifest: %v, %v", providers, err)
	}
	c := Config{SchemaVersion: Version, AIProvider: "claude", Skills: []Skill{{ID: "adjust", Directory: "adjust-issue", Content: "adjust"}}}
	if _, err := Scaffold(root, c); err != nil {
		t.Fatal(err)
	}
	manifestPath, err := ManifestPath()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest := map[string]string{}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if _, ok := manifest[filepath.Join(".claude/skills", "create-pr", "SKILL.md")]; !ok {
		t.Fatalf("the scaffold installed no alias to check against: %v", manifest)
	}
	manifest[filepath.Join(".agents/skills", "create-pr", "SKILL.md")] = "alias"
	if raw, err = json.Marshal(manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	providers, err := ManagedProviders()
	if err != nil || len(providers) != 1 || providers[0] != "claude" {
		t.Fatalf("providers = %v, %v; want [claude]", providers, err)
	}
}

// The overlay merges skill overrides key by key: one the top level states with
// content wins, kind and all; a blank one keeps the base's (#732).
func TestOverlayMergesSkillOverrides(t *testing.T) {
	base := Settings{Skills: map[string]SkillOverride{"clarify": {Content: "legacy"}, "implement": {Content: "legacy"}, "specify": {Content: "legacy"}}}
	top := Settings{Skills: map[string]SkillOverride{
		"clarify":   {Kind: models.SkillOverrideWork, Content: "## Steps\nAsk."},
		"implement": {Kind: models.SkillOverrideWork, Content: " "},
	}}
	got := overlay(base, top).Skills
	if got["clarify"] != top.Skills["clarify"] || got["implement"] != base.Skills["implement"] || got["specify"] != base.Skills["specify"] {
		t.Fatalf("overlay: %+v", got)
	}
}
