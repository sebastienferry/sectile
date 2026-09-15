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

var terminalEnvKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// externalTerminalScript renders the launcher for the host the agent runs on.
func externalTerminalScript(targetPath, initialCommand string, env map[string]string) (string, error) {
	return externalTerminalScriptFor(runtime.GOOS, targetPath, initialCommand, env)
}

// externalTerminalScriptFor renders the launcher for an explicitly named platform. The target
// is a parameter rather than a build tag so the Windows script can be tested from any host;
// the Windows path rotted precisely because only a Windows machine could exercise it.
func externalTerminalScriptFor(goos, targetPath, initialCommand string, env map[string]string) (string, error) {
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
	if goos == "windows" {
		return windowsTerminalScript(targetPath, initialCommand, env, keys)
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

// windowsTerminalScript renders a batch launcher. `cmd.exe` reads a batch file line by line, so
// the POSIX trick of deleting the script on its first line would corrupt the run; the script
// removes itself at the end instead, which is why the agent token does not outlive it.
func windowsTerminalScript(targetPath, initialCommand string, env map[string]string, keys []string) (string, error) {
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
// quoted argument as the window title, so the empty title is required or it swallows the
// command; `/k` keeps the window open once the skill exits, which is the whole point of
// running in the host terminal.
func windowsConsoleCommand(scriptPath string) *exec.Cmd {
	return exec.Command("cmd.exe", "/c", "start", "", "cmd.exe", "/k", scriptPath)
}
