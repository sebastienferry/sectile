package agent

import (
	"encoding/json"
	"net/http"

	"tasks/internal/agentconfig"
)

func (d *agentDaemon) workstationHarnessSettings() agentconfig.HarnessSettings {
	settings, err := d.readSettings()
	if err != nil || settings.Defaults.Harness == nil {
		return agentconfig.HarnessSettings{}
	}
	return *settings.Defaults.Harness
}

func (d *agentDaemon) workstationHarnessPtr() *agentconfig.HarnessSettings {
	h := d.workstationHarnessSettings()
	return &h
}

// desktopWorkstationHarness reads and writes the workstation-wide harness
// permissions translated across Claude Code, Codex, and Antigravity.
func (d *agentDaemon) desktopWorkstationHarness(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeSandboxJSON(w, d.workstationHarnessSettings())
	case http.MethodPut:
		var input agentconfig.HarnessSettings
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
			http.Error(w, "Invalid harness settings", 400)
			return
		}
		normalized, err := agentconfig.NormalizeHarnessSettings(input)
		if err != nil {
			http.Error(w, "Invalid harness settings: "+err.Error(), 400)
			return
		}
		d.prepareMu.Lock()
		_, err = d.updateSettings(func(settings *agentconfig.Settings) error {
			settings.Defaults.Harness = nil
			if normalized.EffectivePermissionMode() != "" || normalized.NetworkAccess || len(normalized.WritablePaths) > 0 {
				settings.Defaults.Harness = &normalized
			}
			return nil
		})
		d.prepareMu.Unlock()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeSandboxJSON(w, normalized)
	default:
		http.Error(w, "Method not allowed", 405)
	}
}
