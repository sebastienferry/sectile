package db

import (
	"testing"

	"tasks/internal/models"
)

func TestResolveSkillMode(t *testing.T) {
	cases := []struct {
		name     string
		override string
		skill    string
		project  string
		want     string
	}{
		{
			name:     "the one-off override wins over both settings",
			override: models.SkillModeAutonomous,
			skill:    models.SkillModeInteractive,
			project:  models.SkillModeInteractive,
			want:     models.SkillModeAutonomous,
		},
		{
			name:    "the skill setting wins over the project default",
			skill:   models.SkillModeInteractive,
			project: models.SkillModeAutonomous,
			want:    models.SkillModeInteractive,
		},
		{
			name:    "the project default applies to a skill with no opinion",
			project: models.SkillModeAutonomous,
			want:    models.SkillModeAutonomous,
		},
		{
			name: "nothing configured is interactive",
			want: models.SkillModeInteractive,
		},
		{
			name:     "an unknown override falls through to the skill",
			override: "headless",
			skill:    models.SkillModeAutonomous,
			want:     models.SkillModeAutonomous,
		},
		{
			name:    "an unknown skill setting falls through to the project",
			skill:   "non_interactive",
			project: models.SkillModeAutonomous,
			want:    models.SkillModeAutonomous,
		},
		{
			name:    "an unknown project default reads as interactive",
			project: "yes",
			want:    models.SkillModeInteractive,
		},
		{
			name:     "case and padding do not change the reading",
			override: "  Autonomous ",
			want:     models.SkillModeAutonomous,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveSkillMode(tc.override, tc.skill, tc.project); got != tc.want {
				t.Fatalf("ResolveSkillMode(%q,%q,%q) = %q, want %q", tc.override, tc.skill, tc.project, got, tc.want)
			}
		})
	}
}

func TestNormalizeFullChainStopStage(t *testing.T) {
	cases := map[string]string{
		"implemented":   models.FullChainStopImplemented,
		" Implemented ": models.FullChainStopImplemented,
		"reviewed":      models.FullChainStopReviewed,
		"":              models.FullChainStopReviewed,
		"finished":      models.FullChainStopReviewed,
	}
	for stored, want := range cases {
		if got := models.NormalizeFullChainStopStage(stored); got != want {
			t.Fatalf("NormalizeFullChainStopStage(%q) = %q, want %q", stored, got, want)
		}
	}
}

func TestValidSkillMode(t *testing.T) {
	for _, mode := range []string{"", "interactive", "autonomous", " AUTONOMOUS "} {
		if !models.ValidSkillMode(mode) {
			t.Fatalf("ValidSkillMode(%q) = false, want true", mode)
		}
	}
	for _, mode := range []string{"headless", "non_interactive", "auto"} {
		if models.ValidSkillMode(mode) {
			t.Fatalf("ValidSkillMode(%q) = true, want false", mode)
		}
	}
}
