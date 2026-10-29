package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"tasks/internal/agentexec"
	"tasks/internal/runner"
)

func providerAuthArgs(provider string, login bool) []string {
	if provider == "claude" {
		if login {
			return []string{"auth", "login"}
		}
		return []string{"auth", "status"}
	}
	if login {
		return []string{"login"}
	}
	return []string{"login", "status"}
}

// desktopProviderLogin uses only official CLI commands; credentials never enter HTTP responses.
func (d *agentDaemon) desktopProviderLogin(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	if provider != "claude" && provider != "codex" {
		http.Error(w, "Fournisseur invalide", 400)
		return
	}
	if r.Method != "GET" && r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	path, err := runner.FindCliTool(provider)
	if err != nil {
		if r.Method == "POST" {
			http.Error(w, "CLI introuvable sur ce poste", 409)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"installed": false, "authentication": "unknown"})
		return
	}
	if r.Method == "GET" {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		cmd := providerStatusCommand(ctx, path, provider)
		// Never return CLI output: authentication diagnostics can contain account details or secrets.
		err := cmd.Run()
		status := "unknown"
		if err == nil {
			status = "connected"
		} else if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 && ctx.Err() == nil {
			status = "disconnected"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"installed": true, "path": path, "authentication": status})
		return
	}
	launch, err := providerLoginTerminal(runtime.GOOS, path, providerAuthArgs(provider, true))
	if err != nil {
		http.Error(w, "Impossible de préparer le terminal de connexion", 500)
		return
	}
	// Visible intentionally: the user requested the provider's interactive sign-in terminal.
	cmd := exec.Command(launch.Name, launch.Args...)
	if home, e := os.UserHomeDir(); e == nil {
		cmd.Dir = home
	}
	if err = cmd.Start(); err != nil {
		if launch.ScriptPath != "" {
			_ = os.Remove(launch.ScriptPath)
		}
		http.Error(w, "Impossible d’ouvrir le terminal de connexion", 500)
		return
	}
	go func() { _ = cmd.Wait() }()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"opened": true})
}

func providerLoginTerminal(goos, path string, args []string) (TerminalLaunch, error) {
	switch goos {
	case "darwin":
		file, err := os.CreateTemp("", "sectile-login-*.command")
		if err != nil {
			return TerminalLaunch{}, err
		}
		script := file.Name()
		quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
		command := quote(path)
		for _, arg := range args {
			command += " " + quote(arg)
		}
		_, err = file.WriteString("#!/bin/sh\nrm -f \"$0\"\nexec " + command + "\n")
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Chmod(script, 0700)
		}
		if err != nil {
			_ = os.Remove(script)
			return TerminalLaunch{}, err
		}
		return TerminalLaunch{Name: "open", Args: []string{"-a", "Terminal", script}, ScriptPath: script}, err
	case "windows":
		return TerminalLaunch{Name: "cmd.exe", Args: append([]string{"/c", "start", "", path}, args...)}, nil
	default:
		return TerminalLaunch{Name: "x-terminal-emulator", Args: append([]string{"-e", path}, args...)}, nil
	}
}

func providerStatusCommand(ctx context.Context, path, provider string) *exec.Cmd {
	cmd := agentexec.Hidden(exec.CommandContext(ctx, path, providerAuthArgs(provider, false)...))
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	return cmd
}
