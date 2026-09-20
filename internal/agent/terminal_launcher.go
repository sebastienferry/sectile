package agent

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// TerminalLaunch holds the binary and arguments to invoke an external terminal.
type TerminalLaunch struct {
	Name       string
	Args       []string
	ScriptPath string
}

// BuildTerminalLaunch constructs the execution command for the target terminal emulator.
func BuildTerminalLaunch(goos, terminalApp, exePath, sessionID, loopbackURL, token string) (TerminalLaunch, error) {
	terminalApp = strings.ToLower(strings.TrimSpace(terminalApp))
	if terminalApp == "" {
		terminalApp = detectDefaultTerminal()
	}

	attachArgs := []string{"attach", "--session", sessionID}
	if loopbackURL != "" {
		attachArgs = append(attachArgs, "--url", loopbackURL)
	}
	if token != "" {
		attachArgs = append(attachArgs, "--token", token)
	}

	fullAttachCmd := exePath + " " + strings.Join(attachArgs, " ")

	switch goos {
	case "darwin":
		switch terminalApp {
		case "ghostty":
			// If ghostty CLI is available in PATH, launch directly with ghostty -e
			if _, err := exec.LookPath("ghostty"); err == nil {
				args := append([]string{"-e", exePath}, attachArgs...)
				return TerminalLaunch{Name: "ghostty", Args: args}, nil
			}
			// Otherwise launch via temporary script and open -a Ghostty
			script, err := createTempLauncherScript(exePath, attachArgs)
			if err != nil {
				return TerminalLaunch{}, err
			}
			return TerminalLaunch{Name: "open", Args: []string{"-a", "Ghostty", script}, ScriptPath: script}, nil
		case "iterm", "iterm2":
			script, err := createTempLauncherScript(exePath, attachArgs)
			if err != nil {
				return TerminalLaunch{}, err
			}
			return TerminalLaunch{Name: "open", Args: []string{"-a", "iTerm", script}, ScriptPath: script}, nil
		case "terminal", "apple-terminal", "terminal.app":
			script, err := createTempLauncherScript(exePath, attachArgs)
			if err != nil {
				return TerminalLaunch{}, err
			}
			return TerminalLaunch{Name: "open", Args: []string{"-a", "Terminal", script}, ScriptPath: script}, nil
		default:
			return buildCustomTerminalLaunch(terminalApp, exePath, sessionID, attachArgs, fullAttachCmd)
		}

	case "windows":
		switch terminalApp {
		case "wt", "windows-terminal", "wt.exe":
			args := append([]string{"-w", "0", "nt", exePath}, attachArgs...)
			return TerminalLaunch{Name: "wt.exe", Args: args}, nil
		case "cmd", "cmd.exe":
			args := append([]string{"/c", "start", exePath}, attachArgs...)
			return TerminalLaunch{Name: "cmd.exe", Args: args}, nil
		default:
			return buildCustomTerminalLaunch(terminalApp, exePath, sessionID, attachArgs, fullAttachCmd)
		}

	default: // Linux and other POSIX
		switch terminalApp {
		case "x-terminal-emulator", "terminal":
			args := append([]string{"-e", exePath}, attachArgs...)
			return TerminalLaunch{Name: "x-terminal-emulator", Args: args}, nil
		case "gnome-terminal":
			args := append([]string{"--", exePath}, attachArgs...)
			return TerminalLaunch{Name: "gnome-terminal", Args: args}, nil
		case "kitty", "alacritty":
			args := append([]string{"-e", exePath}, attachArgs...)
			return TerminalLaunch{Name: terminalApp, Args: args}, nil
		default:
			return buildCustomTerminalLaunch(terminalApp, exePath, sessionID, attachArgs, fullAttachCmd)
		}
	}
}

func buildCustomTerminalLaunch(customCmd, exePath, sessionID string, attachArgs []string, fullAttachCmd string) (TerminalLaunch, error) {
	if strings.Contains(customCmd, "{command}") {
		expanded := strings.ReplaceAll(customCmd, "{command}", fullAttachCmd)
		if strings.Contains(expanded, "{session}") {
			expanded = strings.ReplaceAll(expanded, "{session}", sessionID)
		}
		parts := strings.Fields(expanded)
		if len(parts) == 0 {
			return TerminalLaunch{}, fmt.Errorf("empty custom terminal command")
		}
		return TerminalLaunch{Name: parts[0], Args: parts[1:]}, nil
	}
	if strings.Contains(customCmd, "{session}") {
		expanded := strings.ReplaceAll(customCmd, "{session}", sessionID)
		parts := strings.Fields(expanded)
		if len(parts) == 0 {
			return TerminalLaunch{}, fmt.Errorf("empty custom terminal command")
		}
		return TerminalLaunch{Name: parts[0], Args: parts[1:]}, nil
	}

	parts := strings.Fields(customCmd)
	if len(parts) == 0 {
		return TerminalLaunch{}, fmt.Errorf("empty custom terminal command")
	}
	args := append(parts[1:], exePath)
	args = append(args, attachArgs...)
	return TerminalLaunch{Name: parts[0], Args: args}, nil
}

func createTempLauncherScript(exePath string, attachArgs []string) (string, error) {
	tmpDir := os.TempDir()
	f, err := os.CreateTemp(tmpDir, "sectile-attach-*.command")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary terminal launcher script: %w", err)
	}
	defer f.Close()

	quotedArgs := make([]string, len(attachArgs))
	for i, arg := range attachArgs {
		quotedArgs[i] = fmt.Sprintf("%q", arg)
	}

	scriptContent := fmt.Sprintf("#!/bin/sh\nrm -f \"$0\" 2>/dev/null\nexec %q %s\n", exePath, strings.Join(quotedArgs, " "))
	if _, err := f.WriteString(scriptContent); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("failed to write launcher script: %w", err)
	}

	if err := os.Chmod(f.Name(), 0755); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("failed to make launcher script executable: %w", err)
	}

	return f.Name(), nil
}

// launchExternalTerminal launches the external terminal process.
func (d *agentDaemon) launchExternalTerminal(terminalApp, sessionID string) error {
	if d.launchTerminalFn != nil {
		return d.launchTerminalFn(terminalApp, sessionID)
	}

	exe, err := os.Executable()
	if err != nil || exe == "" {
		exe = "sectile-agent"
	}

	loopbackURL := d.loopback.url
	token := d.loopback.desktopToken

	launch, err := BuildTerminalLaunch(runtime.GOOS, terminalApp, exe, sessionID, loopbackURL, token)
	if err != nil {
		return err
	}

	cmd := exec.Command(launch.Name, launch.Args...)
	if err := cmd.Start(); err != nil {
		if launch.ScriptPath != "" {
			_ = os.Remove(launch.ScriptPath)
		}
		return fmt.Errorf("failed to spawn native terminal %s: %w", launch.Name, err)
	}

	// Detach process so agent doesn't hold zombie references
	go func() {
		_ = cmd.Wait()
	}()

	return nil
}
