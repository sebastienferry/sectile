package agent

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/testhome"
)

func TestWorkstationHarnessEndpointAndTranslation(t *testing.T) {
	testhome.Temp(t)
	d, id, log := codexConversationFixture(t)

	if rec := disconnectRequest(d, "PUT", "/desktop/workstation/harness", `{"networkAccess":true,"writablePaths":["~/go/pkg/mod","  ","~/go/pkg/mod"]}`); rec.Code != 400 {
		t.Fatalf("empty entry should fail with 400, got %d %s", rec.Code, rec.Body.String())
	}

	extraDir := filepath.Join(t.TempDir(), "cargo-cache")
	body, _ := json.Marshal(map[string]any{
		"autoApprove":   true,
		"networkAccess": true,
		"writablePaths": []string{extraDir, "~/.pinchtab"},
	})
	if rec := disconnectRequest(d, "PUT", "/desktop/workstation/harness", string(body)); rec.Code != 200 {
		t.Fatalf("PUT /desktop/workstation/harness failed: %d %s", rec.Code, rec.Body.String())
	}

	got := d.workstationHarnessSettings()
	if !got.AutoApprove || !got.NetworkAccess || len(got.WritablePaths) != 2 {
		t.Fatalf("unexpected stored harness settings: %+v", got)
	}
	if mode := d.workstationConversationMode(); mode != "auto" {
		t.Fatalf("Claude workstationConversationMode = %q, want auto", mode)
	}
	if rev := d.workstationCodexReviewer(); rev != "auto_review" {
		t.Fatalf("Codex workstationCodexReviewer = %q, want auto_review", rev)
	}
	if agyMode := d.workstationAgyConversationMode(); agyMode != "auto" {
		t.Fatalf("Antigravity workstationAgyConversationMode = %q, want auto", agyMode)
	}

	// Verify translation to Codex turn/start sandboxPolicy.
	codexMessage(t, d, id, `{"message":"run tests","mode":"workspace-write"}`)
	waitCodex(t, d, id, func(c *providerConversation) bool { return !c.busy })
	text := codexLog(t, log)
	if !strings.Contains(text, `"networkAccess":true`) {
		t.Fatalf("Codex sandboxPolicy did not enable networkAccess from harness: %s", text)
	}
	if !strings.Contains(text, extraDir) {
		t.Fatalf("Codex sandboxPolicy missing harness writablePath %s: %s", extraDir, text)
	}

	// Verify translation to Claude ResolvedClaudeSandbox.
	settings := agentconfig.Settings{
		Defaults: agentconfig.Defaults{
			Harness: &agentconfig.HarnessSettings{
				AutoApprove:   true,
				NetworkAccess: true,
				WritablePaths: []string{extraDir},
			},
		},
	}
	resolved := settings.ResolvedClaudeSandbox("project-1")
	if resolved == nil || len(resolved.AllowWrite) == 0 || resolved.AllowWrite[0] != extraDir {
		t.Fatalf("ResolvedClaudeSandbox did not inherit harness writablePaths: %+v", resolved)
	}
	if resolved.AutoAllowBashIfSandboxed == nil || !*resolved.AutoAllowBashIfSandboxed {
		t.Fatalf("ResolvedClaudeSandbox did not enable AutoAllowBashIfSandboxed: %+v", resolved)
	}
	if resolved.AllowLocalBinding == nil || !*resolved.AllowLocalBinding {
		t.Fatalf("ResolvedClaudeSandbox did not enable AllowLocalBinding: %+v", resolved)
	}

	// Verify headless autonomous Codex command line carries network_access and writablePaths.
	headlessCmd, err := modeCommandLine("codex", "", "gpt-5.4", "run pickup", models.SkillModeAutonomous, agentCommandContext{
		Harness: &agentconfig.HarnessSettings{NetworkAccess: true, WritablePaths: []string{extraDir}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(headlessCmd, "-c sandbox_workspace_write.network_access=true") {
		t.Fatalf("headless Codex command missing network_access override: %s", headlessCmd)
	}
	if !strings.Contains(headlessCmd, "--add-dir=") || !strings.Contains(headlessCmd, extraDir) {
		t.Fatalf("headless Codex command missing harness writablePath: %s", headlessCmd)
	}

	// Verify headless autonomous Codex bypasses sandbox when Harness permissionMode is "auto".
	fullCmd, err := modeCommandLine("codex", "", "gpt-5.4", "run pickup", models.SkillModeAutonomous, agentCommandContext{
		Harness: &agentconfig.HarnessSettings{PermissionMode: "auto", NetworkAccess: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fullCmd, "--dangerously-bypass-approvals-and-sandbox") {
		t.Fatalf("headless Codex command with auto permissionMode missing bypass flag: %s", fullCmd)
	}
	if strings.Contains(fullCmd, "--approve-for-me") {
		t.Fatalf("headless Codex command with auto permissionMode should not pass --approve-for-me: %s", fullCmd)
	}
}
