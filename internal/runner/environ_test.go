package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
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

// Windows spells the inherited variable "Path": a child environment that only
// looked for "PATH=" kept it and added a second PATH with the tool directories
// alone, and os/exec, folding names case-insensitively with the last one
// winning, handed the child a PATH without git on it.
func TestAChildEnvironmentCarriesOnePathEndingWithTheInheritedOne(t *testing.T) {
	inherited := strings.Join([]string{"inherited-a", "inherited-b"}, string(os.PathListSeparator))
	t.Setenv("PATH", inherited)

	var paths []string
	for _, entry := range PathEnviron() {
		if name, value, ok := strings.Cut(entry, "="); ok && strings.EqualFold(name, "PATH") {
			paths = append(paths, value)
		}
	}
	if len(paths) != 1 {
		t.Fatalf("the child environment holds %d PATH entries, want exactly one: %q", len(paths), paths)
	}
	if !strings.HasSuffix(paths[0], inherited) {
		t.Fatalf("PATH %q does not end with the inherited %q", paths[0], inherited)
	}
}

// Sectile Desktop opened from the Finder starts the agent with launchd's PATH.
// exec.Command resolves a bare "claude" against the agent's own PATH, so a CLI
// installed in ~/.local/bin was "not found" until the agent extended it.
func TestTheAgentFindsACliInstalledOutsideLaunchdPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("launchd's PATH is a macOS concern")
	}
	home := t.TempDir()
	bin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")
	if _, err := exec.LookPath("claude"); err == nil {
		t.Fatal("the fixture already finds claude on launchd's PATH")
	}

	ExtendProcessPath()

	got, err := exec.LookPath("claude")
	if err != nil {
		t.Fatalf("claude is still not found on %q: %v", os.Getenv("PATH"), err)
	}
	if got != filepath.Join(bin, "claude") {
		t.Fatalf("found %s, want the one in ~/.local/bin", got)
	}
	// The inherited directories stay, each once.
	dirs := filepath.SplitList(os.Getenv("PATH"))
	if !slices.Contains(dirs, "/usr/sbin") {
		t.Fatalf("an inherited directory was dropped: %q", os.Getenv("PATH"))
	}
	for i, dir := range dirs {
		if slices.Contains(dirs[i+1:], dir) {
			t.Fatalf("%s appears twice in %q", dir, os.Getenv("PATH"))
		}
	}
}
