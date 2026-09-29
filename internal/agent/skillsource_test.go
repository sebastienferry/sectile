package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/runner"
	"tasks/internal/testhome"
)

// installDirect writes the direct setup's copy of a skill for a provider.
func installDirect(t *testing.T, home, skillDir, directory string) {
	t.Helper()
	path := filepath.Join(home, filepath.FromSlash(skillDir), directory, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nname: "+directory+"\n---\nDirect copy."), 0o644); err != nil {
		t.Fatal(err)
	}
}

// pluginInstall is one entry of Claude's installed_plugins.json.
type pluginInstall struct {
	Scope       string `json:"scope"`
	ProjectPath string `json:"projectPath,omitempty"`
	InstallPath string `json:"installPath"`
	Version     string `json:"version"`
}

// installPlugin records the Sectile plugin as Claude does, with its skill
// directories under the install path, and states its enablement when enabled
// is not nil.
func installPlugin(t *testing.T, home string, enabled *bool, entries []pluginInstall, directories ...string) {
	t.Helper()
	for _, entry := range entries {
		for _, directory := range directories {
			path := filepath.Join(entry.InstallPath, "skills", directory, "SKILL.md")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("---\nname: "+directory+"\n---\nPlugin copy."), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	writeJSON(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"),
		map[string]any{"version": 2, "plugins": map[string]any{"sectile@sectile": entries}})
	if enabled != nil {
		writeJSON(t, filepath.Join(home, ".claude", "settings.json"), map[string]any{"enabledPlugins": map[string]bool{"sectile@sectile": *enabled}})
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func skillBool(v bool) *bool { return &v }

// Every rule of the resolution, one row each: the custom skill first when the
// setting lets it, an explicit command verbatim, then the preferred installed
// source and the other one, and a failure naming what was looked for (US4-US6).
func TestChooseSkillResolution(t *testing.T) {
	implement := agentconfig.Skill{ID: "implement", Directory: "code-issue", Command: "/code-issue", Content: "custom body"}
	checkout := filepath.Join(string(filepath.Separator), "src", "app")
	user := func(home string) []pluginInstall {
		return []pluginInstall{{Scope: "user", InstallPath: filepath.Join(home, "plugin-cache", "user"), Version: "1.0.0"}}
	}
	cases := []struct {
		name     string
		provider string
		custom   bool
		explicit bool
		defaults agentconfig.Defaults
		setup    func(t *testing.T, home string)
		kind     string
		command  string
		fails    bool
	}{
		{name: "custom wins by default", provider: "claude", custom: true, kind: skillKindCustom},
		{name: "custom turned off falls to the direct copy", provider: "claude", custom: true,
			defaults: agentconfig.Defaults{CustomSkillsWin: skillBool(false)},
			setup:    func(t *testing.T, home string) { installDirect(t, home, ".claude/skills", "code-issue") },
			kind:     skillKindDirect, command: "code-issue"},
		{name: "custom turned off with nothing installed fails", provider: "claude", custom: true,
			defaults: agentconfig.Defaults{CustomSkillsWin: skillBool(false)}, fails: true},
		{name: "direct preferred by default", provider: "claude",
			setup: func(t *testing.T, home string) {
				installDirect(t, home, ".claude/skills", "code-issue")
				installPlugin(t, home, nil, user(home), "code-issue")
			},
			kind: skillKindDirect, command: "code-issue"},
		{name: "plugin preferred", provider: "claude",
			defaults: agentconfig.Defaults{InstalledSkillSource: agentconfig.SkillSourcePlugin},
			setup: func(t *testing.T, home string) {
				installDirect(t, home, ".claude/skills", "code-issue")
				installPlugin(t, home, nil, user(home), "code-issue")
			},
			kind: skillKindPlugin, command: "sectile:code-issue"},
		{name: "plugin when the direct copy is missing", provider: "claude",
			setup: func(t *testing.T, home string) { installPlugin(t, home, nil, user(home), "code-issue") },
			kind:  skillKindPlugin, command: "sectile:code-issue"},
		{name: "direct when the preferred plugin is missing", provider: "claude",
			defaults: agentconfig.Defaults{InstalledSkillSource: agentconfig.SkillSourcePlugin},
			setup:    func(t *testing.T, home string) { installDirect(t, home, ".claude/skills", "code-issue") },
			kind:     skillKindDirect, command: "code-issue"},
		{name: "disabled plugin is not a source", provider: "claude",
			setup: func(t *testing.T, home string) { installPlugin(t, home, skillBool(false), user(home), "code-issue") },
			fails: true},
		{name: "explicitly enabled plugin", provider: "claude",
			setup: func(t *testing.T, home string) { installPlugin(t, home, skillBool(true), user(home), "code-issue") },
			kind:  skillKindPlugin, command: "sectile:code-issue"},
		{name: "plugin installed for another project", provider: "claude",
			setup: func(t *testing.T, home string) {
				installPlugin(t, home, nil, []pluginInstall{{Scope: "project", ProjectPath: filepath.Join(string(filepath.Separator), "src", "other"), InstallPath: filepath.Join(home, "plugin-cache", "other")}}, "code-issue")
			},
			fails: true},
		{name: "plugin installed for this checkout", provider: "claude",
			setup: func(t *testing.T, home string) {
				installPlugin(t, home, nil, []pluginInstall{{Scope: "project", ProjectPath: checkout, InstallPath: filepath.Join(home, "plugin-cache", "app")}}, "code-issue")
			},
			kind: skillKindPlugin, command: "sectile:code-issue"},
		{name: "plugin without the skill", provider: "claude",
			setup: func(t *testing.T, home string) { installPlugin(t, home, nil, user(home), "clarify-issue") },
			fails: true},
		{name: "codex direct copy", provider: "codex",
			setup: func(t *testing.T, home string) { installDirect(t, home, ".agents/skills", "code-issue") },
			kind:  skillKindDirect, command: "code-issue"},
		{name: "the Claude plugin serves Claude only", provider: "codex",
			defaults: agentconfig.Defaults{InstalledSkillSource: agentconfig.SkillSourcePlugin},
			setup:    func(t *testing.T, home string) { installPlugin(t, home, nil, user(home), "code-issue") },
			fails:    true},
		{name: "a CLI without a skill folder is not probed", provider: "gemini",
			kind: skillKindDirect, command: "code-issue"},
		{name: "an explicit command is used verbatim", provider: "claude", explicit: true,
			kind: skillKindCommand, command: "sectile:code-issue"},
		{name: "custom still wins over an explicit command", provider: "claude", custom: true, explicit: true,
			kind: skillKindCustom},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := testhome.Temp(t)
			if tc.setup != nil {
				tc.setup(t, home)
			}
			skill := implement
			skill.Custom = tc.custom
			if tc.explicit {
				skill.Command, skill.CommandOverridden = "/sectile:code-issue", true
			}
			choice, err := chooseSkill(tc.defaults, agentconfig.Config{AIProvider: tc.provider}, skill, filepath.Join(checkout, ".tasks", "worktrees", "#1"), checkout)
			if tc.fails {
				var missing errSkillNotInstalled
				if !errors.As(err, &missing) || missing.Directory != "code-issue" || missing.Provider != tc.provider {
					t.Fatalf("expected a missing skill for %s, got %+v %v", tc.provider, choice, err)
				}
				return
			}
			if err != nil || choice.Kind != tc.kind || choice.Command != tc.command || choice.Directory != "code-issue" {
				t.Fatalf("choice = %+v %v, want %s %q", choice, err, tc.kind, tc.command)
			}
		})
	}
}

// A workstation set up before implement-issue existed only has code-issue,
// directly or in an older plugin: the stage still runs, under the former name,
// and the current name wins as soon as it is installed (#608).
func TestChooseSkillFallsBackToTheFormerName(t *testing.T) {
	implement := agentconfig.Skill{ID: "implement", Directory: "implement-issue", Command: "/implement-issue"}
	checkout := filepath.Join(string(filepath.Separator), "src", "app")
	user := func(home string) []pluginInstall {
		return []pluginInstall{{Scope: "user", InstallPath: filepath.Join(home, "plugin-cache", "user"), Version: "1.0.0"}}
	}
	cases := []struct {
		name    string
		setup   func(t *testing.T, home string)
		kind    string
		command string
	}{
		{name: "current direct copy", kind: skillKindDirect, command: "implement-issue",
			setup: func(t *testing.T, home string) {
				installDirect(t, home, ".claude/skills", "implement-issue")
				installDirect(t, home, ".claude/skills", "code-issue")
			}},
		{name: "former direct copy only", kind: skillKindDirect, command: "code-issue",
			setup: func(t *testing.T, home string) { installDirect(t, home, ".claude/skills", "code-issue") }},
		{name: "former plugin only", kind: skillKindPlugin, command: "sectile:code-issue",
			setup: func(t *testing.T, home string) { installPlugin(t, home, nil, user(home), "code-issue") }},
		{name: "current plugin before the former direct copy", kind: skillKindPlugin, command: "sectile:implement-issue",
			setup: func(t *testing.T, home string) {
				installDirect(t, home, ".claude/skills", "code-issue")
				installPlugin(t, home, nil, user(home), "implement-issue", "code-issue")
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := testhome.Temp(t)
			tc.setup(t, home)
			choice, err := chooseSkill(agentconfig.Defaults{}, agentconfig.Config{AIProvider: "claude"}, implement, checkout)
			if err != nil || choice.Kind != tc.kind || choice.Command != tc.command {
				t.Fatalf("choice = %+v %v, want %s %q", choice, err, tc.kind, tc.command)
			}
		})
	}
	t.Run("nothing installed names the current skill", func(t *testing.T) {
		testhome.Temp(t)
		_, err := chooseSkill(agentconfig.Defaults{}, agentconfig.Config{AIProvider: "claude"}, implement, checkout)
		var missing errSkillNotInstalled
		if !errors.As(err, &missing) || missing.Directory != "implement-issue" {
			t.Fatalf("expected implement-issue to be reported missing, got %v", err)
		}
	})
}

// A Claude file Sectile cannot parse is not a plugin: the dispatch goes on to
// the other source, or fails as if nothing were installed.
func TestMalformedInstalledPluginsReadsAsNotInstalled(t *testing.T) {
	home := testhome.Temp(t)
	path := filepath.Join(home, ".claude", "plugins", "installed_plugins.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	skill := agentconfig.Skill{ID: "implement", Directory: "code-issue", Command: "/code-issue"}
	plugin := agentconfig.Defaults{InstalledSkillSource: agentconfig.SkillSourcePlugin}
	if _, err := chooseSkill(plugin, agentconfig.Config{AIProvider: "claude"}, skill); err == nil {
		t.Fatal("a malformed plugin list was read as an installed plugin")
	}
	installDirect(t, home, ".claude/skills", "code-issue")
	if choice, err := chooseSkill(plugin, agentconfig.Config{AIProvider: "claude"}, skill); err != nil || choice.Kind != skillKindDirect {
		t.Fatalf("the direct copy was not the fallback: %+v %v", choice, err)
	}
}

// The failure says what was looked for, and only mentions the plugin to a
// Claude user (D8). It is in French: it is shown on the run.
func TestMissingSkillMessage(t *testing.T) {
	claude := errSkillNotInstalled{Directory: "code-issue", Provider: "claude"}.Error()
	for _, want := range []string{"« code-issue »", "claude", "plugin `sectile`", "sectile-agent init --provider claude", "Initialize"} {
		if !strings.Contains(claude, want) {
			t.Errorf("claude message lacks %q: %s", want, claude)
		}
	}
	codex := errSkillNotInstalled{Directory: "code-issue", Provider: "codex"}.Error()
	if strings.Contains(codex, "plugin") || !strings.Contains(codex, "sectile-agent init --provider codex") {
		t.Errorf("codex message: %s", codex)
	}
}

// treeHash fingerprints every file under root, paths and contents, so a test
// can prove a call wrote nothing there. Directories do not count: creating the
// parents of a skipped folder writes no file.
func treeHash(t *testing.T, root string, skip ...string) string {
	t.Helper()
	sum := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for _, prefix := range skip {
			if rel == prefix || strings.HasPrefix(rel, prefix+string(filepath.Separator)) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum.Write([]byte(filepath.ToSlash(rel) + "\x00"))
		sum.Write(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// Two projects with different custom content get a file each and a prompt
// each: the user-level collision the direct setup had is gone (FR6). Nothing
// but the run folders is written, and they go when their runs end.
func TestCustomSkillsAreHandedToTheirRuns(t *testing.T) {
	home := testhome.Temp(t)
	installDirect(t, home, ".claude/skills", "code-issue")
	d := &agentDaemon{repoRoot: t.TempDir()}
	before := treeHash(t, home, filepath.Join(".config", "sectile", "runs"))

	type launched struct {
		file, line string
		done       chan struct{}
	}
	runs := map[string]launched{}
	for _, p := range []struct{ project, run, body string }{{"p1", "run-1", "First project's skill."}, {"p2", "run-2", "Second project's skill."}} {
		config := agentconfig.Config{
			ProjectID: p.project, ProjectName: "Project " + p.project, AIProvider: "custom", AICommandTemplate: "claude {addDirs} {prompt}",
			Skills: []agentconfig.Skill{{ID: "implement", Directory: "code-issue", Command: "/code-issue", Content: p.body, Custom: true}},
		}
		done := make(chan struct{})
		choice, err := d.prepareSkill(config, "implement", "implement", "Remote execution runId: "+p.run, p.run, done, t.TempDir())
		if err != nil || choice == nil || choice.Kind != skillKindCustom {
			t.Fatalf("%s: %+v %v", p.project, choice, err)
		}
		raw, err := os.ReadFile(choice.File)
		if err != nil || string(raw) != p.body {
			t.Fatalf("%s: run file holds %q, %v", p.project, raw, err)
		}
		if info, err := os.Stat(choice.File); err != nil || info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s: the run file is readable by others: %v %v", p.project, info.Mode(), err)
		}
		line, err := dispatchCommand(config, "T-1", "implement", "implement", "Remote execution runId: "+p.run, "", models.SkillModeInteractive, "",
			agentCommandContext{Directory: t.TempDir(), Skill: choice})
		if err != nil {
			t.Fatal(err)
		}
		runs[p.project] = launched{file: choice.File, line: line, done: done}
	}
	first, second := runs["p1"], runs["p2"]
	if first.file == second.file {
		t.Fatalf("two projects share a run file: %s", first.file)
	}
	for project, run := range runs {
		for _, want := range []string{"Sectile task: T-1", "Follow the skill in", "Remote execution runId: run-", "--add-dir="} {
			if !strings.Contains(run.line, want) {
				t.Errorf("%s: prompt lacks %q: %s", project, want, run.line)
			}
		}
		if !strings.Contains(run.line, filepath.Base(filepath.Dir(filepath.Dir(run.file)))) || strings.Contains(run.line, "/code-issue T-1") {
			t.Errorf("%s: prompt does not name its own file or runs the installed command: %s", project, run.line)
		}
	}
	if after := treeHash(t, home, filepath.Join(".config", "sectile", "runs")); after != before {
		t.Fatal("handing a custom skill to a run wrote outside the run folders")
	}

	close(first.done)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Dir(filepath.Dir(first.file))); os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the run folder survived its run")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(second.file); err != nil {
		t.Fatalf("ending one run removed another run's skill: %v", err)
	}
	close(second.done)
}

// A dispatch that runs an installed skill writes nothing at all (FR4, US2).
func TestInstalledSkillDispatchWritesNothing(t *testing.T) {
	home := testhome.Temp(t)
	installDirect(t, home, ".claude/skills", "code-issue")
	installPlugin(t, home, nil, []pluginInstall{{Scope: "user", InstallPath: filepath.Join(home, "plugin-cache", "user")}}, "code-issue")
	d := &agentDaemon{repoRoot: t.TempDir()}
	before := treeHash(t, home)
	config := agentconfig.Config{ProjectID: "p1", AIProvider: "claude", Skills: []agentconfig.Skill{{ID: "implement", Directory: "code-issue", Command: "/code-issue", Content: "built-in"}}}
	choice, err := d.prepareSkill(config, "implement", "implement", "", "run-1", make(chan struct{}))
	if err != nil || choice == nil || choice.Kind != skillKindDirect {
		t.Fatalf("%+v %v", choice, err)
	}
	if after := treeHash(t, home); after != before {
		t.Fatal("a dispatch of an installed skill wrote into the home directory")
	}
	if used := d.customSkillsUsed(); len(used) != 0 {
		t.Fatalf("an installed skill was recorded as custom: %+v", used)
	}
}

// A launch without a run ID still gets a folder of its own.
func TestCustomSkillWithoutARunID(t *testing.T) {
	testhome.Temp(t)
	d := &agentDaemon{repoRoot: t.TempDir()}
	config := agentconfig.Config{ProjectID: "p1", AIProvider: "claude", Skills: []agentconfig.Skill{{ID: "implement", Directory: "code-issue", Command: "/code-issue", Content: "body", Custom: true}}}
	done := make(chan struct{})
	defer close(done)
	choice, err := d.prepareSkill(config, "implement", "implement", "", "", done)
	if err != nil || choice == nil || !strings.Contains(choice.File, "local-") {
		t.Fatalf("%+v %v", choice, err)
	}
}

// A run ID becomes a path, so one that could leave the runs folder is refused.
func TestRunSkillRefusesAPathShapedRunID(t *testing.T) {
	home := testhome.Temp(t)
	skill := agentconfig.Skill{ID: "implement", Directory: "code-issue", Content: "body"}
	for _, runID := range []string{"../escape", "a/b", `a\b`, "..", ".hidden", "run id"} {
		if _, err := writeRunSkill(runID, skill); err == nil {
			t.Errorf("run ID %q accepted", runID)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "escape")); !os.IsNotExist(err) {
		t.Fatal("a refused run ID still wrote a file")
	}
}

// The sweep removes what a crash left behind and keeps the folders of runs
// that may still be going.
func TestSweepRunSkills(t *testing.T) {
	home := testhome.Temp(t)
	root := filepath.Join(home, ".config", "sectile", "runs")
	old, recent := filepath.Join(root, "old-run"), filepath.Join(root, "recent-run")
	for _, dir := range []string{old, recent} {
		if err := os.MkdirAll(filepath.Join(dir, "code-issue"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	stale := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(old, stale, stale); err != nil {
		t.Fatal(err)
	}
	sweepRunSkills(runSkillsMaxAge)
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("a week-old run folder survived the sweep")
	}
	if _, err := os.Stat(recent); err != nil {
		t.Fatalf("a recent run folder was swept: %v", err)
	}
}

// What the CLI is told for each kind of choice: the plugin's namespaced
// command, and for a custom adjust the adjustment contract still follows.
func TestDispatchCommandRunsTheChoice(t *testing.T) {
	config := agentconfig.Config{AIProvider: "custom", AICommandTemplate: "/bin/echo {prompt}", Skills: []agentconfig.Skill{
		{ID: "implement", Directory: "code-issue", Command: "/code-issue"},
		{ID: "adjust", Directory: "adjust-issue", Command: "/adjust-issue"},
	}}
	line, err := dispatchCommand(config, "T-1", "implement", "implement", "", "", models.SkillModeInteractive, "",
		agentCommandContext{Skill: &skillChoice{Kind: skillKindPlugin, Command: "sectile:code-issue", Directory: "code-issue"}})
	if err != nil || !strings.Contains(line, "/sectile:code-issue T-1") {
		t.Fatalf("plugin choice: %s %v", line, err)
	}
	file := filepath.Join(t.TempDir(), "run-1", "adjust-issue", "SKILL.md")
	line, err = dispatchCommand(config, "T-1", "adjust", "adjust", "Remote execution runId: run-1", "", models.SkillModeInteractive, "",
		agentCommandContext{Skill: &skillChoice{Kind: skillKindCustom, File: file, Directory: "adjust-issue"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Sectile task: T-1", file, "Remote execution runId: run-1", firstLine(runner.AdjustmentContract)} {
		if !strings.Contains(line, want) {
			t.Errorf("custom adjust lacks %q: %s", want, line)
		}
	}
	if strings.Contains(line, "/adjust-issue T-1") {
		t.Errorf("custom adjust also runs the installed command: %s", line)
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

// The passive signal lists the custom skills that ran, and only them: not an
// installed skill, not a custom one the setting turned off, not one resolved
// for a launch that failed before its command line was built (D9).
func TestCustomSkillsUsedRecordsCustomDispatchesOnly(t *testing.T) {
	home := testhome.Temp(t)
	installDirect(t, home, ".claude/skills", "clarify-issue")
	root := t.TempDir()
	d := &agentDaemon{repoRoot: root}
	config := agentconfig.Config{ProjectID: "p1", ProjectName: "Sectile", AIProvider: "claude", Skills: []agentconfig.Skill{
		{ID: "implement", Directory: "code-issue", Command: "/code-issue", Content: "custom", Custom: true},
		{ID: "clarify", Directory: "clarify-issue", Command: "/clarify-issue", Content: "built-in"},
	}}
	done := make(chan struct{})
	defer close(done)
	installed, err := d.prepareSkill(config, "clarify", "clarify", "", "run-1", done)
	if err != nil {
		t.Fatal(err)
	}
	d.recordCustomSkillUse(config, installed, "")
	if used := d.customSkillsUsed(); len(used) != 0 {
		t.Fatalf("an installed skill was recorded: %+v", used)
	}
	custom, err := d.prepareSkill(config, "implement", "implement", "", "run-2", done)
	if err != nil {
		t.Fatal(err)
	}
	if used := d.customSkillsUsed(); len(used) != 0 {
		t.Fatalf("a custom skill was recorded before its command line was built: %+v", used)
	}
	d.recordCustomSkillUse(config, custom, "")
	used := d.customSkillsUsed()
	if len(used) != 1 || used[0].ProjectID != "p1" || used[0].ProjectName != "Sectile" || used[0].SkillID != "implement" || used[0].Directory != "code-issue" || used[0].LastRun.IsZero() {
		t.Fatalf("custom use = %+v", used)
	}

	if _, err := agentconfig.UpdateSettings(root, func(s *agentconfig.Settings) error {
		s.Defaults.CustomSkillsWin = skillBool(false)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	fresh := &agentDaemon{repoRoot: root}
	installDirect(t, home, ".claude/skills", "code-issue")
	choice, err := fresh.prepareSkill(config, "implement", "implement", "", "run-3", done)
	if err != nil || choice.Kind != skillKindDirect {
		t.Fatalf("custom skills turned off: %+v %v", choice, err)
	}
	fresh.recordCustomSkillUse(config, choice, "")
	if used := fresh.customSkillsUsed(); len(used) != 0 {
		t.Fatalf("a skill run from its installed copy was recorded as custom: %+v", used)
	}
}

// Claude resolves a plugin's enablement from the user settings, then the
// project's .claude/settings.json, then its settings.local.json, the later
// stated key winning. The run's folder is the project.
func TestPluginEnablementFollowsClaudeScopes(t *testing.T) {
	cases := []struct {
		name                 string
		user, project, local *bool
		enabled              bool
	}{
		{name: "nothing stated", enabled: true},
		{name: "disabled for the user", user: skillBool(false), enabled: false},
		{name: "disabled in the project", user: skillBool(true), project: skillBool(false), enabled: false},
		{name: "disabled locally", project: skillBool(true), local: skillBool(false), enabled: false},
		{name: "enabled locally over the project", project: skillBool(false), local: skillBool(true), enabled: true},
		{name: "enabled in the project over the user", user: skillBool(false), project: skillBool(true), enabled: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := testhome.Temp(t)
			work := t.TempDir()
			installPlugin(t, home, tc.user, []pluginInstall{{Scope: "user", InstallPath: filepath.Join(home, "plugin-cache", "user")}}, "implement-issue")
			state := func(path string, v *bool) {
				if v != nil {
					writeJSON(t, path, map[string]any{"enabledPlugins": map[string]bool{"sectile@sectile": *v}})
				}
			}
			state(filepath.Join(work, ".claude", "settings.json"), tc.project)
			state(filepath.Join(work, ".claude", "settings.local.json"), tc.local)
			if got := claudePluginSkill(home, "implement-issue", work); got != tc.enabled {
				t.Fatalf("plugin enabled = %v, want %v", got, tc.enabled)
			}
		})
	}
}
