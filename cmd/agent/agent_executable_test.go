package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A registration written with a throwaway binary points native clients at a
// path that stops resolving, and the only symptom is a run that fails with no
// explanation. The check is what keeps that from being written at all.
func TestTemporaryExecutablesAreRecognised(t *testing.T) {
	temporary := []string{
		"/Users/someone/Library/Caches/go-build/77/7755756f8653b54f74f2ff12ff549b3f214fadba5e17f99096c9ca8310e143ee-d/agent",
		"/home/someone/.cache/go-build/ab/abcdef-d/agent",
		filepath.Join(os.TempDir(), "go-build123", "b001", "agent"),
	}
	for _, path := range temporary {
		if !temporaryExecutable(path) {
			t.Errorf("temporaryExecutable(%q) = false, want true", path)
		}
	}

	installed := []string{
		"/Users/someone/Sources/sectile/bin/agent",
		"/usr/local/bin/sectile-agent",
		"/opt/sectile/sectile-agent",
		"/Applications/Sectile.app/Contents/Resources/sectile-agent",
	}
	for _, path := range installed {
		if temporaryExecutable(path) {
			t.Errorf("temporaryExecutable(%q) = true, want false", path)
		}
	}
}

// The guard refuses to register MCP from a throwaway binary. A test binary is
// exactly that, so without this exception the guard fails the suite it was
// added to protect, which is how it shipped the first time.
func TestTheGuardDoesNotFireInsideATestBinary(t *testing.T) {
	if !runningUnderTest() {
		t.Fatal("runningUnderTest is false inside a test binary, so the guard would refuse every test run")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if !temporaryExecutable(executable) {
		t.Skip("this test binary is not in a temporary location, so there is nothing to exempt")
	}
	if temporaryExecutable(executable) && !runningUnderTest() {
		t.Fatal("the guard would fire on a test binary")
	}
}
