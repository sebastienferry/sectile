package models

import (
	"os"
	"strings"
	"testing"
)

// The board reads a silence off the summary, with the prefix written a second
// time in TypeScript. This pins the two together: rewording the Go sentence
// without the shared definition would make every silent run read as running.
func TestSilencePrefixMatchesTheSharedRunStates(t *testing.T) {
	source, err := os.ReadFile("../../shared/runStates.ts")
	if err != nil {
		t.Fatal(err)
	}
	want := "export const RUN_SILENCE_PREFIX = '" + RunSilencePrefix + "'"
	if !strings.Contains(string(source), want) {
		t.Fatalf("shared/runStates.ts must declare %q", want)
	}
	if !strings.HasPrefix(RunSilenceNote(0), RunSilencePrefix) {
		t.Fatal("the silence sentence no longer opens on its prefix")
	}
}
