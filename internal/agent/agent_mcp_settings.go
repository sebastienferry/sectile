package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"tasks/internal/agentconfig"
	"time"
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
	executable, err := os.Executable()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if r.Method == http.MethodPost {
		// Pre-filled with the current choice, so a client-supplied Written is ignored.
		body := struct {
			agentconfig.MCPConnection
			Repair bool `json:"repair"`
		}{MCPConnection: choice}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
			http.Error(w, "Invalid MCP configuration", 400)
			return
		}
		choice = body.MCPConnection
		choice.Written = ""
		if (choice.Target != "local" && choice.Target != "remote") || (choice.Transport != "http" && choice.Transport != "stdio") {
			http.Error(w, "Invalid MCP connection mode", 400)
			return
		}
		if body.Repair && provider != "claude" {
			http.Error(w, "Repair applies to Claude Code only", 400)
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
		// The newest key this workstation holds, so a repair never writes back the older one a daemon restarted with.
		key := d.currentMCPKey()
		if _, err := agentconfig.ConfigureMCP(provider, executable, server, key, choice.Transport, choice.Target == "local"); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		choice.Written = mcpFingerprint(server, key, executable)
		_, err = agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
			if settings.MCPConnections == nil {
				settings.MCPConnections = map[string]agentconfig.MCPConnection{}
			}
			settings.MCPConnections[provider] = choice
			return nil
		})
		if err != nil {
			http.Error(w, "Provider configuration updated, but preference could not be saved: "+err.Error(), 500)
			return
		}
		if settings, err = agentconfig.ReadSettings(d.localSettingsRoot()); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if body.Repair {
			views, err := d.claudeEntryViews(settings, executable)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			var projects []string
			for _, view := range views {
				if view.Scope == "project" && view.Stale {
					projects = append(projects, view.Project)
				}
			}
			// R1 (#716): stale project entries are removed so the managed user entry applies.
			if _, err := agentconfig.RemoveClaudeProjectEntries(projects); err != nil {
				http.Error(w, "User registration repaired, but outdated project entries could not be removed: "+err.Error(), 500)
				return
			}
		}
	}
	response := map[string]any{"choice": choice, "path": filepath.Join(loc.Home, loc.MCPFile), "server": d.link.serverURL, "localURL": d.loopback.url}
	if provider == "claude" {
		views, err := d.claudeEntryViews(settings, executable)
		if err != nil {
			// A malformed ~/.claude.json must not hide the whole section. A fixed message, as a JSON syntax
			// error may quote a character of the file.
			views = []mcpEntryView{}
			response["entriesError"] = "Claude Code's ~/.claude.json could not be read"
		}
		needsRepair := false
		for _, view := range views {
			needsRepair = needsRepair || view.Stale
		}
		response["entries"], response["needsRepair"] = views, needsRepair
	}
	w.Header().Set("Content-Type", "application/json")
	// Never return the pairing key to the renderer in a configuration preview.
	_ = json.NewEncoder(w).Encode(response)
}

// mcpEntryView describes one Claude Code "sectile" registration to the desktop
// with booleans and enums only: no key material ever leaves the agent.
type mcpEntryView struct {
	Scope      string `json:"scope"`
	Project    string `json:"project,omitempty"`
	Managed    bool   `json:"managed"`
	KeyMatches bool   `json:"keyMatches"`
	URLMatches bool   `json:"urlMatches"`
	Stale      bool   `json:"stale"`
}

// claudeEntryViews compares every "sectile" entry in ~/.claude.json with what
// this agent would write. An entry is stale when its content differs from that,
// whether the agent saved it or not, so a managed entry replaced by hand is
// caught too. An unmanaged entry is still only reported, never adopted (ADR
// 0023): it is rewritten when the user asks for a repair.
func (d *agentDaemon) claudeEntryViews(settings agentconfig.Settings, executable string) ([]mcpEntryView, error) {
	entries, err := agentconfig.ReadClaudeEntries()
	if err != nil {
		return nil, err
	}
	choice, saved := settings.MCPConnections["claude"]
	local := saved && choice.Target == "local"
	expected := d.link.serverURL
	if local {
		expected = d.loopback.url
	}
	key := d.currentMCPKey()
	views := make([]mcpEntryView, 0, len(entries))
	for _, entry := range entries {
		view := mcpEntryView{Scope: entry.Scope, Project: entry.Project}
		// A local choice writes no key by design, so "the key we would write" is none.
		if local {
			view.KeyMatches = entry.Keyless()
		} else {
			view.KeyMatches = entry.KeyMatches(key)
		}
		view.URLMatches = entry.URL == "" || entry.URL == agentconfig.MCPURL(expected)
		if entry.Scope == "user" {
			view.Managed = saved && choice.Written == mcpFingerprint(expected, key, executable)
		}
		view.Stale = !(view.KeyMatches && view.URLMatches)
		views = append(views, view)
	}
	return views, nil
}

