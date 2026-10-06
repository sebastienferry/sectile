package agent

import (
	"fmt"
	"os/exec"
	"strings"
)

// BuildDirectoryTerminal is the command that opens a plain terminal window on
// directory, running the user's own shell: no Sectile session is attached to
// it. A custom terminal command names the folder with {dir}; without the
// placeholder the folder is appended as its last argument.
func BuildDirectoryTerminal(goos, terminalApp, directory string) (TerminalLaunch, error) {
	raw := strings.TrimSpace(terminalApp)
	if raw == "" {
		raw = detectDefaultTerminal()
	}
	switch goos {
	case "darwin":
		switch strings.ToLower(raw) {
		case "ghostty":
			return TerminalLaunch{Name: "open", Args: []string{"-na", "Ghostty", "--args", "--working-directory=" + directory}}, nil
		case "iterm", "iterm2":
			return TerminalLaunch{Name: "open", Args: []string{"-a", "iTerm", directory}}, nil
		case "terminal", "apple-terminal", "terminal.app":
			return TerminalLaunch{Name: "open", Args: []string{"-a", "Terminal", directory}}, nil
		}
	case "windows":
		switch strings.ToLower(raw) {
		case "wt", "windows-terminal", "wt.exe":
			return TerminalLaunch{Name: "wt.exe", Args: []string{"-w", "0", "nt", "-d", directory}}, nil
		case "cmd", "cmd.exe":
			return TerminalLaunch{Name: "cmd.exe", Args: []string{"/c", "start", "", "/D", directory, "cmd.exe"}}, nil
		}
	default:
		switch strings.ToLower(raw) {
		case "gnome-terminal":
			return TerminalLaunch{Name: "gnome-terminal", Args: []string{"--working-directory=" + directory}}, nil
		case "kitty":
			return TerminalLaunch{Name: "kitty", Args: []string{"--directory", directory}}, nil
		case "alacritty":
			return TerminalLaunch{Name: "alacritty", Args: []string{"--working-directory", directory}}, nil
		case "x-terminal-emulator", "terminal":
			// No flag is common to every emulator behind this name; the
			// process starts in the folder instead.
			return TerminalLaunch{Name: "x-terminal-emulator"}, nil
		}
	}
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return TerminalLaunch{}, fmt.Errorf("empty custom terminal command")
	}
	args, placed := make([]string, 0, len(parts)), false
	for _, part := range parts[1:] {
		if strings.Contains(part, "{dir}") {
			part, placed = strings.ReplaceAll(part, "{dir}", directory), true
		}
		args = append(args, part)
	}
	if !placed {
		args = append(args, directory)
	}
	return TerminalLaunch{Name: parts[0], Args: args}, nil
}

// openDirectoryTerminal opens a plain terminal on directory. The window is
// visible on purpose: the user asked for it, from the conversation's Terminal
// button or from Open terminal in a project's menu.
func (d *agentDaemon) openDirectoryTerminal(goos, terminalApp, directory string) error {
	if d.openTerminalFn != nil {
		return d.openTerminalFn(terminalApp, directory)
	}
	launch, err := BuildDirectoryTerminal(goos, terminalApp, directory)
	if err != nil {
		return err
	}
	cmd := exec.Command(launch.Name, launch.Args...)
	cmd.Dir = directory
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open terminal %s: %w", launch.Name, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
