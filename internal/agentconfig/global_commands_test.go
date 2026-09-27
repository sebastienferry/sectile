package agentconfig

import "testing"

func TestWorkstationCommandsApplyAcrossProjects(t *testing.T) {
	s := Settings{Defaults: Defaults{SkillCommands: map[string]string{"implement": "build-it"}}, ProjectSettings: map[string]ProjectSettings{"a": {SkillCommands: map[string]string{"implement": "old-command"}}}}
	for _, id := range []string{"a", "b"} {
		c := Resolve(Config{ProjectID: id, Skills: []Skill{{ID: "implement", Command: "code-issue"}}}, s)
		if c.Skills[0].Command != "build-it" {
			t.Fatalf("project %s: %s", id, c.Skills[0].Command)
		}
	}
}

func TestValidateWorkstationInitializationSettings(t *testing.T) {
	if err := ValidateDefaults(Defaults{InitializationProvider: "codex", SkillCommands: map[string]string{"implement": "/build-it"}}); err != nil {
		t.Fatal(err)
	}
	for _, settings := range []Defaults{{InitializationProvider: "unknown"}, {SkillCommands: map[string]string{"implement": "two words"}}} {
		if err := ValidateDefaults(settings); err == nil {
			t.Fatal("invalid workstation setting accepted")
		}
	}
}
