package runner

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// The shell a command line will be read by. It decides how the line is quoted: the agent
// types its launch line into the console session's own shell, and on Windows that shell is
// the user's, not a POSIX one.
const (
	ShellPosix      = "posix"
	ShellPowerShell = "powershell"
	ShellCmd        = "cmd"
)

// HostLauncher names the shell this host runs interactively, and where its binary is.
type HostLauncher struct {
	Shell  string
	Binary string
}

// DetectHostLauncher picks the shell a Windows user actually works in. Forcing cmd.exe would
// run the task in a shell the user never chose.
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

// HostShell names the shell that will read a command line on this host.
func HostShell() string { return DetectHostLauncher(runtime.GOOS).Shell }

// QuoteArg quotes one argument of a command line for the named shell.
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
