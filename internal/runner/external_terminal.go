package runner

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

// The shell that will read the launcher script. It decides the script's language, its file
// extension and how the supervised command line is quoted.
const (
	ShellPosix      = "posix"
	ShellPowerShell = "powershell"
	ShellCmd        = "cmd"
)

var terminalEnvKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// HostLauncher describes how to run a launcher script on this host.
type HostLauncher struct {
	Shell  string
	Binary string
}

// DetectHostLauncher picks the shell a Windows user actually works in. Forcing cmd.exe would
// run the task in a shell the user never chose, in the terminal they did.
func DetectHostLauncher(goos string) HostLauncher {
	if goos != "windows" {
		return HostLauncher{Shell: ShellPosix}
	}
	for _, candidate := range []string{"pwsh.exe", "powershell.exe"} {
		if bin, err := exec.LookPath(candidate); err == nil {
			return HostLauncher{Shell: ShellPowerShell, Binary: bin}
		}
	}
	return HostLauncher{Shell: ShellCmd, Binary: "cmd.exe"}
}

// HostShell names the shell that will run a supervised command line on this host.
func HostShell() string { return DetectHostLauncher(runtime.GOOS).Shell }

// Extension is the suffix the shell needs to recognise the script.
func (h HostLauncher) Extension() string {
	switch h.Shell {
	case ShellPowerShell:
		return ".ps1"
	case ShellCmd:
		return ".cmd"
	}
	return ".command"
}

// Argv runs the script and leaves the window open once it returns, so a finished skill can
// still be read. The execution policy is set explicitly: a machine that blocks scripts would
// otherwise refuse the launcher without saying why.
func (h HostLauncher) Argv(scriptPath string) []string {
	if h.Shell == ShellPowerShell {
		return []string{h.Binary, "-NoExit", "-ExecutionPolicy", "Bypass", "-File", scriptPath}
	}
	return []string{"cmd.exe", "/k", scriptPath}
}

// QuoteArg quotes one argument of a supervised command line for the named shell.
func QuoteArg(shell, s string) string {
	switch shell {
	case ShellPowerShell:
		return psQuote(s)
	case ShellCmd:
		// cmd.exe has no escape for a quote inside a quoted argument beyond the one
		// CommandLineToArgvW understands, and a percent sign would otherwise expand.
		return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `"`, `\"`), "%", "%%") + `"`
	}
	return shellQuote(s)
}

// psQuote renders a value as a PowerShell literal string, where doubling is the only escape.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// externalTerminalScript renders the launcher for the host the agent runs on.
func externalTerminalScript(targetPath, initialCommand string, env map[string]string) (string, error) {
	return externalTerminalScriptFor(HostShell(), targetPath, initialCommand, env)
}

// externalTerminalScriptFor renders the launcher for an explicitly named shell. The shell is a
// parameter rather than a build tag so every renderer is testable from any host; the Windows
// path rotted precisely because only a Windows machine could exercise it.
func externalTerminalScriptFor(shell, targetPath, initialCommand string, env map[string]string) (string, error) {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !terminalEnvKey.MatchString(key) {
			return "", fmt.Errorf("invalid terminal environment key %q", key)
		}
	}
	switch shell {
	case ShellPowerShell:
		return powershellTerminalScript(targetPath, initialCommand, env, keys)
	case ShellCmd:
		return batchTerminalScript(targetPath, initialCommand, env, keys)
	}
	return posixTerminalScript(targetPath, initialCommand, env, keys)
}

// posixTerminalScript renders values as literal shell data. In particular, displaying the
// initial command must not execute its substitutions a second time.
func posixTerminalScript(targetPath, initialCommand string, env map[string]string, keys []string) (string, error) {
	var b strings.Builder
	b.WriteString("#!/bin/bash\n# Sectile external terminal session\nrm -- \"$0\"\n")
	if customPath := dynamicCustomPathFor("linux"); customPath != "" {
		fmt.Fprintf(&b, "export PATH=%s:\"$PATH\"\n", shellQuote(customPath))
	}
	for _, key := range keys {
		fmt.Fprintf(&b, "export %s=%s\n", key, shellQuote(env[key]))
	}
	fmt.Fprintf(&b, "cd %s || exit 1\n", shellQuote(targetPath))
	fmt.Fprintf(&b, "printf '%%s\\n' %s\n", shellQuote("Sectile external terminal — "+targetPath))
	if initialCommand = strings.TrimSpace(initialCommand); initialCommand != "" {
		fmt.Fprintf(&b, "printf '%%s\\n' %s\n%s\n", shellQuote("Running: "+initialCommand), initialCommand)
	}
	b.WriteString("exec \"${SHELL:-/bin/zsh}\" -l\n")
	return b.String(), nil
}

