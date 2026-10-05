package agent

import (
	"encoding/json"
	"reflect"
	"testing"

	"tasks/internal/agentconfig"
)

type workstationSandboxView struct {
	ClaudeSandbox   agentconfig.ClaudeSandbox `json:"claudeSandbox"`
	Projects        []string                  `json:"projects"`
	PlatformSandbox bool                      `json:"platformSandbox"`
}

func readWorkstationSandbox(t *testing.T, d *agentDaemon) workstationSandboxView {
	t.Helper()
	rec := disconnectRequest(d, "GET", "/desktop/workstation/sandbox", "")
	if rec.Code != 200 {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body.String())
	}
	var view workstationSandboxView
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	return view
}

// The workstation Sandbox values (#730) are read and saved through their own
// endpoint, with the whitelist restricted to the projects of the workstation.
func TestDesktopWorkstationSandboxEndpoint(t *testing.T) {
	d, _ := disconnectFixture(t)
	if view := readWorkstationSandbox(t, d); view.ClaudeSandbox.Enabled != nil || len(view.ClaudeSandbox.Allow) != 0 || view.Projects == nil || len(view.Projects) != 0 {
		t.Fatalf("an empty workstation: %+v", view)
	}
	body := `{"claudeSandbox":{"enabled":true,"allow":[" Read ","Read"],"deny":["Bash(git push:*)"]},"projects":["other"," p","gone","p"]}`
	if rec := disconnectRequest(d, "PUT", "/desktop/workstation/sandbox", body); rec.Code != 200 {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body.String())
	}
	view := readWorkstationSandbox(t, d)
	if view.ClaudeSandbox.Enabled == nil || !*view.ClaudeSandbox.Enabled || !reflect.DeepEqual(view.ClaudeSandbox.Allow, []string{"Read"}) ||
		!reflect.DeepEqual(view.ClaudeSandbox.Deny, []string{"Bash(git push:*)"}) {
		t.Fatalf("values not normalized: %+v", view.ClaudeSandbox)
	}
	if !reflect.DeepEqual(view.Projects, []string{"other", "p"}) {
		t.Fatalf("whitelist = %v", view.Projects)
	}
	for _, body := range []string{`{"claudeSandbox":{"allow":["  "]}}`, `{"projects":["p"]}`, `not json`} {
		if rec := disconnectRequest(d, "PUT", "/desktop/workstation/sandbox", body); rec.Code != 400 {
			t.Errorf("%s accepted: %d", body, rec.Code)
		}
	}
	if view := readWorkstationSandbox(t, d); !reflect.DeepEqual(view.ClaudeSandbox.Allow, []string{"Read"}) {
		t.Fatalf("a refused save changed the values: %+v", view.ClaudeSandbox)
	}

	// The workstation defaults form does not carry them and must not drop them.
	if rec := disconnectRequest(d, "PUT", "/desktop/workstation", `{"editorCommand":"vim"}`); rec.Code != 204 {
		t.Fatalf("workstation PUT = %d %s", rec.Code, rec.Body.String())
	}
	if view := readWorkstationSandbox(t, d); !reflect.DeepEqual(view.ClaudeSandbox.Deny, []string{"Bash(git push:*)"}) || len(view.Projects) != 2 {
		t.Fatalf("the workstation form erased the Sandbox values: %+v", view)
	}

	// Emptied, the values leave the file.
	if rec := disconnectRequest(d, "PUT", "/desktop/workstation/sandbox", `{"claudeSandbox":{},"projects":[]}`); rec.Code != 200 {
		t.Fatalf("clear = %d %s", rec.Code, rec.Body.String())
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil || settings.Defaults.ClaudeSandbox != nil || settings.Defaults.ClaudeSandboxProjects != nil {
		t.Fatalf("cleared values stored: %+v %v", settings.Defaults, err)
	}
}

