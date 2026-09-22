// Package marketplace reads the Claude plugin marketplace format: a git
// repository (or a plain directory) carrying `.claude-plugin/marketplace.json`,
// plugins under it, and skills as `skills/<name>/SKILL.md`.
//
// Sectile parses the format itself instead of driving the `claude` CLI. A
// project running codex, agy, gemini, cursor or vibe must be able to source its
// workflow prompts from a marketplace too, and the revision a project pins has
// to stay Sectile's to control rather than whatever `~/.claude/plugins/` holds.
//
// The package is pure: no network, no database, no process. Everything it does
// is reading files under a root, which is what makes the whole format testable
// from testdata.
package marketplace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"tasks/internal/models"
)

// MaxSkillBytes caps an accepted SKILL.md. A skill body is a prompt, not an
// asset: past this size the entry is a mistake, and Sectile would be splicing
// it into every rendered file.
const MaxSkillBytes = 256 * 1024

// ManifestPath is where the format keeps the marketplace manifest.
const ManifestPath = ".claude-plugin/marketplace.json"

// Owner is the `owner` field of a manifest. The format writes it either as a
// bare string or as an object, so both are read into the same value.
type Owner struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

func (o *Owner) UnmarshalJSON(raw []byte) error {
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		o.Name = asString
		return nil
	}
	var asObject struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(raw, &asObject); err != nil {
		return err
	}
	o.Name, o.Email = asObject.Name, asObject.Email
	return nil
}

