package runner

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// The console session runs the user's own shell. What the agent needs to know about it is
// which one it is, because that decides how the launch line has to be quoted.
func TestDetectHostLauncherNamesTheUsersShell(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		if got := DetectHostLauncher(goos); got.Shell != ShellPosix {
			t.Errorf("%s should read POSIX, got %q", goos, got.Shell)
		}
	}
	windows := DetectHostLauncher("windows")
	if windows.Shell != ShellPowerShell && windows.Shell != ShellCmd {
		t.Fatalf("Windows must resolve to a Windows shell, got %q", windows.Shell)
	}
	if windows.Binary == "" {
		t.Error("a Windows launcher needs the binary to start")
	}
	if runtime.GOOS == "windows" && HostShell() != windows.Shell {
		t.Errorf("HostShell disagrees with the detected launcher: %q vs %q", HostShell(), windows.Shell)
	}
}

// PATH is joined with the host's separator: a Windows PATH glued with a colon is one
// unusable entry, and the discovered tool directories have to come first to be found.
func TestPrefixedPathUsesTheHostSeparator(t *testing.T) {
	got := joinPath("first", "", "second")
	want := "first" + string(os.PathListSeparator) + "second"
	if got != want {
		t.Fatalf("joinPath = %q, want %q", got, want)
	}
	if prefixed := prefixedPath(); !strings.HasPrefix(prefixed, GetDynamicCustomPath()) {
		t.Fatalf("discovered tools must come first: %q", prefixed)
	}
}
