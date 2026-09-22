// Package testhome points os.UserHomeDir at a directory of the test's choosing
// on every platform. It reads HOME on POSIX and USERPROFILE on Windows: a test
// that sets only the first writes into the developer's real home directory on
// the second, which is how test runs overwrote a real ~/.config/sectile and
// installed retired Claude Code hooks into a real ~/.claude (#351).
package testhome

import "testing"

// Set makes home the directory os.UserHomeDir returns for the rest of the test.
func Set(t testing.TB, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

// Temp creates a temporary directory, makes it the home and returns it.
func Temp(t testing.TB) string {
	t.Helper()
	home := t.TempDir()
	Set(t, home)
	return home
}
