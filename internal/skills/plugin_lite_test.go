package skills_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/skills"
)

func TestRenderLitePluginMatchesGolden(t *testing.T) {
	files, err := skills.RenderLitePlugin(pluginTestVersion)
	if err != nil {
		t.Fatal(err)
	}
	matchPluginGolden(t, filepath.Join("testdata", "plugin-lite"), files)
}

// The Lite plugin runs without a Sectile server: nothing in it points at one,
// and Claude asks nothing at install time.
func TestRenderLitePluginDeclaresNoServer(t *testing.T) {
	files, err := skills.RenderLitePlugin(pluginTestVersion)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files[".mcp.json"]; ok {
		t.Fatal("the Lite plugin declares an MCP server")
	}
	var manifest map[string]any
	if err := json.Unmarshal(files[".claude-plugin/plugin.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest["name"] != skills.LitePluginName {
		t.Fatalf("plugin name = %v", manifest["name"])
	}
	if _, ok := manifest["userConfig"]; ok {
		t.Fatal("the Lite plugin asks for a configuration at install time")
	}
}

// Macros have no Jira equivalent: their skills stay in the full plugin, every
// ticket skill ships in the Lite one.
func TestRenderLitePluginShipsTheTicketSkills(t *testing.T) {
	files, err := skills.RenderLitePlugin(pluginTestVersion)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range skills.StageSkills {
		_, ok := files["skills/"+s.DirName+"/SKILL.md"]
		if s.Scope == "macro" && ok {
			t.Errorf("the Lite plugin ships the macro skill %s", s.DirName)
		}
		if s.Scope != "macro" && !ok {
			t.Errorf("the Lite plugin lacks the ticket skill %s", s.DirName)
		}
	}
}

// The Lite skills differ from the generic ones by their contracts only: the
// goal, steps and guards are the same text, so the two cannot drift apart.
func TestLiteSkillsShareTheGenericBody(t *testing.T) {
	for _, s := range skills.LiteStageSkills() {
		generic := skillBody(skills.RenderGenericSkillContent(s))
		lite := skillBody(skills.RenderLiteSkillContent(s))
		if generic == "" || lite != generic {
			t.Errorf("%s: the Lite body differs from the generic one", s.DirName)
		}
	}
}

// A Lite skill keeps the ticket on Jira: it says what replaces each Sectile
// tool, records its stage as a label, and never refers to a Sectile launch.
func TestLiteSkillsKeepTheStageOnJira(t *testing.T) {
	for _, s := range skills.LiteStageSkills() {
		content := skills.RenderLiteSkillContent(s)
		for _, required := range []string{
			"Sectile Lite plugin",
			"`transition_stage`: the stage change",
			"`start_run`, `finish_run` and `report_waiting` have no equivalent",
			"`sectile:clarified`",
		} {
			if !strings.Contains(content, required) {
				t.Errorf("%s lacks %q", s.DirName, required)
			}
		}
		for _, banned := range []string{
			"## Launched by Sectile",
			"not when launched by Sectile",
			"call start_run",
			"prepare_task_spec_worktree` MCP tool",
			"`/sectile:<skill>`",
		} {
			if strings.Contains(content, banned) {
				t.Errorf("%s still carries %q", s.DirName, banned)
			}
		}
		if s.FromStage != "" && !strings.Contains(content, "Sectile · <stage>") {
			t.Errorf("%s does not post its stage report", s.DirName)
		}
	}
}

// skillBody is the part of a SKILL.md between its goal and its report, which
// the contracts do not touch.
func skillBody(content string) string {
	start := strings.Index(content, "## Goal\n")
	end := strings.LastIndex(content, "## Report\n")
	if start < 0 || end < start {
		return ""
	}
	return content[start:end]
}

// The dispatcher opens a session per ticket and does none of the work itself:
// it never records a stage, so it carries no transition contract.
func TestLitePluginShipsTheDispatcher(t *testing.T) {
	files, err := skills.RenderLitePlugin(pluginTestVersion)
	if err != nil {
		t.Fatal(err)
	}
	content, ok := files["skills/dispatch/SKILL.md"]
	if !ok {
		t.Fatal("the Lite plugin lacks the dispatcher")
	}
	text := string(content)
	for _, required := range []string{
		"---\nname: dispatch\n",
		"## Ticket access",
		"Start one session per ticket",
		"/sectile-lite:<skill> <KEY>",
		"Do not do the ticket work in this session",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("the dispatcher lacks %q", required)
		}
	}
	for _, banned := range []string{"## Execution and ticket state", "## Session title"} {
		if strings.Contains(text, banned) {
			t.Errorf("the dispatcher carries %q", banned)
		}
	}
	if _, ok := skills.StageSkillByID("dispatch"); ok {
		t.Error("the dispatcher leaked into the shared catalogue")
	}
}
