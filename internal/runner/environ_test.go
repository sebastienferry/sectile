package runner

import (
	"os"
	"slices"
	"testing"

	"tasks/internal/secrets"
)

// The list is written by hand on the agent side, so it is pinned here against
// the name the server actually reads. Renaming the variable on one side and not
// the other would hand the key back to every spawned process in silence.
func TestTheServerKeyNeverReachesASpawnedProcess(t *testing.T) {
	if !slices.Contains(serverOnlySecrets, secrets.KeyEnvVar) {
		t.Fatalf("%s is not stripped from the environment of spawned processes", secrets.KeyEnvVar)
	}
	t.Setenv(secrets.KeyEnvVar, "not-for-children")
	t.Setenv("PATH", os.Getenv("PATH"))

	env := SanitizedEnviron()
	for _, entry := range env {
		if len(entry) > len(secrets.KeyEnvVar) && entry[:len(secrets.KeyEnvVar)+1] == secrets.KeyEnvVar+"=" {
			t.Fatalf("the server key is in the child environment: %q", entry)
		}
	}
	// And nothing else was dropped on the way: a child with no PATH finds no
	// provider binary at all.
	if !slices.ContainsFunc(env, func(e string) bool { return len(e) > 5 && e[:5] == "PATH=" }) {
		t.Fatal("PATH did not survive the filtering")
	}
}
