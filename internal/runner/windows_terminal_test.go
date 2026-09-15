package runner

import (
	"strings"
	"testing"
)

func TestWindowsScriptSetsEnvironmentAndRunsCommand(t *testing.T) {
	script, err := externalTerminalScriptFor("windows", `C:\git\sectile`, "claude -p 'go'",
		map[string]string{"SECTILE_TASK_KEY": "gh-144", "SECTILE_RUN_ID": "abc"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"@echo off",
		`set "SECTILE_RUN_ID=abc"`,
		`set "SECTILE_TASK_KEY=gh-144"`,
		`cd /d "C:\git\sectile" || exit /b 1`,
		"claude -p 'go'",
		`del /f /q "%~f0"`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
	// POSIX syntax in a batch file is the bug this replaces.
	for _, unwanted := range []string{"export ", "#!/bin/bash", `exec "${SHELL`} {
		if strings.Contains(script, unwanted) {
			t.Fatalf("script still carries POSIX syntax %q:\n%s", unwanted, script)
		}
	}
	// The variables must be set before the command that reads them.
	if strings.Index(script, "SECTILE_TASK_KEY") > strings.Index(script, "claude -p") {
		t.Fatal("environment is set after the command")
	}
}

func TestWindowsScriptKeepsPathExpansion(t *testing.T) {
	script, err := externalTerminalScriptFor("windows", `C:\repo`, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(script, "%%PATH%%") {
		t.Fatalf("PATH expansion was escaped into a literal:\n%s", script)
	}
	if strings.Contains(script, "set \"PATH=") && !strings.Contains(script, ";%PATH%\"") {
		t.Fatalf("PATH prefix dropped the inherited value:\n%s", script)
	}
	if strings.Contains(script, "/usr/bin") {
		t.Fatalf("Unix directories leaked into the Windows PATH:\n%s", script)
	}
}

func TestWindowsScriptEscapesPercentInValues(t *testing.T) {
	script, err := externalTerminalScriptFor("windows", `C:\repo`, "", map[string]string{"BRANCH": "feat/100%done"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, `set "BRANCH=feat/100%%done"`) {
		t.Fatalf("percent not doubled:\n%s", script)
	}
}

func TestWindowsScriptRefusesUnrepresentableValue(t *testing.T) {
	_, err := externalTerminalScriptFor("windows", `C:\repo`, "", map[string]string{"PROMPT": `say "hi"`})
	if err == nil || !strings.Contains(err.Error(), "PROMPT") {
		t.Fatalf("a value cmd.exe cannot express was accepted: %v", err)
	}
}

func TestScriptRenderingRejectsInvalidKeyOnBothHosts(t *testing.T) {
	for _, goos := range []string{"windows", "linux"} {
		if _, err := externalTerminalScriptFor(goos, "/repo", "", map[string]string{"BAD KEY": "x"}); err == nil {
			t.Fatalf("%s accepted an invalid environment key", goos)
		}
	}
}

func TestPosixScriptIsUnchangedByTheWindowsSplit(t *testing.T) {
	script, err := externalTerminalScriptFor("linux", "/repo", "echo hi", map[string]string{"K": "v"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"#!/bin/bash", `rm -- "$0"`, "export K='v'", "cd '/repo' || exit 1", `exec "${SHELL:-/bin/zsh}" -l`} {
		if !strings.Contains(script, want) {
			t.Fatalf("POSIX script missing %q:\n%s", want, script)
		}
	}
}

func TestJoinPathSkipsEmptyFragments(t *testing.T) {
	if got := joinPath("", "only"); got != "only" {
		t.Fatalf("joinPath left a separator behind: %q", got)
	}
}
