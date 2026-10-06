package agentconfig

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestCoversProjectTreatsAnEmptyWhitelistAsEveryProject(t *testing.T) {
	if !(Defaults{}).CoversProject("a") {
		t.Fatal("an empty whitelist must cover every project")
	}
	listed := Defaults{ClaudeSandboxProjects: []string{"a"}}
	if !listed.CoversProject("a") || listed.CoversProject("b") {
		t.Fatalf("a whitelist covers only the projects it names: a=%v b=%v", listed.CoversProject("a"), listed.CoversProject("b"))
	}
}

func TestResolvedClaudeSandboxLaysTheProjectOverTheWorkstation(t *testing.T) {
	global := &ClaudeSandbox{
		Enabled:        boolPtr(true),
		AllowedDomains: []string{"registry.npmjs.org"},
		Allow:          []string{"Bash(make test:*)", "Read"},
		Deny:           []string{"Bash(git push:*)"},
	}
	project := &ClaudeSandbox{
		Enabled:    boolPtr(false),
		AllowWrite: []string{"~/.cache/go-build"},
		Allow:      []string{"Read", "Bash(npm test:*)"},
	}
	cases := []struct {
		name            string
		global, project *ClaudeSandbox
		whitelist       []string
		want            *ClaudeSandbox
	}{
		{name: "nothing stated"},
		{name: "workstation only", global: global, want: global},
		{name: "project only", project: project, want: project},
		{name: "both", global: global, project: project, want: &ClaudeSandbox{
			Enabled:        boolPtr(false),
			AllowedDomains: []string{"registry.npmjs.org"},
			AllowWrite:     []string{"~/.cache/go-build"},
			Allow:          []string{"Bash(make test:*)", "Read", "Bash(npm test:*)"},
			Deny:           []string{"Bash(git push:*)"},
		}},
		{name: "project inherits the state", global: global, project: &ClaudeSandbox{Allow: []string{"Grep"}}, want: &ClaudeSandbox{
			Enabled:        boolPtr(true),
			AllowedDomains: []string{"registry.npmjs.org"},
			Allow:          []string{"Bash(make test:*)", "Read", "Grep"},
			Deny:           []string{"Bash(git push:*)"},
		}},
		{name: "project covered by name", global: global, whitelist: []string{"p"}, want: global},
		{name: "project left out", global: global, project: project, whitelist: []string{"other"}, want: project},
		{name: "project left out without values", global: global, whitelist: []string{"other"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := Settings{Defaults: Defaults{ClaudeSandbox: c.global, ClaudeSandboxProjects: c.whitelist}}
			s.SetProject("p", ProjectSettings{Path: "/p", ClaudeSandbox: c.project})
			if got := s.ResolvedClaudeSandbox("p"); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestResolvedClaudeSandboxDoesNotTouchTheStoredLists(t *testing.T) {
	global := &ClaudeSandbox{Allow: []string{"Read"}}
	s := Settings{Defaults: Defaults{ClaudeSandbox: global}}
	s.SetProject("p", ProjectSettings{Path: "/p", ClaudeSandbox: &ClaudeSandbox{Allow: []string{"Grep"}}})
	s.ResolvedClaudeSandbox("p")
	if !reflect.DeepEqual(global.Allow, []string{"Read"}) {
		t.Fatalf("the workstation list was changed: %v", global.Allow)
	}
}

// foldOf runs the start-up migration on raw, the only place the fold runs
// (#744), and reads the result back.
func foldOf(t *testing.T, raw string) (Settings, SettingsMigration) {
	t.Helper()
	writeSettingsFile(t, raw)
	_, report, err := MigrateSettingsReport(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s, err := ReadSettings(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s, report
}

func TestFoldMovesTheProjectListsToTheWorkstation(t *testing.T) {
	s, report := foldOf(t, `{"layout":3,"projectSettings":{
		"b":{"path":"/b","claudeSandbox":{"allow":["Read","Bash(npm test:*)"],"deny":["Bash(git push:*)"]}},
		"a":{"path":"/a","claudeSandbox":{"allowedDomains":["registry.npmjs.org"],"allow":["Bash(make test:*)","Read"]}}}}`)
	if !report.SandboxFolded {
		t.Fatal("the fold must report that values moved")
	}
	want := &ClaudeSandbox{
		AllowedDomains: []string{"registry.npmjs.org"},
		Allow:          []string{"Bash(make test:*)", "Read", "Bash(npm test:*)"},
		Deny:           []string{"Bash(git push:*)"},
	}
	if !reflect.DeepEqual(s.Defaults.ClaudeSandbox, want) {
		t.Fatalf("workstation values: %+v, want %+v", s.Defaults.ClaudeSandbox, want)
	}
	for _, id := range []string{"a", "b"} {
		if p := s.Project(id); p.ClaudeSandbox != nil || p.Path == "" {
			t.Fatalf("project %s after the fold: %+v", id, p)
		}
	}
	if len(s.Defaults.ClaudeSandboxProjects) != 0 {
		t.Fatalf("the whitelist must start empty: %v", s.Defaults.ClaudeSandboxProjects)
	}
}

func TestFoldMovesAStateEveryProjectAgreesOn(t *testing.T) {
	s, _ := foldOf(t, `{"layout":3,"projectSettings":{
		"a":{"path":"/a","claudeSandbox":{"enabled":true}},
		"b":{"path":"/b","claudeSandbox":{"enabled":true,"allow":["Read"]}},
		"c":{"path":"/c","claudeSandbox":{"deny":["Bash(rm:*)"]}}}}`)
	if g := s.Defaults.ClaudeSandbox; g == nil || g.Enabled == nil || !*g.Enabled {
		t.Fatalf("an agreed state must move up: %+v", g)
	}
	for _, id := range []string{"a", "b", "c"} {
		if p := s.Project(id); p.ClaudeSandbox != nil {
			t.Fatalf("project %s kept values: %+v", id, p.ClaudeSandbox)
		}
	}
}

func TestFoldKeepsDivergingStatesOnTheirProjects(t *testing.T) {
	s, _ := foldOf(t, `{"layout":3,"projectSettings":{
		"a":{"path":"/a","claudeSandbox":{"enabled":true,"allow":["Read"]}},
		"b":{"path":"/b","claudeSandbox":{"enabled":false}}}}`)
	if g := s.Defaults.ClaudeSandbox; g == nil || g.Enabled != nil || !reflect.DeepEqual(g.Allow, []string{"Read"}) {
		t.Fatalf("workstation values: %+v", g)
	}
	if p := s.Project("a").ClaudeSandbox; p == nil || p.Enabled == nil || !*p.Enabled || len(p.Allow) != 0 {
		t.Fatalf("project a: %+v", p)
	}
	if p := s.Project("b").ClaudeSandbox; p == nil || p.Enabled == nil || *p.Enabled {
		t.Fatalf("project b: %+v", p)
	}
}

func TestFoldLeavesAnInvalidEntryOnItsProject(t *testing.T) {
	s, report := foldOf(t, `{"layout":3,"projectSettings":{
		"a":{"path":"/a","claudeSandbox":{"allow":["Read","  ","Bash(x\ny)"]}}}}`)
	if g := s.Defaults.ClaudeSandbox; g == nil || !reflect.DeepEqual(g.Allow, []string{"Read"}) {
		t.Fatalf("workstation values: %+v", g)
	}
	if p := s.Project("a").ClaudeSandbox; p == nil || len(p.Allow) != 2 {
		t.Fatalf("the invalid entries must stay on the project: %+v", p)
	}
	if len(report.SandboxWarnings) != 2 || !strings.Contains(report.SandboxWarnings[0], "project a, allow") {
		t.Fatalf("warnings: %q", report.SandboxWarnings)
	}
}

func TestFoldWithNothingToMoveChangesNothing(t *testing.T) {
	s, report := foldOf(t, `{"layout":3,"defaults":{"terminal":"ghostty"},"projectSettings":{"a":{"path":"/a"}}}`)
	if report.SandboxFolded || s.Defaults.ClaudeSandbox != nil {
		t.Fatalf("nothing to fold: %+v %+v", report, s.Defaults.ClaudeSandbox)
	}
}

// A file of the current layout was written after the fold: a value a project
// holds there was set on the project since, and stays with it.
func TestFoldNeverRunsOnTheCurrentLayout(t *testing.T) {
	s, report := foldOf(t, `{"layout":4,"projectSettings":{"a":{"path":"/a","claudeSandbox":{"allow":["Read"]}}}}`)
	if report.SandboxFolded || s.Defaults.ClaudeSandbox != nil || s.Project("a").ClaudeSandbox == nil {
		t.Fatalf("a current file must not be folded: %+v %+v", report, s)
	}
}

func TestMigrateSettingsPersistsTheFoldOnce(t *testing.T) {
	previous := `{"layout":3,"projectSettings":{"a":{"path":"/a","claudeSandbox":{"allow":["Read"]}}}}`
	path := writeSettingsFile(t, previous)
	migrated, report, err := MigrateSettingsReport(t.TempDir())
	if !migrated || !report.SandboxFolded || err != nil {
		t.Fatalf("migration: %v %+v %v", migrated, report, err)
	}
	if backup, err := os.ReadFile(path + ".bak-layout3"); err != nil || string(backup) != previous {
		t.Fatalf("backup: %s %v", backup, err)
	}
	raw, _ := os.ReadFile(path)
	var written struct {
		Layout   int      `json:"layout"`
		Defaults Defaults `json:"defaults"`
	}
	if err := json.Unmarshal(raw, &written); err != nil || written.Layout != SettingsLayout ||
		written.Defaults.ClaudeSandbox == nil || !reflect.DeepEqual(written.Defaults.ClaudeSandbox.Allow, []string{"Read"}) {
		t.Fatalf("rewritten file:\n%s", raw)
	}
	// A rule added to the project after the fold stays with it.
	if _, err := UpdateSettings(t.TempDir(), func(s *Settings) error {
		p := s.Project("a")
		p.ClaudeSandbox = &ClaudeSandbox{Allow: []string{"Grep"}}
		s.SetProject("a", p)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if migrated, report, err := MigrateSettingsReport(t.TempDir()); migrated || report.SandboxFolded || err != nil {
		t.Fatalf("second start: %v %+v %v", migrated, report, err)
	}
	s, err := ReadSettings(t.TempDir())
	if err != nil || !reflect.DeepEqual(s.Project("a").ClaudeSandbox.Allow, []string{"Grep"}) ||
		!reflect.DeepEqual(s.Defaults.ClaudeSandbox.Allow, []string{"Read"}) {
		t.Fatalf("after the second start: %+v %v", s, err)
	}
}

func TestOverlayKeepsTheWorkstationSandbox(t *testing.T) {
	top := Settings{Defaults: Defaults{ClaudeSandbox: &ClaudeSandbox{Allow: []string{"Read"}}, ClaudeSandboxProjects: []string{"a"}}}
	out := overlay(Settings{}, top)
	if !reflect.DeepEqual(out.Defaults.ClaudeSandbox, top.Defaults.ClaudeSandbox) || !reflect.DeepEqual(out.Defaults.ClaudeSandboxProjects, []string{"a"}) {
		t.Fatalf("overlay dropped the workstation Sandbox values: %+v", out.Defaults)
	}
	base := Settings{Defaults: Defaults{ClaudeSandbox: &ClaudeSandbox{Deny: []string{"Bash(rm:*)"}}}}
	if out := overlay(base, Settings{}); !reflect.DeepEqual(out.Defaults.ClaudeSandbox, base.Defaults.ClaudeSandbox) {
		t.Fatalf("overlay must fall back to the base: %+v", out.Defaults)
	}
}
