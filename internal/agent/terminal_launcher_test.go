package agent

import (
	"os"
	"strings"
	"testing"
)

func TestBuildTerminalLaunchDarwin(t *testing.T) {
	sessionID := "sess-123"
	exe := "/usr/local/bin/sectile-agent"
	url := "http://127.0.0.1:8090"
	token := "tok"

	// Terminal.app on macOS creates a script and open -a Terminal
	launchTerm, err := BuildTerminalLaunch("darwin", "terminal", exe, sessionID, url, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		if launchTerm.ScriptPath != "" {
			_ = os.Remove(launchTerm.ScriptPath)
		}
	}()
	if launchTerm.Name != "open" {
		t.Errorf("expected command 'open', got %q", launchTerm.Name)
	}
	if len(launchTerm.Args) != 3 || launchTerm.Args[0] != "-a" || launchTerm.Args[1] != "Terminal" {
		t.Errorf("unexpected args: %v", launchTerm.Args)
	}
	if launchTerm.ScriptPath == "" {
		t.Fatal("expected non-empty ScriptPath for Terminal.app launcher")
	}
	rawScript, err := os.ReadFile(launchTerm.ScriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawScript), "attach") || !strings.Contains(string(rawScript), sessionID) {
		t.Errorf("script does not contain attach command: %s", string(rawScript))
	}

	// iTerm on macOS
	launchIterm, err := BuildTerminalLaunch("darwin", "iterm", exe, sessionID, url, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		if launchIterm.ScriptPath != "" {
			_ = os.Remove(launchIterm.ScriptPath)
		}
	}()
	if launchIterm.Name != "open" || launchIterm.Args[1] != "iTerm" {
		t.Errorf("expected open -a iTerm, got %s %v", launchIterm.Name, launchIterm.Args)
	}
}

func TestBuildTerminalLaunchWindows(t *testing.T) {
	sessionID := "sess-win"
	exe := "C:\\sectile\\agent.exe"
	url := "http://127.0.0.1:8090"
	token := "tok"

	launchWT, err := BuildTerminalLaunch("windows", "wt", exe, sessionID, url, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if launchWT.Name != "wt.exe" {
		t.Errorf("expected wt.exe, got %q", launchWT.Name)
	}
	if len(launchWT.Args) < 5 || launchWT.Args[0] != "-w" || launchWT.Args[1] != "0" || launchWT.Args[2] != "nt" {
		t.Errorf("unexpected wt args: %v", launchWT.Args)
	}

	launchCmd, err := BuildTerminalLaunch("windows", "cmd", exe, sessionID, url, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if launchCmd.Name != "cmd.exe" {
		t.Errorf("expected cmd.exe, got %q", launchCmd.Name)
	}
	if len(launchCmd.Args) < 4 || launchCmd.Args[0] != "/c" || launchCmd.Args[1] != "start" {
		t.Errorf("unexpected cmd args: %v", launchCmd.Args)
	}
}

func TestBuildTerminalLaunchLinux(t *testing.T) {
	sessionID := "sess-linux"
	exe := "/usr/bin/sectile-agent"
	url := "http://127.0.0.1:8090"
	token := "tok"

	launchLinux, err := BuildTerminalLaunch("linux", "x-terminal-emulator", exe, sessionID, url, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if launchLinux.Name != "x-terminal-emulator" || launchLinux.Args[0] != "-e" {
		t.Errorf("unexpected linux launcher: %s %v", launchLinux.Name, launchLinux.Args)
	}

	launchGnome, err := BuildTerminalLaunch("linux", "gnome-terminal", exe, sessionID, url, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if launchGnome.Name != "gnome-terminal" || launchGnome.Args[0] != "--" {
		t.Errorf("unexpected gnome-terminal launcher: %s %v", launchGnome.Name, launchGnome.Args)
	}
}

func TestBuildTerminalLaunchCustomTemplates(t *testing.T) {
	sessionID := "sess-custom"
	exe := "sectile-agent"
	url := "http://127.0.0.1:8090"
	token := "tok"

	// Template with {command}
	launch1, err := BuildTerminalLaunch("darwin", "alacritty -e {command}", exe, sessionID, url, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if launch1.Name != "alacritty" || launch1.Args[0] != "-e" {
		t.Errorf("unexpected custom launch: %s %v", launch1.Name, launch1.Args)
	}
	if !strings.Contains(strings.Join(launch1.Args, " "), "attach --session sess-custom") {
		t.Errorf("expected args to include attach command, got: %v", launch1.Args)
	}

	// Template with {session}
	launch2, err := BuildTerminalLaunch("linux", "foot sectile-agent attach --session {session}", exe, sessionID, url, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if launch2.Name != "foot" || launch2.Args[len(launch2.Args)-1] != "sess-custom" {
		t.Errorf("unexpected {session} expansion: %s %v", launch2.Name, launch2.Args)
	}

	// Bare command (appends exe and attach args)
	launch3, err := BuildTerminalLaunch("linux", "kitty", exe, sessionID, url, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if launch3.Name != "kitty" || launch3.Args[0] != "-e" {
		t.Errorf("unexpected kitty launch: %s %v", launch3.Name, launch3.Args)
	}

	// Preserves casing for custom command paths and arguments
	launch4, err := BuildTerminalLaunch("darwin", "/Applications/CustomTerm.app/Contents/MacOS/CustomTerm -T 'Sectile' -e {command}", exe, sessionID, url, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if launch4.Name != "/Applications/CustomTerm.app/Contents/MacOS/CustomTerm" {
		t.Errorf("expected preserved casing, got: %s", launch4.Name)
	}
}
