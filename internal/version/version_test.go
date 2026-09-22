package version

import "testing"

// A build with nothing injected must say so. The whole point of the default is
// that an unreleased binary cannot be mistaken for a release.
func TestAnUninjectedBuildReportsDev(t *testing.T) {
	restore := Version
	t.Cleanup(func() { Version = restore })

	Version = ""
	if got := Current().Version; got != Dev {
		t.Fatalf("empty injected version reported as %q, want %q", got, Dev)
	}
}

func TestAnInjectedTagIsReportedVerbatim(t *testing.T) {
	restoreVersion, restoreCommit, restoreDate := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = restoreVersion, restoreCommit, restoreDate })

	Version, Commit, Date = " v1.4.0 ", "0123456789abcdef0123456789abcdef01234567", "2026-09-22T08:00:00Z"
	info := Current()
	if info.Version != "v1.4.0" {
		t.Errorf("version = %q, want v1.4.0", info.Version)
	}
	if info.Date != "2026-09-22T08:00:00Z" {
		t.Errorf("date = %q", info.Date)
	}
	// The one-line form abbreviates the commit; a full forty-character sha in
	// a footer or a log line is noise, not information.
	if got, want := String(), "v1.4.0 (0123456789ab)"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestStringOmitsAnUnknownCommit(t *testing.T) {
	restoreVersion, restoreCommit := Version, Commit
	t.Cleanup(func() { Version, Commit = restoreVersion, restoreCommit })

	Version, Commit = "v0.1.0", ""
	// Current() may still find a commit through the toolchain's own stamps, so
	// the assertion is on the prefix rather than on the whole string.
	if got := String(); got != "v0.1.0" && !hasPrefix(got, "v0.1.0 (") {
		t.Errorf("String() = %q, want v0.1.0 with an optional commit", got)
	}
}

func hasPrefix(s, prefix string) bool { return len(s) >= len(prefix) && s[:len(prefix)] == prefix }