// PluginRef is one entry of the manifest's `plugins` array. Source is a path
// inside the marketplace repository.
type PluginRef struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`
}

// Marketplace is the parsed manifest.
type Marketplace struct {
	Name        string      `json:"name"`
	Owner       Owner       `json:"owner"`
	Description string      `json:"description,omitempty"`
	Plugins     []PluginRef `json:"plugins"`
}

// Entry is one accepted skill of a pack: a workflow directory Sectile knows and
// the whole SKILL.md found under it.
type Entry struct {
	Dir  string
	Body string
}

// Ignored is a directory the pack ships and Sectile does not install. It is
// reported rather than dropped: a team that named a skill wrong has to see why
// nothing happened.
type Ignored struct {
	Dir    string
	Reason string
}

// Pack is one plugin resolved at one revision.
type Pack struct {
	Plugin   PluginRef
	Entries  []Entry
	Ignored  []Ignored
	Rejected map[string]string // directory -> why the SKILL.md was refused
}

// Bodies keys the accepted entries by workflow directory name.
func (p Pack) Bodies() map[string]string {
	out := make(map[string]string, len(p.Entries))
	for _, e := range p.Entries {
		out[e.Dir] = e.Body
	}
	return out
}

// Dirs lists the accepted workflow directories, in the order they were read.
func (p Pack) Dirs() []string {
	out := make([]string, 0, len(p.Entries))
	for _, e := range p.Entries {
		out = append(out, e.Dir)
	}
	return out
}

// WorkflowDirNames are the skill directories a pack may supply, derived from
// the values of models.SkillDirNames so a rename in the catalogue cannot leave
// a second list behind here.
func WorkflowDirNames() []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(models.SkillDirNames))
	for _, dir := range models.SkillDirNames {
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		out = append(out, dir)
	}
	sort.Strings(out)
	return out
}

func isWorkflowDir(dir string) bool {
	for _, known := range WorkflowDirNames() {
		if known == dir {
			return true
		}
	}
	return false
}

// ParseMarketplace reads `<root>/.claude-plugin/marketplace.json`.
func ParseMarketplace(root string) (Marketplace, error) {
	var m Marketplace
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ManifestPath)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return m, fmt.Errorf("no %s at the root of %s", ManifestPath, root)
		}
		return m, err
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, fmt.Errorf("%s is not valid JSON: %w", ManifestPath, err)
	}
	if strings.TrimSpace(m.Name) == "" {
		return m, fmt.Errorf("%s carries no name", ManifestPath)
	}
	if len(m.Plugins) == 0 {
		return m, fmt.Errorf("marketplace %q lists no plugin", m.Name)
	}
	for i, p := range m.Plugins {
		if strings.TrimSpace(p.Name) == "" {
			return Marketplace{}, fmt.Errorf("plugin %d of marketplace %q has no name", i, m.Name)
		}
	}
	return m, nil
}

// PluginByName finds one plugin of a parsed manifest.
func (m Marketplace) PluginByName(name string) (PluginRef, bool) {
	for _, p := range m.Plugins {
		if strings.EqualFold(strings.TrimSpace(p.Name), strings.TrimSpace(name)) {
			return p, true
		}
	}
	return PluginRef{}, false
}

// pluginDir resolves the plugin directory and refuses anything that leaves the
// marketplace root, symlinks included: the source comes from a repository
// someone else writes.
func pluginDir(root string, plugin PluginRef) (string, error) {
	source := strings.TrimSpace(plugin.Source)
	if source == "" {
		source = "./" + plugin.Name
	}
	source = filepath.FromSlash(source)
	if filepath.IsAbs(source) || strings.HasPrefix(source, string(filepath.Separator)) {
		return "", fmt.Errorf("plugin %q uses an absolute source %q", plugin.Name, plugin.Source)
	}
	dir := filepath.Join(root, source)
	if !within(root, dir) {
		return "", fmt.Errorf("plugin %q points outside the marketplace (%s)", plugin.Name, plugin.Source)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("plugin %q has no directory at %s", plugin.Name, plugin.Source)
		}
		return "", err
	}
	if !within(resolvedRoot, resolved) {
		return "", fmt.Errorf("plugin %q resolves outside the marketplace (%s)", plugin.Name, plugin.Source)
	}
	return dir, nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// pluginManifest reads plugin.json at the plugin root or under its
// `.claude-plugin/`. Both placements are legal, and a plugin without one is
// legal too: the manifest only refines what the marketplace entry already says.
func pluginManifest(dir string) (version, skillsDir string) {
	for _, candidate := range []string{
		filepath.Join(dir, "plugin.json"),
		filepath.Join(dir, ".claude-plugin", "plugin.json"),
	} {
		raw, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		var manifest struct {
			Version string `json:"version"`
			Skills  string `json:"skills"`
		}
		if err := json.Unmarshal(raw, &manifest); err != nil {
			continue
		}
		return strings.TrimSpace(manifest.Version), strings.TrimSpace(manifest.Skills)
	}
	return "", ""
}

// ResolvePlugin reads one plugin of a marketplace and splits its skills into
// the workflow bodies Sectile accepts, the directories it ignores and the
// entries it refuses. A plugin supplying none of the workflow skills is an
// error naming what was found, never an empty success.
func ResolvePlugin(root string, plugin PluginRef) (Pack, error) {
	pack := Pack{Plugin: plugin, Rejected: map[string]string{}}

	dir, err := pluginDir(root, plugin)
	if err != nil {
		return pack, err
	}

	version, skillsKey := pluginManifest(dir)
	if pack.Plugin.Version == "" {
		pack.Plugin.Version = version
	}
	if skillsKey == "" {
		skillsKey = "./skills/"
	}
	skillsRoot := filepath.Join(dir, filepath.FromSlash(skillsKey))
	if !within(dir, skillsRoot) {
		return pack, fmt.Errorf("plugin %q points its skills outside itself (%s)", plugin.Name, skillsKey)
	}

	items, err := os.ReadDir(skillsRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return pack, fmt.Errorf("plugin %q has no skills directory at %s", plugin.Name, skillsKey)
		}
		return pack, err
	}

	var found []string
	for _, item := range items {
		if !item.IsDir() {
			continue
		}
		name := item.Name()
		found = append(found, name)
		if !isWorkflowDir(name) {
			pack.Ignored = append(pack.Ignored, Ignored{Dir: name, Reason: "not a Sectile workflow skill"})
			continue
		}
		body, err := readSkill(filepath.Join(skillsRoot, name, "SKILL.md"))
		if err != nil {
			pack.Rejected[name] = err.Error()
			continue
		}
		pack.Entries = append(pack.Entries, Entry{Dir: name, Body: body})
	}

	if len(pack.Entries) == 0 {
		if len(found) == 0 {
			return pack, fmt.Errorf("plugin %q supplies no skill at all", plugin.Name)
		}
		return pack, fmt.Errorf("plugin %q supplies no Sectile workflow skill; found %s", plugin.Name, strings.Join(found, ", "))
	}
	return pack, nil
}

// readSkill validates one SKILL.md: a frontmatter block, a non-empty body
// after it, and a size that stays a prompt.
func readSkill(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errors.New("no SKILL.md in the skill directory")
		}
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("SKILL.md is not a regular file")
	}
	if info.Size() > MaxSkillBytes {
		return "", fmt.Errorf("SKILL.md is %d bytes, over the %d byte limit", info.Size(), MaxSkillBytes)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return "", errors.New("SKILL.md has no frontmatter block")
	}
	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		return "", errors.New("SKILL.md frontmatter block is not closed")
	}
	if strings.TrimSpace(content[4+end+5:]) == "" {
		return "", errors.New("SKILL.md has no body after its frontmatter")
	}
	return content, nil
}
