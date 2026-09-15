package runner

import (
	"strings"
	"testing"
)

func TestWindowsScriptSetsEnvironmentAndRunsCommand(t *testing.T) {
	script, err := externalTerminalScriptFor(ShellCmd, `C:\git\sectile`, "claude -p 'go'",
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
	script, err := externalTerminalScriptFor(ShellCmd, `C:\repo`, "", nil)
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
	script, err := externalTerminalScriptFor(ShellCmd, `C:\repo`, "", map[string]string{"BRANCH": "feat/100%done"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, `set "BRANCH=feat/100%%done"`) {
		t.Fatalf("percent not doubled:\n%s", script)
	}
}

func TestWindowsScriptRefusesUnrepresentableValue(t *testing.T) {
	_, err := externalTerminalScriptFor(ShellCmd, `C:\repo`, "", map[string]string{"PROMPT": `say "hi"`})
	if err == nil || !strings.Contains(err.Error(), "PROMPT") {
		t.Fatalf("a value cmd.exe cannot express was accepted: %v", err)
	}
}

func TestScriptRenderingRejectsInvalidKeyOnBothHosts(t *testing.T) {
	for _, goos := range []string{ShellCmd, ShellPowerShell, ShellPosix} {
		if _, err := externalTerminalScriptFor(goos, "/repo", "", map[string]string{"BAD KEY": "x"}); err == nil {
			t.Fatalf("%s accepted an invalid environment key", goos)
		}
	}
}

func TestPosixScriptIsUnchangedByTheWindowsSplit(t *testing.T) {
	script, err := externalTerminalScriptFor(ShellPosix, "/repo", "echo hi", map[string]string{"K": "v"})
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

func TestPowerShellScriptUsesPowerShellSyntax(t *testing.T) {
	script, err := externalTerminalScriptFor(ShellPowerShell, `C:\git\sectile`, "& 'agent.exe' agent-exec",
		map[string]string{"SECTILE_TASK_KEY": "gh-144"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`$env:SECTILE_TASK_KEY = 'gh-144'`,
		`Set-Location -LiteralPath 'C:\git\sectile'`,
		"& 'agent.exe' agent-exec",
		`Remove-Item -LiteralPath $PSCommandPath`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
	for _, unwanted := range []string{"@echo off", "set \"", "export ", "cd /d"} {
		if strings.Contains(script, unwanted) {
			t.Fatalf("script carries foreign syntax %q:\n%s", unwanted, script)
		}
	}
	// PowerShell parses the whole file before running it, so the token can go before the command.
	if strings.Index(script, "Remove-Item") > strings.Index(script, "agent-exec") {
		t.Fatal("the launcher outlives the command it starts")
	}
}

// A value a Windows user really can hit: a quote is fatal to the batch renderer but ordinary here.
func TestPowerShellScriptAcceptsValuesBatchCannotExpress(t *testing.T) {
	env := map[string]string{"PROMPT": `say "hi" it's fine`}
	if _, err := externalTerminalScriptFor(ShellCmd, `C:\r`, "", env); err == nil {
		t.Fatal("the batch renderer should refuse a quote it cannot escape")
	}
	script, err := externalTerminalScriptFor(ShellPowerShell, `C:\r`, "", env)
	if err != nil {
		t.Fatalf("powershell should express it: %v", err)
	}
	if !strings.Contains(script, `$env:PROMPT = 'say "hi" it''s fine'`) {
		t.Fatalf("value not rendered as a literal:\n%s", script)
	}
}

func TestLauncherPicksTheUsersShell(t *testing.T) {
	ps := HostLauncher{Shell: ShellPowerShell, Binary: `C:\pwsh.exe`}
	if ps.Extension() != ".ps1" {
		t.Fatalf("powershell script needs a .ps1 extension, got %s", ps.Extension())
	}
	argv := ps.Argv(`C:\t.ps1`)
	if argv[0] != `C:\pwsh.exe` || !contains(argv, "-NoExit") || !contains(argv, "-File") {
		t.Fatalf("powershell launcher argv is wrong: %v", argv)
	}
	if !contains(argv, "Bypass") {
		t.Fatal("a machine that blocks scripts would refuse the launcher without saying why")
	}
	cmdLauncher := HostLauncher{Shell: ShellCmd, Binary: "cmd.exe"}
	if cmdLauncher.Extension() != ".cmd" || cmdLauncher.Argv("x")[0] != "cmd.exe" {
		t.Fatalf("cmd fallback is wrong: %v", cmdLauncher.Argv("x"))
	}
	if posix := (HostLauncher{Shell: ShellPosix}); posix.Extension() != ".command" {
		t.Fatalf("posix extension changed: %s", posix.Extension())
	}
	if got := DetectHostLauncher("darwin").Shell; got != ShellPosix {
		t.Fatalf("non-Windows host must stay POSIX, got %s", got)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
