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

// A creation stage never rejects the contract: the values of this build and
// one a newer server may add both validate (#580).
func TestConfigurationContractAcceptsAnyPRCreationStage(t *testing.T) {
	for _, stage := range []string{"", "clarified", "specified", "implemented", "after-review"} {
		c := Config{SchemaVersion: Version, ProjectID: "project", AIProvider: "codex", PRCreationStage: stage}
		if err := c.Validate(); err != nil {
			t.Errorf("prCreationStage %q: %v", stage, err)
		}
	}
}
