package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"tasks/internal/agentconfig"
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
		http.Error(w, "Invalid provider", 400)
		return
	}
	if r.Method != "GET" && r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	path, err := runner.FindCliTool(provider)
	if err != nil {
		if r.Method == "POST" {
			http.Error(w, "CLI not found on this workstation", 409)
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
		version := detectProviderVersion(ctx, path)
		models := detectProviderModels(ctx, path, provider)
		if len(models) > 0 && d.settingsRoot != "" {
			d.prepareMu.Lock()
			_, _ = d.updateSettings(func(settings *agentconfig.Settings) error {
				if settings.Defaults.AIProviderModels == nil {
					settings.Defaults.AIProviderModels = map[string][]string{}
				}
				settings.Defaults.AIProviderModels[provider] = models
				return nil
			})
			d.prepareMu.Unlock()
		} else if len(models) == 0 {
			if settings, readErr := d.readSettings(); readErr == nil {
				models = agentconfig.ProviderModels(settings.Defaults, provider)
			} else {
				models = agentconfig.ProviderModels(agentconfig.Defaults{}, provider)
			}
		}
		payload := map[string]any{
			"installed":      true,
			"path":           path,
			"authentication": status,
			"models":         models,
		}
		if version != "" {
			payload["version"] = version
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	launch, err := providerLoginTerminal(runtime.GOOS, path, providerAuthArgs(provider, true))
	if err != nil {
		http.Error(w, "Could not prepare sign-in terminal", 500)
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
		http.Error(w, "Could not open sign-in terminal", 500)
		return
	}
	go func() { _ = cmd.Wait() }()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"opened": true})
}

func detectProviderVersion(parent context.Context, path string) string {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	cmd := agentexec.Hidden(exec.CommandContext(ctx, path, "--version"))
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	if len(line) > 80 {
		line = line[:80]
	}
	return line
}

func detectProviderModels(parent context.Context, path, provider string) []string {
	switch provider {
	case "claude":
		return detectClaudeModels(parent, path)
	case "codex":
		if models := detectCodexModels(parent, path); len(models) > 0 {
			return models
		}
		if home, err := os.UserHomeDir(); err == nil {
			if raw, readErr := os.ReadFile(filepath.Join(home, ".codex", "models_cache.json")); readErr == nil {
				if models := parseCodexModelsCache(raw); len(models) > 0 {
					return models
				}
			}
		}
	}
	return nil
}

func detectClaudeModels(parent context.Context, path string) []string {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	cmd := agentexec.Hidden(exec.CommandContext(ctx, path, "-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose"))
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil
	}
	defer func() {
		_ = stdin.Close()
		cancel()
		_ = cmd.Wait()
	}()
	initMsg, _ := conversationInitialize()
	if raw, err := json.Marshal(initMsg); err == nil {
		_, _ = stdin.Write(append(raw, '\n'))
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		if models := parseClaudeInitializeModels(scanner.Bytes()); len(models) > 0 {
			return models
		}
	}
	return nil
}

func parseClaudeInitializeModels(line []byte) []string {
	var envelope struct {
		Type     string `json:"type"`
		Response struct {
			Subtype  string `json:"subtype"`
			Response struct {
				Models []struct {
					Value         string `json:"value"`
					ResolvedModel string `json:"resolvedModel"`
				} `json:"models"`
			} `json:"response"`
		} `json:"response"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(line), &envelope); err != nil || envelope.Type != "control_response" {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	add := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || candidate == "default" || agentconfig.ValidModel(candidate) != nil || seen[candidate] {
			return
		}
		seen[candidate] = true
		out = append(out, candidate)
	}
	for _, entry := range envelope.Response.Response.Models {
		if entry.Value == "default" {
			add(entry.ResolvedModel)
			continue
		}
		add(entry.ResolvedModel)
		add(entry.Value)
	}
	return out
}

func detectCodexModels(parent context.Context, path string) []string {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	cmd := agentexec.Hidden(exec.CommandContext(ctx, path, "app-server", "--listen", "stdio://"))
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil
	}
	defer func() {
		_ = stdin.Close()
		cancel()
		_ = cmd.Wait()
	}()
	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"sectile","version":"1.0"},"capabilities":{"experimentalApi":true}}}`,
		`{"jsonrpc":"2.0","method":"initialized","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"model/list","params":{"includeHidden":false}}`,
	}
	for _, req := range requests {
		if _, err := stdin.Write([]byte(req + "\n")); err != nil {
			return nil
		}
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		if models := parseCodexModelList(scanner.Bytes()); len(models) > 0 {
			return models
		}
	}
	return nil
}

func parseCodexModelList(line []byte) []string {
	var envelope struct {
		ID     int `json:"id"`
		Result struct {
			Data []struct {
				ID     string `json:"id"`
				Model  string `json:"model"`
				Hidden bool   `json:"hidden"`
			} `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(line), &envelope); err != nil || len(envelope.Result.Data) == 0 {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, entry := range envelope.Result.Data {
		if entry.Hidden {
			continue
		}
		candidate := strings.TrimSpace(entry.Model)
		if candidate == "" {
			candidate = strings.TrimSpace(entry.ID)
		}
		if candidate == "" || agentconfig.ValidModel(candidate) != nil || seen[candidate] {
			continue
		}
		seen[candidate] = true
		out = append(out, candidate)
	}
	return out
}

func parseCodexModelsCache(raw []byte) []string {
	var cache struct {
		Models []struct {
			Slug       string `json:"slug"`
			Visibility string `json:"visibility"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &cache); err != nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, entry := range cache.Models {
		if entry.Visibility == "hide" {
			continue
		}
		candidate := strings.TrimSpace(entry.Slug)
		if candidate == "" || agentconfig.ValidModel(candidate) != nil || seen[candidate] {
			continue
		}
		seen[candidate] = true
		out = append(out, candidate)
	}
	return out
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
