package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"tasks/internal/agentexec"
)

// profileCodexVersion is deliberately conservative: these are the CLI
// interfaces attested by the profile protocol fixtures and local schema.
var profileCodexVersion = regexp.MustCompile(`codex-cli (\d+)\.(\d+)\.(\d+)`)

func profileCommand(ctx context.Context, provider string, args ...string) *exec.Cmd {
	cmd := agentexec.Hidden(exec.CommandContext(ctx, provider, args...))
	cmd.WaitDelay = headlessStopGrace
	return cmd
}

func checkProfileProvider(ctx context.Context, provider, template string) error {
	if template != "" || (provider != "claude" && provider != "codex") {
		return fmt.Errorf("Agent profiles require a built-in Claude or Codex engine without a custom command template")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if provider == "claude" {
		raw, err := profileCommand(ctx, "claude", "--help").Output()
		if err != nil || !strings.Contains(string(raw), "--append-system-prompt") {
			return fmt.Errorf("This Claude version cannot load supplemental agent instructions")
		}
		return nil
	}
	raw, err := profileCommand(ctx, "codex", "--version").Output()
	parts := profileCodexVersion.FindStringSubmatch(string(raw))
	if err != nil || len(parts) != 4 {
		return fmt.Errorf("Could not verify Codex agent-profile support")
	}
	major, _ := strconv.Atoi(parts[1])
	minor, _ := strconv.Atoi(parts[2])
	if major == 0 && minor < 162 {
		return fmt.Errorf("Agent profiles require Codex CLI 0.162 or newer")
	}
	return nil
}

// codexProfileInstructions reads effective native config without starting a
// thread or submitting a user request. Existing developer instructions are
// retained before the selected profile, in both views and later turns.
func codexProfileInstructions(ctx context.Context, directory, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := profileCommand(ctx, "codex", "app-server")
	cmd.Dir = directory
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	rpc := &codexRPC{input: &conversationInput{w: stdin}, pending: map[string]chan codexFrame{}, done: make(chan struct{}), notify: func(codexFrame) {}}
	cmd.Stdout = &conversationOutput{line: rpc.read}
	cmd.WaitDelay = headlessStopGrace
	release, err := agentexec.StartDetached(cmd)
	if err != nil {
		return "", fmt.Errorf("Could not start Codex configuration reader")
	}
	waited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(rpc.done); close(waited) }()
	defer func() { _ = stdin.Close(); agentexec.StopControlled(cmd, true); <-waited; release() }()
	if err = rpc.call(ctx, "initialize", map[string]any{"clientInfo": map[string]string{"name": "sectile-profile", "version": "1"}}, nil); err != nil {
		return "", fmt.Errorf("Codex configuration reader could not initialize")
	}
	if err = rpc.input.send(map[string]any{"method": "initialized"}); err != nil {
		return "", fmt.Errorf("Codex configuration reader disconnected")
	}
	var response struct {
		Config map[string]json.RawMessage `json:"config"`
	}
	if err = rpc.call(ctx, "config/read", map[string]any{"cwd": directory, "includeLayers": false}, &response); err != nil || response.Config == nil {
		return "", fmt.Errorf("Could not read Codex instruction configuration")
	}
	var existing string
	raw := response.Config["developer_instructions"]
	if len(raw) > 0 && string(raw) != "null" && json.Unmarshal(raw, &existing) != nil {
		return "", fmt.Errorf("Unsupported Codex developer instruction configuration")
	}
	if existing != "" {
		return existing + "\n\n" + prompt, nil
	}
	return prompt, nil
}

// A JSON string is also a TOML basic string for the accepted UTF-8 prompt
// alphabet, except JSON's escaped slash (which encoding/json never emits).
func profileConsoleCommand(command, provider string, profile *profileSnapshot) string {
	if profile == nil {
		return command
	}
	if provider == "claude" {
		return command + " --append-system-prompt " + quoteShell(profile.Prompt)
	}
	raw, _ := json.Marshal(profile.DeveloperInstructions)
	return command + " -c " + quoteShell("developer_instructions="+string(raw))
}

func applyProfile(run *controlledRun, profile *profileSnapshot) {
	run.profile = profile
	if profile != nil {
		run.desktop.AgentProfileID = profile.ID
		run.desktop.AgentProfileName = profile.ID
	}
}
