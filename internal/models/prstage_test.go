package models

import "testing"

// An empty or unknown creation stage leaves creation to implementation: an
// agent that meets a value added after it was built must still run (#580).
func TestPRCreationOwner(t *testing.T) {
	for in, want := range map[string]string{
		"clarified":   "clarify",
		"specified":   "specify",
		"implemented": "implement",
		"":            "implement",
		"reviewed":    "implement",
		"Clarified":   "implement",
	} {
		if got := PRCreationOwner(in); got != want {
			t.Errorf("PRCreationOwner(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidPRCreationStage(t *testing.T) {
	for in, want := range map[string]bool{
		"clarified":   true,
		"specified":   true,
		"implemented": true,
		"":            false,
		"new":         false,
		"reviewed":    false,
	} {
		if got := ValidPRCreationStage(in); got != want {
			t.Errorf("ValidPRCreationStage(%q) = %v, want %v", in, got, want)
		}
	}
}
