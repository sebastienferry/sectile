package agentconfig

import "testing"

func TestConfigurationContractValidation(t *testing.T) {
	valid := Config{SchemaVersion: Version, ProjectID: "project", AIProvider: "codex", Skills: []Skill{{ID: "clarify", Directory: "clarify-issue", Command: "/clarify-issue"}}}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		change func(*Config)
	}{
		{"version", func(c *Config) { c.SchemaVersion = 99 }},
		{"identity", func(c *Config) { c.ProjectID = "" }},
		{"provider", func(c *Config) { c.AIProvider = "unknown" }},
		{"template", func(c *Config) { c.AICommandTemplate = "codex fixed" }},
		{"custom", func(c *Config) { c.AIProvider = "custom" }},
		{"duplicate", func(c *Config) { c.Skills = append(c.Skills, c.Skills[0]) }},
		{"path collision", func(c *Config) {
			c.Skills = append(c.Skills, Skill{ID: "other", Directory: "clarify-issue", Command: "/other"})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid
			tc.change(&c)
			if c.Validate() == nil {
				t.Fatal("invalid contract accepted")
			}
		})
	}
}

func TestNamespacedCommandOnlyOnTheCommand(t *testing.T) {
	base := Config{SchemaVersion: Version, ProjectID: "project", Skills: []Skill{{ID: "clarify", Directory: "clarify-issue", Command: "/sectile:clarify-issue"}}}
	if err := base.Validate(); err != nil {
		t.Fatalf("namespaced command refused: %v", err)
	}
	for name, skill := range map[string]Skill{
		"id":        {ID: "sectile:clarify", Directory: "clarify-issue", Command: "/clarify-issue"},
		"directory": {ID: "clarify", Directory: "sectile:clarify-issue", Command: "/clarify-issue"},
	} {
		c := base
		c.Skills = []Skill{skill}
		if c.Validate() == nil {
			t.Errorf("namespaced %s accepted", name)
		}
	}
}
