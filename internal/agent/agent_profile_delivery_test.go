package agent

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"tasks/internal/agentconfig"
	"testing"
	"time"

	"tasks/internal/agentexec"
)

func TestProfileConsoleInstructionsAreShellData(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell quoting; Windows process flags are covered separately")
	}
	root := profileTestRoot(t)
	sentinel := filepath.Join(root, "unexpected")
	prompt := "review 'quoted' \"double\"\n$(touch " + sentinel + "); 你好 \\ end"
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			profile := &profileSnapshot{Prompt: prompt, DeveloperInstructions: "native\n\n" + prompt}
			command := profileConsoleCommand("printf '%s\\n'", provider, profile)
			raw, err := agentexec.Hidden(exec.Command("sh", "-c", command)).Output()
			if err != nil {
				t.Fatal(err)
			}
			if provider == "claude" && string(raw) != "--append-system-prompt\n"+prompt+"\n" {
				t.Fatalf("Claude argv: %q", raw)
			}
			if provider == "codex" {
				value := strings.TrimSuffix(strings.TrimPrefix(string(raw), "-c\ndeveloper_instructions="), "\n")
				var decoded string
				if json.Unmarshal([]byte(value), &decoded) != nil || decoded != profile.DeveloperInstructions {
					t.Fatalf("Codex TOML value: %q", raw)
				}
			}
			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Fatal("prompt ran a shell command")
			}
		})
	}
}

func TestClaudeProfileIsAppendedOnInitialAndResumedTurns(t *testing.T) {
	profile := &profileSnapshot{Prompt: "follow repository instructions\nthen review"}
	for _, session := range []string{"", "session-id"} {
		cmd := claudeConversationCommand(t.TempDir(), "sonnet", "high", "default", session, "", []string{"/context"}, nil, profile)
		joined := strings.Join(cmd.Args, "\x00")
		if !strings.Contains(joined, "--append-system-prompt\x00"+profile.Prompt) || strings.Contains(joined, "\x00--system-prompt\x00") {
			t.Fatalf("instruction replacement: %q", cmd.Args)
		}
		if !strings.Contains(joined, "--add-dir=/context") {
			t.Fatal("folder context lost")
		}
	}
	plain := claudeConversationCommand(t.TempDir(), "", "", "", "", "", nil, nil)
	if strings.Contains(strings.Join(plain.Args, " "), "system-prompt") {
		t.Fatal("profile leaked into plain session")
	}
}

func TestCodexProfileConfigurationComposition(t *testing.T) {
	d, _, _ := codexConversationFixture(t)
	instructions, err := codexProfileInstructions(t.Context(), d.queue.runs["source"].desktop.Directory, "profile instructions")
	if err != nil || instructions != "native instructions\n\nprofile instructions" {
		t.Fatalf("composition: %q %v", instructions, err)
	}
}

func TestCodexProfileIsSentOnThreadResumeAndLaterTurns(t *testing.T) {
	d, id, log := codexConversationFixture(t)
	d.queue.mu.Lock()
	run := d.queue.runs[id]
	applyProfile(run, &profileSnapshot{ID: "reviewer", Prompt: "profile", DeveloperInstructions: "native\n\nprofile"})
	d.queue.mu.Unlock()
	// Crash the initially unprofiled fixture process, then inspect the actual
	// resumed thread request and all subsequent collaboration-mode turn requests.
	codexMessage(t, d, id, `{"message":"crash"}`)
	waitCodex(t, d, id, func(c *providerConversation) bool { return c.codex == nil && !c.busy })
	for _, message := range []string{"resume", "later"} {
		codexMessage(t, d, id, `{"message":"`+message+`"}`)
		waitCodex(t, d, id, func(c *providerConversation) bool { return c.codex != nil && !c.busy })
	}
	foundResume, foundTurns := false, 0
	for _, line := range strings.Split(codexLog(t, log), "\n") {
		var frame struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if json.Unmarshal([]byte(line), &frame) != nil {
			continue
		}
		if frame.Method == "thread/resume" {
			foundResume = true
			if frame.Params["developerInstructions"] != "native\n\nprofile" {
				t.Fatal("resume lost instructions")
			}
		}
		if frame.Method == "turn/start" {
			foundTurns++
			settings := frame.Params["collaborationMode"].(map[string]any)["settings"].(map[string]any)
			if settings["developer_instructions"] != "native\n\nprofile" {
				t.Fatal("turn lost instructions")
			}
		}
	}
	if !foundResume || foundTurns != 3 {
		t.Fatalf("resume=%v turns=%d", foundResume, foundTurns)
	}
}