// A save keeps an entry the store gained after the category opened, such as
// a rule moved up from a project meanwhile.
func TestWorkstationSandboxSaveKeepsWhatTheStoreGained(t *testing.T) {
	d, _ := disconnectFixture(t)
	if rec := disconnectRequest(d, "PUT", "/desktop/workstation/sandbox", `{"claudeSandbox":{"allow":["Read"]}}`); rec.Code != 200 {
		t.Fatalf("PUT = %d", rec.Code)
	}
	if _, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(s *agentconfig.Settings) error {
		s.Defaults.ClaudeSandbox.AddAllow("Grep")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	body := `{"claudeSandbox":{"allow":["Read","Glob"]},"claudeSandboxBase":{"allow":["Read"]}}`
	if rec := disconnectRequest(d, "PUT", "/desktop/workstation/sandbox", body); rec.Code != 200 {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body.String())
	}
	if view := readWorkstationSandbox(t, d); !reflect.DeepEqual(view.ClaudeSandbox.Allow, []string{"Read", "Glob", "Grep"}) {
		t.Fatalf("allow = %v", view.ClaudeSandbox.Allow)
	}
}

// "Move to global" takes the rule off the project and adds it to the
// workstation allow rules once.
func TestPromoteSandboxRuleMovesItToTheWorkstation(t *testing.T) {
	d, _ := disconnectFixture(t)
	if _, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(s *agentconfig.Settings) error {
		p := s.Project("p")
		p.ClaudeSandbox = &agentconfig.ClaudeSandbox{Allow: []string{"Read", "Grep"}}
		s.SetProject("p", p)
		s.Defaults.ClaudeSandbox = &agentconfig.ClaudeSandbox{Allow: []string{"Grep"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, rule := range []string{"Read", " Grep "} {
		rec := disconnectRequest(d, "POST", "/desktop/project/sandbox/promote", `{"projectId":"p","rule":"`+rule+`"}`)
		if rec.Code != 200 {
			t.Fatalf("promote %q = %d %s", rule, rec.Code, rec.Body.String())
		}
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		t.Fatal(err)
	}
	if settings.Project("p").ClaudeSandbox != nil || settings.Project("p").Path == "" {
		t.Fatalf("project after the moves: %+v", settings.Project("p"))
	}
	if !reflect.DeepEqual(settings.Defaults.ClaudeSandbox.Allow, []string{"Grep", "Read"}) {
		t.Fatalf("workstation allow = %v", settings.Defaults.ClaudeSandbox.Allow)
	}
	for _, body := range []string{`{"projectId":"p","rule":"Read"}`, `{"projectId":"other","rule":"Read"}`} {
		if rec := disconnectRequest(d, "POST", "/desktop/project/sandbox/promote", body); rec.Code != 404 {
			t.Errorf("%s = %d", body, rec.Code)
		}
	}
	if rec := disconnectRequest(d, "POST", "/desktop/project/sandbox/promote", `{"projectId":"p"}`); rec.Code != 400 {
		t.Errorf("a promotion without a rule = %d", rec.Code)
	}
	if rec := disconnectRequest(d, "GET", "/desktop/project/sandbox/promote", ""); rec.Code != 405 {
		t.Errorf("GET = %d", rec.Code)
	}
}

// Removing a project from Desktop takes it out of the whitelist.
func TestRemovedProjectLeavesTheSandboxWhitelist(t *testing.T) {
	d, _ := disconnectFixture(t)
	if rec := disconnectRequest(d, "PUT", "/desktop/workstation/sandbox", `{"claudeSandbox":{"allow":["Read"]},"projects":["p","other"]}`); rec.Code != 200 {
		t.Fatalf("PUT = %d", rec.Code)
	}
	if rec := disconnectRequest(d, "DELETE", "/desktop/projects?id=p", ""); rec.Code != 204 {
		t.Fatalf("DELETE = %d %s", rec.Code, rec.Body.String())
	}
	if view := readWorkstationSandbox(t, d); !reflect.DeepEqual(view.Projects, []string{"other"}) {
		t.Fatalf("whitelist = %v", view.Projects)
	}
}

func TestProjectSandboxInheritanceFollowsTheWhitelist(t *testing.T) {
	settings := agentconfig.Settings{Defaults: agentconfig.Defaults{ClaudeSandbox: &agentconfig.ClaudeSandbox{Deny: []string{"Bash(rm:*)"}}}}
	if covered, inherited := projectSandboxInheritance(settings, "p"); !covered || !reflect.DeepEqual(inherited["deny"], []string{"Bash(rm:*)"}) {
		t.Fatalf("covered = %v %v", covered, inherited)
	}
	settings.Defaults.ClaudeSandboxProjects = []string{"other"}
	if covered, inherited := projectSandboxInheritance(settings, "p"); covered || inherited != nil {
		t.Fatalf("a project left out inherits: %v %v", covered, inherited)
	}
}
