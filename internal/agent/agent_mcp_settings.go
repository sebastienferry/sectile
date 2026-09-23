package agent

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"tasks/internal/agentconfig"
)

// Only an explicit workstation preference opens MCP without a client key.
// The gateway still rejects browser origins and unexpected Host headers.
func (d *agentDaemon) localMCPEnabled() bool {
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		return false
	}
	for _, choice := range settings.MCPConnections {
		if choice.Target == "local" {
			return true
		}
	}
	return false
}

func (d *agentDaemon) desktopMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !d.prepareMu.TryLock() {
		http.Error(w, "Project preparation or deployment is in progress", 409)
		return
	}
	defer d.prepareMu.Unlock()
	provider := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
	loc, err := agentconfig.ResolveLocations(provider)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	choice, selected := settings.MCPConnections[provider]
	if !selected {
		choice = agentconfig.MCPConnection{Target: "remote", Transport: "http"}
	}
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&choice); err != nil {
			http.Error(w, "Invalid MCP configuration", 400)
			return
		}
		if (choice.Target != "local" && choice.Target != "remote") || (choice.Transport != "http" && choice.Transport != "stdio") {
			http.Error(w, "Invalid MCP connection mode", 400)
			return
		}
		executable, err := os.Executable()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if temporaryExecutable(executable) && !runningUnderTest() {
			http.Error(w, "Run an installed agent before updating provider configuration", 400)
			return
		}
		server := d.link.serverURL
		if choice.Target == "local" {
			server = d.loopback.url
		}
		if _, err := agentconfig.ConfigureMCP(provider, executable, server, d.link.token, choice.Transport, choice.Target == "local"); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if settings.MCPConnections == nil {
			settings.MCPConnections = map[string]agentconfig.MCPConnection{}
		}
		settings.MCPConnections[provider] = choice
		if err := agentconfig.WriteSettings(settings); err != nil {
			http.Error(w, "Provider configuration updated, but preference could not be saved: "+err.Error(), 500)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	// Never return the pairing key to the renderer in a configuration preview.
	_ = json.NewEncoder(w).Encode(map[string]any{"choice": choice, "path": filepath.Join(loc.Home, loc.MCPFile), "server": d.link.serverURL, "localURL": d.loopback.url})
}

// Refresh saved choices after a restart, including a changed loopback port.
func (d *agentDaemon) refreshMCPConnections() error {
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	for provider, choice := range settings.MCPConnections {
		server := d.link.serverURL
		if choice.Target == "local" {
			server = d.loopback.url
		}
		if _, err := agentconfig.ConfigureMCP(provider, executable, server, d.link.token, choice.Transport, choice.Target == "local"); err != nil {
			return err
		}
	}
	return nil
}
