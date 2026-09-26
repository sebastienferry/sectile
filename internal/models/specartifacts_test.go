package models

import "testing"

// Only an explicit drop drops: a stored value nobody recognises, or no value at
// all, keeps committing the artefacts as before the setting existed.
func TestNormalizeSpecArtifacts(t *testing.T) {
	for in, want := range map[string]string{
		"":        SpecArtifactsKeep,
		"keep":    SpecArtifactsKeep,
		"drop":    SpecArtifactsDrop,
		" Drop ":  SpecArtifactsDrop,
		"DROP":    SpecArtifactsDrop,
		"discard": SpecArtifactsKeep,
	} {
		if got := NormalizeSpecArtifacts(in); got != want {
			t.Errorf("NormalizeSpecArtifacts(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidSpecArtifacts(t *testing.T) {
	for in, want := range map[string]bool{
		"":        true,
		"keep":    true,
		"Drop":    true,
		"discard": false,
		"true":    false,
	} {
		if got := ValidSpecArtifacts(in); got != want {
			t.Errorf("ValidSpecArtifacts(%q) = %v, want %v", in, got, want)
		}
	}
}