// powershellTerminalScript renders the launcher for the shell a Windows user is most likely in.
// PowerShell parses a script file completely before running any of it, so the script can remove
// itself first: unlike the batch renderer, the agent token never outlives the launch.
func powershellTerminalScript(targetPath, initialCommand string, env map[string]string, keys []string) (string, error) {
	var b strings.Builder
	b.WriteString("# Sectile external terminal session\r\n")
	b.WriteString("Remove-Item -LiteralPath $PSCommandPath -Force -ErrorAction SilentlyContinue\r\n")
	if customPath := dynamicCustomPathFor("windows"); customPath != "" {
		fmt.Fprintf(&b, "$env:PATH = %s + ';' + $env:PATH\r\n", psQuote(customPath))
	}
	for _, key := range keys {
		fmt.Fprintf(&b, "$env:%s = %s\r\n", key, psQuote(env[key]))
	}
	fmt.Fprintf(&b, "Set-Location -LiteralPath %s\r\n", psQuote(targetPath))
	fmt.Fprintf(&b, "Write-Host %s\r\n", psQuote("Sectile external terminal - "+targetPath))
	if initialCommand = strings.TrimSpace(initialCommand); initialCommand != "" {
		fmt.Fprintf(&b, "Write-Host %s\r\n%s\r\n", psQuote("Running: "+initialCommand), initialCommand)
	}
	return b.String(), nil
}

// batchTerminalScript is the fallback for a Windows host without PowerShell. `cmd.exe` reads a
// batch file line by line, so the POSIX trick of deleting the script on its first line would
// corrupt the run; this script removes itself at the end instead.
func batchTerminalScript(targetPath, initialCommand string, env map[string]string, keys []string) (string, error) {
	var b strings.Builder
	b.WriteString("@echo off\r\nrem Sectile external terminal session\r\n")
	if customPath := dynamicCustomPathFor("windows"); customPath != "" {
		// Only the prefix is escaped; the trailing %PATH% must survive as an expansion.
		value, err := batchValue("PATH", customPath)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "set \"PATH=%s;%%PATH%%\"\r\n", value)
	}
	for _, key := range keys {
		value, err := batchValue(key, env[key])
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "set \"%s=%s\"\r\n", key, value)
	}
	target, err := batchValue("directory", targetPath)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, "cd /d \"%s\" || exit /b 1\r\n", target)
	fmt.Fprintf(&b, "echo %s\r\n", batchEcho("Sectile external terminal - "+targetPath))
	if initialCommand = strings.TrimSpace(initialCommand); initialCommand != "" {
		fmt.Fprintf(&b, "echo %s\r\n%s\r\n", batchEcho("Running: "+initialCommand), initialCommand)
	}
	// Closing the batch context before deleting lets the file remove itself while cmd still runs.
	b.WriteString("(goto) 2>nul & del /f /q \"%~f0\"\r\n")
	return b.String(), nil
}

// batchValue escapes a value for `set "KEY=value"`. A percent sign is doubled so it reaches the
// command as data. `cmd.exe` offers no escape for a quote inside a quoted assignment, so a value
// carrying one is refused rather than silently truncated.
func batchValue(name, value string) (string, error) {
	if strings.ContainsAny(value, "\"\r\n") {
		return "", fmt.Errorf("terminal environment value for %q cannot contain a quote or newline on Windows", name)
	}
	return strings.ReplaceAll(value, "%", "%%"), nil
}

// batchEcho escapes the shell metacharacters that would otherwise be interpreted in an echo.
func batchEcho(text string) string {
	text = strings.NewReplacer("\r", " ", "\n", " ").Replace(text)
	var b strings.Builder
	for _, r := range text {
		if strings.ContainsRune("^&<>|()%!\"", r) {
			b.WriteRune('^')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// joinPath concatenates PATH fragments with the host's list separator, tolerating empty parts.
func joinPath(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, string(os.PathListSeparator))
}

// prefixedPath puts the discovered tool directories ahead of the inherited PATH.
func prefixedPath() string {
	return joinPath(GetDynamicCustomPath(), os.Getenv("PATH"))
}

// windowsConsoleCommand opens the launcher in a new console window. `start` treats its first
// quoted argument as the window title, so the empty title is required or it swallows the command.
func windowsConsoleCommand(argv []string) *exec.Cmd {
	args := append([]string{"/c", "start", ""}, argv...)
	return exec.Command("cmd.exe", args...)
}
