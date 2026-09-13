package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalScriptPreservesValuesAndExecutesCommandOnce(t *testing.T) {
	root := t.TempDir()
	// Dollar signs and backticks in repository paths and environment values are data.
	target := filepath.Join(root, "checkout $HOME `literal`")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "marker")
	result := filepath.Join(root, "result")
	command := `printf '%s' "$(printf x >> ` + shellQuote(marker) + `; printf done)" > ` + shellQuote(result)
	script, err := externalTerminalScript(target, command, map[string]string{"SHELL": "/usr/bin/true", "TASKFLOW_TEST": "$HOME `literal` 'quote'"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "launch.command")
	if err = os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("bash", path).CombinedOutput()
	if err != nil {
		t.Fatalf("script failed: %v %s", err, out)
	}
	raw, _ := os.ReadFile(marker)
	if string(raw) != "x" {
		t.Fatalf("command executed more than once: %q", raw)
	}
	raw, _ = os.ReadFile(result)
	if string(raw) != "done" {
		t.Fatalf("bad result %q", raw)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("launcher script was not cleaned up")
	}
}

func TestExternalTerminalReportsLauncherFailure(t *testing.T) {
	// A custom launcher exercises the real process exit path without opening a UI.
	err := NewRunner().OpenExternalTerminal("sh -c 'echo terminal-unavailable >&2; exit 7' -- {script}", t.TempDir(), "", nil)
	if err == nil || !strings.Contains(err.Error(), "terminal-unavailable") {
		t.Fatalf("launcher failure hidden: %v", err)
	}
}
