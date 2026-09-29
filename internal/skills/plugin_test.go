package skills_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"tasks/internal/skills"
)

var updatePlugin = flag.Bool("update", false, "rewrite the plugin golden files under testdata/plugin")

const pluginTestVersion = "0.0.0-test"

func TestRenderPluginMatchesGolden(t *testing.T) {
	files, err := skills.RenderPlugin(pluginTestVersion)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("testdata", "plugin")
	if *updatePlugin {
		if err := os.RemoveAll(root); err != nil {
			t.Fatal(err)
		}
		for name, content := range files {
			path := filepath.Join(root, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	onDisk := map[string]bool{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		onDisk[filepath.ToSlash(rel)] = true
		return nil
	})
	if err != nil {
		t.Fatalf("missing golden plugin (run with -update to generate): %v", err)
	}
	for name, content := range files {
		want, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("missing golden file %s (run with -update to generate): %v", name, err)
		}
		if !bytes.Equal(content, want) {
			t.Errorf("%s differs from its golden file (run with -update after checking the change)", name)
		}
		delete(onDisk, name)
	}
	for name := range onDisk {
		t.Errorf("golden file %s is no longer generated", name)
	}
}

func TestRenderPluginIsDeterministic(t *testing.T) {
	first, err := skills.RenderPlugin(pluginTestVersion)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := skills.RenderPlugin(pluginTestVersion)
	if len(first) != len(second) {
		t.Fatalf("%d files, then %d", len(first), len(second))
	}
	for name, content := range first {
		if !bytes.Equal(content, second[name]) {
			t.Fatalf("%s differs between two runs", name)
		}
	}
}

func TestRenderPluginShipsOneSkillPerCatalogueEntry(t *testing.T) {
	files, err := skills.RenderPlugin(pluginTestVersion)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for name := range files {
		if strings.HasPrefix(name, "skills/") {
			found = append(found, name)
		}
	}
	if len(found) != len(skills.StageSkills) {
		t.Fatalf("%d skill files for %d catalogue entries: %v", len(found), len(skills.StageSkills), found)
	}
	for _, s := range skills.StageSkills {
		content, ok := files["skills/"+s.DirName+"/SKILL.md"]
		if !ok {
			t.Fatalf("no SKILL.md for %s", s.DirName)
		}
		if !strings.HasPrefix(string(content), "---\nname: "+s.DirName+"\n") {
			t.Fatalf("%s: frontmatter name is not the directory:\n%s", s.DirName, firstLines(string(content), 3))
		}
	}
}

func TestRenderPluginManifestAndMCP(t *testing.T) {
	files, err := skills.RenderPlugin(pluginTestVersion)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Name       string `json:"name"`
		Version    string `json:"version"`
		Skills     string `json:"skills"`
		UserConfig map[string]struct {
			Type      string `json:"type"`
			Required  bool   `json:"required"`
			Sensitive bool   `json:"sensitive"`
		} `json:"userConfig"`
	}
	if err := json.Unmarshal(files[".claude-plugin/plugin.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Name != "sectile" || manifest.Version != pluginTestVersion || manifest.Skills != "./skills/" {
		t.Fatalf("manifest = %+v", manifest)
	}
	if key := manifest.UserConfig["api_key"]; !key.Required || !key.Sensitive || key.Type != "string" {
		t.Fatalf("api_key must be a required sensitive string: %+v", key)
	}
	if url := manifest.UserConfig["server_url"]; !url.Required || url.Sensitive {
		t.Fatalf("server_url must be required and not sensitive: %+v", url)
	}

	var mcp struct {
		MCPServers map[string]struct {
			Type    string            `json:"type"`
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(files[".mcp.json"], &mcp); err != nil {
		t.Fatal(err)
	}
	server, ok := mcp.MCPServers["sectile"]
	if !ok || len(mcp.MCPServers) != 1 {
		t.Fatalf("mcpServers = %+v", mcp.MCPServers)
	}
	if server.Type != "http" || server.URL != "${user_config.server_url}/mcp" || server.Headers["Authorization"] != "Bearer ${user_config.api_key}" {
		t.Fatalf("sectile MCP server = %+v", server)
	}
}

func TestRenderPluginCarriesNoAddressOrKey(t *testing.T) {
	files, err := skills.RenderPlugin(pluginTestVersion)
	if err != nil {
		t.Fatal(err)
	}
	url := regexp.MustCompile(`https?://[^\s"')` + "`" + `]*`)
	for name, content := range files {
		for _, found := range url.FindAllString(string(content), -1) {
			if found != "https://sectile.example.com" {
				t.Errorf("%s carries the address %s", name, found)
			}
		}
		for _, forbidden := range []string{"localhost", "127.0.0.1", "Bearer sk", "SECTILE_AGENT_TOKEN"} {
			if strings.Contains(string(content), forbidden) {
				t.Errorf("%s carries %q", name, forbidden)
			}
		}
	}
}

func TestRenderPluginRefusesABadVersion(t *testing.T) {
	for _, version := range []string{"", "v1.2.3", "1.2", "latest", " 1.2.3"} {
		if _, err := skills.RenderPlugin(version); err == nil {
			t.Errorf("version %q accepted", version)
		}
		if _, err := skills.RenderMarketplace(version); err == nil {
			t.Errorf("marketplace version %q accepted", version)
		}
	}
	for _, version := range []string{"0.0.0", "1.4.0", "0.0.0-test", "2.1.0-rc.1+build.5"} {
		if _, err := skills.RenderPlugin(version); err != nil {
			t.Errorf("version %q refused: %v", version, err)
		}
	}
}

func TestRenderMarketplaceWrapsThePlugin(t *testing.T) {
	plugin, _ := skills.RenderPlugin(pluginTestVersion)
	files, err := skills.RenderMarketplace(pluginTestVersion)
	if err != nil {
		t.Fatal(err)
	}
	var marketplace struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name    string `json:"name"`
			Source  string `json:"source"`
			Version string `json:"version"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(files[".claude-plugin/marketplace.json"], &marketplace); err != nil {
		t.Fatal(err)
	}
	if marketplace.Name != "sectile" || len(marketplace.Plugins) != 1 {
		t.Fatalf("marketplace = %+v", marketplace)
	}
	entry := marketplace.Plugins[0]
	if entry.Name != "sectile" || entry.Version != pluginTestVersion || entry.Source != "./"+skills.MarketplacePluginDir {
		t.Fatalf("plugin entry = %+v", entry)
	}
	for name, content := range plugin {
		if !bytes.Equal(files[skills.MarketplacePluginDir+"/"+name], content) {
			t.Fatalf("%s is not the plugin's", name)
		}
	}
	if len(files) != len(plugin)+1 {
		t.Fatalf("%d files for a %d-file plugin", len(files), len(plugin))
	}
}

// TestGenericSkillsCarryEveryFrameworkVariant checks the generic skills lose
// nothing the per-project ones carry, and keep each framework's text under its
// own heading.
func TestGenericSkillsCarryEveryFrameworkVariant(t *testing.T) {
	frameworks := skills.SpecFrameworks()
	if strings.Join(frameworks, ",") != "openspec,speckit" {
		t.Fatalf("frameworks read from the fragments = %v", frameworks)
	}
	heading := regexp.MustCompile(`^#{3,4} (When get_project_context reports specFramework "([a-z]+)"|Otherwise)$`)

	for _, s := range skills.StageSkills {
		generic := skills.RenderGenericSkillContent(s)
		sources := []string{s.ID}
		if s.ID == "pickup" || s.ID == "pickup_issues" {
			sources = append(sources, "clarify", "specify", "implement", "adjust")
		}
		// Section of the generic output each line belongs to: the framework
		// named by the last variant heading, "*" under "Otherwise", "" outside.
		sections := map[string]string{}
		current := ""
		for _, line := range strings.Split(generic, "\n") {
			if m := heading.FindStringSubmatch(line); m != nil {
				current = m[2]
				if current == "" {
					current = "*"
				}
				continue
			}
			if strings.HasPrefix(line, "## ") || (strings.HasPrefix(line, "### ") && !heading.MatchString(line)) {
				current = ""
			}
			sections[current] += line + "\n"
		}

		for _, source := range sources {
			paths, _ := filepath.Glob(filepath.Join("fragments", source, "*.md"))
			for _, path := range paths {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				text := strings.TrimSpace(string(data))
				if text == "" {
					continue
				}
				parts := strings.Split(filepath.Base(path), ".")
				if s.ID != source && parts[0] == "goal" {
					// A composed pickup carries the other stages' steps, not their goals.
					continue
				}
				if !strings.Contains(generic, text) {
					t.Errorf("%s: generic skill lacks %s", s.ID, path)
					continue
				}
				if len(parts) != 3 {
					continue
				}
				if fallback, err := os.ReadFile(filepath.Join("fragments", source, parts[0]+".md")); err == nil && strings.TrimSpace(string(fallback)) == text {
					// Same text as the unsuffixed fragment: carried once, under "Otherwise".
					continue
				}
				framework := parts[1]
				if !strings.Contains(sections[framework], text) {
					t.Errorf("%s: %s is not under its %q heading", s.ID, path, framework)
				}
				for other, body := range sections {
					if other != framework && strings.Contains(body, text) {
						t.Errorf("%s: %s also appears outside its subsection (%q)", s.ID, path, other)
					}
				}
			}
		}

		// Every fragment the per-project rendering picks is in the generic one.
		for _, framework := range frameworks {
			perProject := skills.RenderSkillContent(s, framework)
			for i, paragraph := range strings.Split(perProject, "\n\n") {
				paragraph = strings.TrimSpace(paragraph)
				if strings.HasPrefix(paragraph, "#") {
					// The heading is the generic skill's own, the text under it is what counts.
					_, paragraph, _ = strings.Cut(paragraph, "\n")
				}
				// The frontmatter and title name the framework; the task-access
				// fallback address is the one line the generic skill words differently.
				if i == 0 || paragraph == "" || strings.Contains(paragraph, "localhost") {
					continue
				}
				if !strings.Contains(generic, paragraph) {
					t.Errorf("%s/%s: generic skill lacks %q", s.ID, framework, firstLines(paragraph, 2))
				}
			}
		}
	}
}

func TestGenericSkillsCarryAGenericPullRequestPolicy(t *testing.T) {
	for _, s := range skills.StageSkills {
		generic := skills.RenderGenericSkillContent(s)
		has := strings.Contains(generic, "## Project pull request policy")
		if has != skills.HasPullRequestPolicy(s.ID) {
			t.Fatalf("%s: pull-request policy present = %v", s.ID, has)
		}
		if !has {
			continue
		}
		if strings.Contains(generic, "PR creation stage: ") {
			t.Fatalf("%s: the generic policy names a project's creation stage", s.ID)
		}
		for _, timing := range []string{"clarified", "specified", "implemented"} {
			policy := skills.ProjectPullRequestPolicy(timing)
			wording := policy[strings.Index(policy, "before executing. ")+len("before executing. "):]
			if !strings.Contains(generic, wording) {
				t.Fatalf("%s: generic policy lacks the %s wording", s.ID, timing)
			}
		}
		if !strings.Contains(generic, "Read prCreationStage from get_project_context") {
			t.Fatalf("%s: generic policy does not say where to read the stage", s.ID)
		}
	}
}

func TestPluginGoldenListsEverySkill(t *testing.T) {
	// Guards the golden directory from silently shrinking with the catalogue.
	var dirs []string
	entries, _ := os.ReadDir(filepath.Join("testdata", "plugin", "skills"))
	for _, entry := range entries {
		dirs = append(dirs, entry.Name())
	}
	var want []string
	for _, s := range skills.StageSkills {
		want = append(want, s.DirName)
	}
	sort.Strings(want)
	if strings.Join(dirs, ",") != strings.Join(want, ",") {
		t.Fatalf("golden skills = %v, catalogue = %v", dirs, want)
	}
}

func firstLines(text string, n int) string {
	lines := strings.SplitN(text, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
