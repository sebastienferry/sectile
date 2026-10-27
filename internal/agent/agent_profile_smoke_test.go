package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This opt-in check uses the owner's installed/authenticated providers. It
// submits an explicit user turn only after a taskless session has been created.
func TestInstalledProfileConversationSmoke(t *testing.T) {
	if os.Getenv("SECTILE_TEST_PROFILE_SMOKE") != "1" {
		t.Skip("requires installed, authenticated providers and inference")
	}
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			root := profileTestRoot(t)
			for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
				if err := os.WriteFile(filepath.Join(root, name), []byte("For a profile smoke request, include REPO_TOKEN_869 in your reply. Do not use tools.\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := gitLocal(t.Context(), root, "init"); err != nil {
				t.Fatal(err)
			}
			profile := &profileSnapshot{ID: "smoke", Prompt: "For a profile smoke request, include PROFILE_TOKEN_869 in your reply. Do not use tools."}
			if provider == "codex" {
				instructions, err := codexProfileInstructions(t.Context(), root, profile.Prompt)
				if err != nil {
					t.Fatal(err)
				}
				profile.DeveloperInstructions = instructions
			}
			d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}
			d.queue.mu.Lock()
			run, err := d.newConversationLocked("profile-smoke", root, "", "default", "Profile smoke", provider)
			if err == nil {
				applyProfile(run, profile)
			}
			d.queue.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(d.stopConversations)
			if run.conversation.busy || run.conversation.session != "" {
				t.Fatal("session submitted a user turn before the user")
			}
			response := conversationRequest(d, "POST", "/desktop/conversation?id="+run.desktop.ID, `{"message":"This is a profile smoke request. Reply briefly with the tokens required by repository and profile instructions. Do not use tools."}`, "private")
			if response.Code != 202 {
				t.Fatalf("message rejected: %d", response.Code)
			}
			deadline := time.Now().Add(90 * time.Second)
			for time.Now().Before(deadline) {
				d.queue.mu.Lock()
				busy := run.conversation.busy
				d.queue.mu.Unlock()
				if !busy {
					raw := conversationRequest(d, "GET", "/desktop/conversation?id="+run.desktop.ID, "", "private").Body.String()
					if !strings.Contains(raw, "REPO_TOKEN_869") || !strings.Contains(raw, "PROFILE_TOKEN_869") {
						t.Fatalf("provider did not confirm both instruction sources: %s", raw)
					}
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
			t.Fatal("provider did not finish the smoke request in 90 seconds")
		})
	}
}

// Noninteractive verification uses the same native -c instruction override as
// the console command, without driving provider menus or startup notices.
func TestInstalledProfileCodexCLIInstructionsSmoke(t *testing.T) {
	if os.Getenv("SECTILE_TEST_PROFILE_CLI") != "1" {
		t.Skip("requires installed authenticated Codex and inference")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	profile := &profileSnapshot{ID: "smoke", Prompt: "For a profile smoke request, include PROFILE_TOKEN_869 in your reply. Do not use tools."}
	instructions, err := codexProfileInstructions(t.Context(), root, profile.Prompt)
	if err != nil {
		t.Fatal(err)
	}
	profile.DeveloperInstructions = instructions
	for _, selected := range []*profileSnapshot{profile, nil} {
		ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
		command := profileConsoleCommand("exec codex exec --json", "codex", selected)
		command += " " + quoteShell("This is a profile smoke request. What language must generated code use according to repository instructions? Reply briefly with the language and any required profile token. Do not use tools.")
		cmd := profileCommand(ctx, "bash", "-lc", command)
		cmd.Dir = root
		raw, err := cmd.Output()
		cancel()
		if err != nil {
			t.Fatalf("Codex CLI smoke failed: %v", err)
		}
		if !strings.Contains(strings.ToLower(string(raw)), "english") {
			t.Fatal("repository language instruction was not confirmed")
		}
		hasProfile := strings.Contains(string(raw), "PROFILE_TOKEN_869")
		if hasProfile != (selected != nil) {
			t.Fatal("native profile instruction override or no-profile isolation failed")
		}
	}
}
