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
			"Never call a Sectile MCP server's tools, even when one is connected",
			"`transition_stage`: the stage change",
			"`start_run`, `finish_run` and `report_waiting` have no equivalent",
			"`#clarified`",
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
		"~/.config/sectile-lite/repositories.json",
		"never guess a path",
		"write the file only on a yes",
		"Compare two addresses as host and path only",
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

// The board ships beside its skill, stamped with the plugin version so that the
// page and the skills it launches come from the same release, and with nothing
// of one Jira site in it: the skill fills the configuration in at publish time.
func TestLitePluginShipsTheBoard(t *testing.T) {
	files, err := skills.RenderLitePlugin(pluginTestVersion)
	if err != nil {
		t.Fatal(err)
	}
	skill, ok := files["skills/board/SKILL.md"]
	if !ok {
		t.Fatal("the Lite plugin lacks the board skill")
	}
	for _, required := range []string{"---\nname: board\n", "`__BOARD_CONFIG__`", "searchJiraIssuesUsingJql", "editJiraIssue", "titled `Sectile Board`"} {
		if !strings.Contains(string(skill), required) {
			t.Errorf("the board skill lacks %q", required)
		}
	}
	page := string(files["skills/board/board.html"])
	if !strings.Contains(page, "Sectile Lite "+pluginTestVersion+".") || strings.Contains(page, "__PLUGIN_VERSION__") {
		t.Error("the board page is not stamped with the plugin version")
	}
	if strings.Count(page, "__BOARD_CONFIG__") != 2 {
		t.Error("the board page lost its configuration placeholder")
	}
	if strings.Contains(strings.ToLower(page), "atlassian.net\"") {
		t.Error("the board page names a Jira site")
	}
	if !strings.Contains(page, `const PLUGIN = "`+skills.LitePluginName+`"`) {
		t.Error("the board page launches another plugin's skills")
	}
	// The page may call only the tools the skill declares for it.
	for _, tool := range []string{"searchJiraIssuesUsingJql", "editJiraIssue"} {
		if !strings.Contains(page, `"`+tool+`"`) {
			t.Errorf("the board page does not call %s, which its skill declares", tool)
		}
	}
}

// The morning brief reads the user's own sources, remembered on the
// workstation, and hands the tickets it starts to the dispatcher: it names no
// team default and records no stage.
func TestLitePluginShipsTheDailyBrief(t *testing.T) {
	files, err := skills.RenderLitePlugin(pluginTestVersion)
	if err != nil {
		t.Fatal(err)
	}
	content, ok := files["skills/daily/SKILL.md"]
	if !ok {
		t.Fatal("the Lite plugin lacks the daily brief")
	}
	text := string(content)
	for _, required := range []string{
		"---\nname: daily\n",
		"## Ticket access",
		"~/.config/sectile-lite/daily.json",
		"write it only on a yes",
		"`lastRunAt`",
		"sectile-lite:dispatch",
		"Do not assume a team, project, board or query",
		"Do not do the ticket work in this session",
		"**Triage the newcomers.**",
		"`triage.jql`",
		"Ask once for the whole batch",
		"Do not write to Jira beyond the triage the user approved",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("the daily brief lacks %q", required)
		}
	}
	for _, banned := range []string{"## Execution and ticket state", "## Session title"} {
		if strings.Contains(text, banned) {
			t.Errorf("the daily brief carries %q", banned)
		}
	}
	// Its own sections name no team: the examples of the shared ticket access
	// contract are the only Jira keys in it.
	body := text[strings.Index(text, "## Goal\n"):]
	for _, banned := range []string{"PE-", "equativ"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(banned)) {
			t.Errorf("the daily brief names a team: %q", banned)
		}
	}
	if _, ok := skills.StageSkillByID("daily"); ok {
		t.Error("the daily brief leaked into the shared catalogue")
	}
}