// registerMCP writes the provider's Sectile entry for init, Initialize and sync_config. A saved choice wins; without
// one the provider default is written and, for Claude, recorded as the managed choice so the next agent start rewrites
// it after a new pairing (#716).
func (d *agentDaemon) registerMCP(provider, executable string) (string, error) {
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		return "", err
	}
	choice, selected := settings.MCPConnections[provider]
	key := d.currentMCPKey()
	if !selected && provider != "claude" {
		return agentconfig.BootstrapMCP(provider, executable, d.link.serverURL, key)
	}
	if !selected {
		choice = agentconfig.MCPConnection{Target: "remote", Transport: "http"}
	}
	server, path := d.link.serverURL, ""
	if choice.Target == "local" && d.loopback.url == "" {
		// CLI init has no loopback: leave the entry and clear Written so the next agent start rewrites it.
		choice.Written = ""
		loc, err := agentconfig.ResolveLocations(provider)
		if err != nil {
			return "", err
		}
		path = filepath.Join(loc.Home, loc.MCPFile)
	} else {
		if choice.Target == "local" {
			server = d.loopback.url
		}
		if path, err = agentconfig.ConfigureMCP(provider, executable, server, key, choice.Transport, choice.Target == "local"); err != nil {
			return "", err
		}
		choice.Written = mcpFingerprint(server, key, executable)
	}
	_, err = agentconfig.UpdateSettings(d.localSettingsRoot(), func(s *agentconfig.Settings) error {
		if s.MCPConnections == nil {
			s.MCPConnections = map[string]agentconfig.MCPConnection{}
		}
		s.MCPConnections[provider] = choice
		return nil
	})
	return path, err
}

// refreshMCPConnections keeps the MCP connections the user saved from the
// desktop working after a restart. A local choice points at the loopback,
// whose port changes when 8091 is taken, and every choice carries the key and,
// over stdio, the executable: an entry is rewritten only when one of them
// changed since it was written, never merely because the agent started (#267).
func (d *agentDaemon) refreshMCPConnections() error {
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	// A daemon restarted with an older key (TOKEN survives the self-restart) must not write it over the newer one that
	// `sectile-agent pair` or the desktop stored and already put in the registrations (ADR 0049): the old one may be revoked.
	_, newerKeyStored := d.newerStoredKey()
	key := d.currentMCPKey()
	rewritten := map[string]string{}
	var errs []error
	for provider, choice := range settings.MCPConnections {
		// A local choice never carries the key, so it still follows the loopback.
		if newerKeyStored && choice.Target != "local" {
			continue
		}
		server := d.link.serverURL
		if choice.Target == "local" {
			server = d.loopback.url
		}
		fingerprint := mcpFingerprint(server, key, executable)
		if choice.Written == fingerprint {
			continue
		}
		// One broken provider must not keep the others on a stale key (#716).
		path, err := agentconfig.ConfigureMCP(provider, executable, server, key, choice.Transport, choice.Target == "local")
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", provider, err))
			continue
		}
		// The path only: neither the key nor its fingerprint belongs in the log.
		log.Printf("[Agent] MCP registration for %s rewritten after a server, key or executable change: %s", provider, path)
		rewritten[provider] = fingerprint
	}
	// A Codex or Antigravity registration written by `sectile-agent init` or with an earlier key, with no saved desktop
	// choice, still carries the key it was written with: it follows the key the agent now holds (#717). It is never
	// created. An unsaved Claude Code entry is only reported, and rewritten on Repair (ADR 0023).
	if !newerKeyStored && !(temporaryExecutable(executable) && !runningUnderTest()) {
		for _, provider := range unsavedMCPProviders(settings) {
			if _, err := agentconfig.RefreshRegisteredMCPKey(provider, executable, d.link.serverURL, d.link.token); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", provider, err))
			}
		}
	}
	if len(rewritten) == 0 {
		return errors.Join(errs...)
	}
	_, err = agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
		for provider, fingerprint := range rewritten {
			if choice, ok := settings.MCPConnections[provider]; ok {
				choice.Written = fingerprint
				settings.MCPConnections[provider] = choice
			}
		}
		return nil
	})
	return errors.Join(append(errs, err)...)
}

// unsavedMCPProviders are the providers whose existing registration follows a new key without a saved desktop choice:
// Codex and Antigravity. Claude Code is left out, as its unmanaged entry is only reported (ADR 0023).
func unsavedMCPProviders(settings agentconfig.Settings) []string {
	var providers []string
	for _, provider := range agentconfig.MCPProviders {
		if _, saved := settings.MCPConnections[provider]; !saved && provider != "claude" {
			providers = append(providers, provider)
		}
	}
	return providers
}

// newerStoredKey returns the key `sectile-agent pair` or the desktop stored for this server when it differs from the
// one the daemon started with: a daemon restarted with an older key keeps TOKEN across the self-restart, and that key
// may be revoked (ADR 0049). A stored connection with no server is the same server, as in resolveCredential.
//
// A daemon Sectile Desktop started knows when its key was paired: a stored key
// is newer only when it was paired later, so a key Desktop left in the file
// before it kept its own is never taken for a newer one (#746).
func (d *agentDaemon) newerStoredKey() (string, bool) {
	stored, _ := agentconfig.ReadConnection()
	if stored.APIKey == "" || stored.APIKey == d.link.token || (stored.Server != "" && stored.Server != strings.TrimRight(d.link.serverURL, "/")) {
		return "", false
	}
	if !d.link.pairedAt.IsZero() && !stored.NewerThan(d.link.pairedAt) {
		return "", false
	}
	return stored.APIKey, true
}

// desktopPairedAt reads the pairing moment Sectile Desktop passes with the
// key; empty or unreadable is zero, the rule of an agent started by hand.
func desktopPairedAt(value string) time.Time {
	pairedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return pairedAt
}

// currentMCPKey is the key a registration is written with and compared to: the newer stored one when there is one,
// else the daemon's own, so a report or a repair never pushes an older key back.
func (d *agentDaemon) currentMCPKey() string {
	if key, newer := d.newerStoredKey(); newer {
		return key
	}
	return d.link.token
}

// mcpFingerprint identifies what a registration was written with, without
// keeping the key: a truncated SHA-256 of the three values.
func mcpFingerprint(server, token, executable string) string {
	sum := sha256.Sum256([]byte(server + "\x00" + token + "\x00" + executable))
	return hex.EncodeToString(sum[:8])
}
