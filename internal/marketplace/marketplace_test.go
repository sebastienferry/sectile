package marketplace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/models"
)

const fixture = "testdata/marketplace"

func plugin(t *testing.T, name string) PluginRef {
	t.Helper()
	m, err := ParseMarketplace(fixture)
	if err != nil {
		t.Fatal(err)
	}
	ref, ok := m.PluginByName(name)
	if !ok {
		t.Fatalf("fixture has no plugin %q", name)
	}
	return ref
}

func TestParseMarketplaceReadsTheOfficialManifest(t *testing.T) {
	m, err := ParseMarketplace(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "acme-workflow" || m.Owner.Name != "Acme Platform" {
		t.Fatalf("manifest: %+v", m)
	}
	if len(m.Plugins) != 5 {
		t.Fatalf("expected five plugins, got %d", len(m.Plugins))
	}
	if _, ok := m.PluginByName("acme-flow"); !ok {
		t.Fatal("acme-flow not found by name")
	}
}

// The owner field is a string in some published marketplaces and an object in
// others; both have to land in the same value.
func TestParseMarketplaceAcceptsAStringOwner(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, `{"name":"solo","owner":"Acme","plugins":[{"name":"p","source":"./p"}]}`)
	m, err := ParseMarketplace(root)
	if err != nil {
		t.Fatal(err)
	}
	if m.Owner.Name != "Acme" {
		t.Fatalf("owner: %+v", m.Owner)
	}
}

func TestParseMarketplaceRefusesAnUnusableManifest(t *testing.T) {
	cases := map[string]string{
		"not JSON":     `{`,
		"no name":      `{"plugins":[{"name":"p","source":"./p"}]}`,
		"no plugin":    `{"name":"empty","plugins":[]}`,
		"unnamed plug": `{"name":"m","plugins":[{"source":"./p"}]}`,
	}
	for label, body := range cases {
		t.Run(label, func(t *testing.T) {
			root := t.TempDir()
			writeManifest(t, root, body)
			if _, err := ParseMarketplace(root); err == nil {
				t.Fatal("accepted")
			}
		})
	}

	if _, err := ParseMarketplace(t.TempDir()); err == nil || !strings.Contains(err.Error(), ManifestPath) {
		t.Fatalf("a directory without a manifest must name it: %v", err)
	}
}

func TestResolvePluginReadsTheWorkflowSkills(t *testing.T) {
	pack, err := ResolvePlugin(fixture, plugin(t, "acme-flow"))
	if err != nil {
		t.Fatal(err)
	}
	bodies := pack.Bodies()
	for _, dir := range []string{"clarify-issue", "specify-issue", "code-issue", "adjust-issue"} {
		body, ok := bodies[dir]
		if !ok {
			t.Fatalf("missing %s", dir)
		}
		if !strings.HasPrefix(body, "---\n") || !strings.Contains(body, "Acme checklist") {
			t.Fatalf("%s body: %q", dir, body)
		}
	}
	if len(pack.Ignored) != 0 || len(pack.Rejected) != 0 {
		t.Fatalf("a clean plugin reports nothing: %+v %+v", pack.Ignored, pack.Rejected)
	}
	// plugin.json sits at the plugin root here, and the manifest already
	// carried the same version.
	if pack.Plugin.Version != "2.1.0" {
		t.Fatalf("version: %q", pack.Plugin.Version)
	}
}

// plugin.json under .claude-plugin/ is as legal as at the root, and its
// "skills" key moves the directory the bodies are read from.
func TestResolvePluginHonoursTheSkillsKeyAndReportsWhatItIgnores(t *testing.T) {
	pack, err := ResolvePlugin(fixture, plugin(t, "acme-extras"))
	if err != nil {
		t.Fatal(err)
	}
	if got := pack.Dirs(); len(got) != 1 || got[0] != "handoff-issue" {
		t.Fatalf("accepted: %v", got)
	}
	if pack.Plugin.Version != "0.4.1" {
		t.Fatalf("version from .claude-plugin/plugin.json: %q", pack.Plugin.Version)
	}
	ignored := map[string]string{}
	for _, item := range pack.Ignored {
		ignored[item.Dir] = item.Reason
	}
	for _, dir := range []string{"docs-writer", "release-notes"} {
		if ignored[dir] == "" {
			t.Fatalf("%s was dropped instead of reported: %+v", dir, pack.Ignored)
		}
	}
}

func TestResolvePluginRejectsABodyItCannotUseAndKeepsTheRest(t *testing.T) {
	pack, err := ResolvePlugin(fixture, plugin(t, "acme-broken"))
	if err != nil {
		t.Fatal(err)
	}
	if got := pack.Dirs(); len(got) != 1 || got[0] != "pickup-issue" {
		t.Fatalf("accepted: %v", got)
	}
	if reason := pack.Rejected["create-pr"]; !strings.Contains(reason, "frontmatter") {
		t.Fatalf("rejection reason: %q", reason)
	}
}

func TestResolvePluginRefusesAPluginWithNothingSectileOwns(t *testing.T) {
	_, err := ResolvePlugin(fixture, plugin(t, "acme-docs"))
	if err == nil {
		t.Fatal("an empty pack is not a success")
	}
	if !strings.Contains(err.Error(), "docs-writer") {
		t.Fatalf("the error must name what was found: %v", err)
	}
}

func TestResolvePluginRefusesASourceLeavingTheMarketplace(t *testing.T) {
	if _, err := ResolvePlugin(fixture, plugin(t, "acme-escape")); err == nil {
		t.Fatal("a source outside the root was accepted")
	}
	for _, source := range []string{"/etc", "../../elsewhere"} {
		if _, err := ResolvePlugin(fixture, PluginRef{Name: "x", Source: source}); err == nil {
			t.Fatalf("source %q accepted", source)
		}
	}
}

func TestResolvePluginRefusesAnOversizedBody(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, `{"name":"big","plugins":[{"name":"p","source":"./p"}]}`)
	dir := filepath.Join(root, "p", "skills", "clarify-issue")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: clarify-issue\ndescription: big\n---\n\n" + strings.Repeat("x", MaxSkillBytes)
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	pack, err := ResolvePlugin(root, PluginRef{Name: "p", Source: "./p"})
	if err == nil {
		t.Fatal("an oversized prompt was accepted")
	}
	if reason := pack.Rejected["clarify-issue"]; !strings.Contains(reason, "limit") {
		t.Fatalf("rejection reason: %q", reason)
	}
}

// The accepted directories are the catalogue's own, so renaming a skill
// directory cannot leave this package matching the old name.
func TestWorkflowDirNamesMirrorTheCatalogue(t *testing.T) {
	dirs := WorkflowDirNames()
	if len(dirs) != 10 {
		t.Fatalf("expected the ten workflow directories, got %v", dirs)
	}
	known := map[string]bool{}
	for _, dir := range dirs {
		known[dir] = true
	}
	for _, dir := range models.SkillDirNames {
		if !known[dir] {
			t.Fatalf("%s is a catalogue directory this package would ignore", dir)
		}
	}
}

func writeManifest(t *testing.T, root, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(ManifestPath)), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