func TestUnsupportedProfileDeliveryDoesNotLaunchAProcess(t *testing.T) {
	for _, provider := range []string{"agy", "custom", ""} {
		if err := checkProfileProvider(t.Context(), provider, ""); err == nil {
			t.Fatalf("accepted %q", provider)
		}
	}
	if err := checkProfileProvider(t.Context(), "codex", "custom template"); err == nil {
		t.Fatal("accepted custom template")
	}
}

// Opt-in local compatibility check: no thread, user turn or inference is sent.
func TestInstalledProfileProviders(t *testing.T) {
	if os.Getenv("SECTILE_TEST_PROFILE_PROVIDERS") != "1" {
		t.Skip("requires installed supported CLIs")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	for _, provider := range []string{"claude", "codex"} {
		if err := checkProfileProvider(ctx, provider, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := codexProfileInstructions(ctx, profileTestRoot(t), "profile probe"); err != nil {
		t.Fatal(err)
	}
}

func TestCodexProfileIsSentOnInitialThread(t *testing.T) {
	_, _, log := codexConversationFixture(t, &profileSnapshot{ID: "initial", Prompt: "profile", DeveloperInstructions: "native\n\ninitial"})
	for _, line := range strings.Split(codexLog(t, log), "\n") {
		var frame struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if json.Unmarshal([]byte(line), &frame) == nil && frame.Method == "thread/start" {
			if frame.Params["developerInstructions"] != "native\n\ninitial" {
				t.Fatal("initial thread lost instructions")
			}
			return
		}
	}
	t.Fatal("no thread start request")
}

func TestProfileProviderVersionAndCapabilityRejection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture; Windows flags are tested separately")
	}
	root := profileTestRoot(t)
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, version := range []string{"0.161.0", "invalid"} {
		if err := os.WriteFile(filepath.Join(root, "codex"), []byte("#!/bin/sh\nprintf 'codex-cli "+version+"\\n'\n"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := checkProfileProvider(t.Context(), "codex", ""); err == nil {
			t.Fatalf("accepted unsupported version %q", version)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "claude"), []byte("#!/bin/sh\nprintf 'no additive instruction option\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := checkProfileProvider(t.Context(), "claude", ""); err == nil {
		t.Fatal("accepted Claude without additive delivery")
	}
}

func TestCodexSimultaneousProfilesDoNotLeakIntoPlainSessions(t *testing.T) {
	d, first, log := codexConversationFixture(t, &profileSnapshot{ID: "first", Prompt: "first", DeveloperInstructions: "native plus first"})
	ids := []string{first}
	for _, profile := range []*profileSnapshot{{ID: "second", Prompt: "second", DeveloperInstructions: "native plus second"}, nil} {
		d.queue.mu.Lock()
		run, err := d.newConversationLocked("p", d.queue.runs[first].desktop.Directory, "codex-test", "workspace-write", "isolation", "codex")
		if err == nil {
			applyProfile(run, profile)
		}
		d.queue.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, run.desktop.ID)
		conversationRequest(d, "GET", "/desktop/conversation?id="+run.desktop.ID, "", "private")
		waitCodex(t, d, run.desktop.ID, func(c *providerConversation) bool { return c.input != nil && len(c.models) > 0 })
	}
	for _, id := range ids {
		codexMessage(t, d, id, `{"message":"hello"}`)
	}
	for _, id := range ids {
		waitCodex(t, d, id, func(c *providerConversation) bool { return !c.busy })
	}
	threads, turns := map[string]int{}, map[string]int{}
	for _, line := range strings.Split(codexLog(t, log), "\n") {
		var frame struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if json.Unmarshal([]byte(line), &frame) != nil {
			continue
		}
		if frame.Method == "thread/start" {
			value, _ := frame.Params["developerInstructions"].(string)
			threads[value]++
			if _, exists := frame.Params["baseInstructions"]; exists {
				t.Fatal("replaced repository base instructions")
			}
		}
		if frame.Method == "turn/start" {
			settings := frame.Params["collaborationMode"].(map[string]any)["settings"].(map[string]any)
			value, _ := settings["developer_instructions"].(string)
			turns[value]++
		}
	}
	for _, value := range []string{"native plus first", "native plus second", ""} {
		if threads[value] != 1 || turns[value] != 1 {
			t.Fatalf("instruction isolation failed: threads=%v turns=%v", threads, turns)
		}
	}
}

func TestClaudeProfileProcessesKeepSnapshotsAcrossTurns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX provider fixture")
	}
	bin := profileTestRoot(t)
	script := `#!/bin/sh
if [ "$1" = "--help" ]; then printf '%s\n' '--append-system-prompt'; exit; fi
read -r init
read -r message
printf '%s\000' "$@" >> "$PROFILE_ARGS"
printf '%s\n' '{"type":"result","is_error":false,"result":"ok","session_id":"11111111-1111-4111-8111-111111111111"}'
`
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, prompt := range []string{"first '\"\n你好 $(touch unexpected)", "second instructions", ""} {
		d, id := conversationFixture(t)
		run := d.queue.runs[id]
		log := filepath.Join(profileTestRoot(t), "args")
		run.conversation.env = map[string]string{"PROFILE_ARGS": log}
		if prompt != "" {
			applyProfile(run, &profileSnapshot{ID: "snapshot", Prompt: prompt})
		}
		for range 2 {
			response := conversationRequest(d, "POST", "/desktop/conversation?id="+id, `{"message":"user request"}`, "private")
			if response.Code != 202 {
				t.Fatal(response.Body.String())
			}
			waitConversationIdle(t, d, run)
		}
		raw, err := os.ReadFile(log)
		if err != nil {
			t.Fatal(err)
		}
		args := strings.Split(string(raw), "\x00")
		appended, resumes := 0, 0
		for i, arg := range args {
			if arg == "--append-system-prompt" {
				appended++
				if i+1 >= len(args) || args[i+1] != prompt {
					t.Fatal("profile was modified between turns")
				}
			}
			if arg == "--resume" {
				resumes++
			}
			if arg == "--system-prompt" {
				t.Fatal("repository instructions replaced")
			}
		}
		expected := 2
		if prompt == "" {
			expected = 0
		}
		if appended != expected || resumes != 1 {
			t.Fatalf("append=%d resume=%d", appended, resumes)
		}
	}
}

func TestProfileAdmissionReadsAndSnapshotsTheSelectedFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX capability probe fixture")
	}
	d := profileHTTPFixture(t)
	bin := profileTestRoot(t)
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\nprintf '%s\\n' '--append-system-prompt'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := createAgentProfile(d.repoRoot, "reviewer", "snapshot instructions"); err != nil {
		t.Fatal(err)
	}
	before, err := agentconfig.ReadSettings(d.repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	response := disconnectRequest(d, "POST", "/desktop/consoles", `{"projectId":"p","provider":"claude","view":"conversation","agentProfileId":"reviewer"}`)
	var entry desktopRun
	if response.Code != 202 || json.Unmarshal(response.Body.Bytes(), &entry) != nil {
		t.Fatalf("admission: %d %s", response.Code, response.Body.String())
	}
	run := d.queue.runs[entry.ID]
	t.Cleanup(d.stopConversations)
	if run.profile == nil || run.profile.Prompt != "snapshot instructions" || entry.AgentProfileID != "reviewer" || run.conversation.busy || entry.TaskID != "" || entry.Prompt != "" {
		t.Fatal("profile or taskless admission contract lost")
	}
	if err := os.WriteFile(filepath.Join(d.repoRoot, ".sectile", "agents", "reviewer.md"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if run.profile.Prompt != "snapshot instructions" {
		t.Fatal("active session reread the file")
	}
	after, err := agentconfig.ReadSettings(d.repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("profile selection changed workstation/project defaults")
	}
}
