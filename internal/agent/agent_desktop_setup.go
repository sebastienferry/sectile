package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"tasks/internal/agentconfig"
)

type setupMCPView struct {
	Target      string `json:"target"`
	Transport   string `json:"transport"`
	Present     bool   `json:"present"`
	Valid       bool   `json:"valid"`
	NeedsRepair bool   `json:"needsRepair"`
}
type setupStep struct {
	Scope         string        `json:"scope"`
	Root          string        `json:"root,omitempty"`
	Framework     string        `json:"framework,omitempty"`
	Configuration *setupMCPView `json:"configuration,omitempty"`
	Status        string        `json:"status"`
	Fingerprint   string        `json:"fingerprint"`
	ProjectID     string        `json:"projectId,omitempty"`
	Message       string        `json:"message,omitempty"`
}
type setupView struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Server        string               `json:"server"`
	Provider      string               `json:"provider"`
	Steps         map[string]setupStep `json:"steps"`
	Permissions   map[string]any       `json:"permissions"`
	Complete      bool                 `json:"complete"`
}

func setupProvider(provider string) bool {
	return provider == "claude" || provider == "codex" || provider == "agy"
}
func setupDigest(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func setupPermissions(settings agentconfig.Settings, provider string) map[string]any {
	switch provider {
	case "claude":
		projects := map[string]any{}
		for id, project := range settings.ProjectSettings {
			if project.ClaudeSandbox != nil && !project.ClaudeSandbox.IsZero() {
				projects[id] = project.ClaudeSandbox
			}
		}
		return map[string]any{"mode": conversationMode(settings.Defaults.ConversationMode), "sandbox": workstationSandboxPayload(settings), "projectSandbox": projects}
	case "codex":
		reviewer := "user"
		if settings.Defaults.CodexApprovalsReviewer == "auto_review" {
			reviewer = "auto_review"
		}
		return map[string]any{"approvalsReviewer": reviewer}
	default:
		mode := settings.Defaults.AgyConversationMode
		if !agyConversationModes[mode] {
			mode = "accept-edits"
		}
		return map[string]any{"conversationMode": mode}
	}
}
func setupAssetsValid(outcome agentconfig.SetupOutcome) bool {
	if len(outcome.Assets) == 0 {
		return false
	}
	for path, expected := range outcome.Assets {
		raw, err := os.ReadFile(path)
		if err != nil || setupDigest(raw) != expected {
			return false
		}
	}
	return true
}

// recordSetupAssets runs after the existing installer while prepareMu is held.
// A failed metadata write leaves installed files intact and the step retryable.
func (d *agentDaemon) recordSetupAssets(provider, step, projectID, root, framework string) error {
	if !setupProvider(provider) {
		return fmt.Errorf("unsupported setup provider")
	}
	assets := map[string]string{}
	if step == "skills" {
		loc, err := agentconfig.ResolveLocations(provider)
		if err != nil {
			return err
		}
		path, err := agentconfig.ManifestPath()
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		manifest := map[string]string{}
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return err
		}
		for relative := range manifest {
			if strings.HasPrefix(filepath.ToSlash(relative), filepath.ToSlash(loc.SkillDir)+"/") {
				path := filepath.Join(loc.Home, relative)
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				// The installer's digest is checked before accepting the readback.
				sum := sha256.Sum256(raw)
				if hex.EncodeToString(sum[:]) != manifest[relative] {
					return fmt.Errorf("installed skill differs from manifest")
				}
				assets[path] = setupDigest(raw)
			}
		}
	} else {
		// Observe framework-owned configuration/templates, not issue artifacts or
		// unrelated project plugins. Existing initialized projects are read-only.
		directories := []string{}
		if framework == "speckit" {
			directories = []string{".specify/scripts", ".specify/templates", ".specify/memory"}
		} else {
			for _, providerDir := range []string{".claude", ".agents", ".gemini"} {
				directories = append(directories, filepath.Join(providerDir, "commands", "opsx"))
				entries, _ := os.ReadDir(filepath.Join(root, providerDir, "skills"))
				for _, entry := range entries {
					if entry.IsDir() && strings.HasPrefix(entry.Name(), "openspec-") {
						directories = append(directories, filepath.Join(providerDir, "skills", entry.Name()))
					}
				}
			}
		}
		for _, directory := range directories {
			err := filepath.WalkDir(filepath.Join(root, directory), func(path string, entry fs.DirEntry, err error) error {
				if os.IsNotExist(err) {
					return nil
				}
				if err != nil {
					return err
				}
				if entry.Type()&os.ModeSymlink != 0 {
					return fmt.Errorf("framework asset is a symlink")
				}
				if entry.IsDir() {
					return nil
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				assets[path] = setupDigest(raw)
				return nil
			})
			if err != nil {
				return err
			}
		}
		paths := []string{"openspec/config.yaml", "openspec/project.md"}
		if framework == "speckit" {
			paths = []string{".specify/memory/constitution.md"}
		}
		for _, relative := range paths {
			path := filepath.Join(root, filepath.FromSlash(relative))
			if raw, err := os.ReadFile(path); err == nil {
				assets[path] = setupDigest(raw)
			}
		}
	}
	if len(assets) == 0 {
		return fmt.Errorf("no installed assets to verify")
	}
	outcome := agentconfig.SetupOutcome{Status: "success", ProjectID: projectID, Assets: assets}
	if step == "sdd" {
		outcome.Root, outcome.Framework = root, framework
	}
	_, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
		if settings.Setup == nil {
			settings.Setup = map[string]map[string]agentconfig.SetupOutcome{}
		}
		if settings.Setup[provider] == nil {
			settings.Setup[provider] = map[string]agentconfig.SetupOutcome{}
		}
		settings.Setup[provider][step] = outcome
		return nil
	})
	return err
}
func (d *agentDaemon) setupRead(r *http.Request, settings agentconfig.Settings, provider string) setupView {
	view := setupView{SchemaVersion: 1, Server: strings.TrimRight(d.link.serverURL, "/"), Provider: provider, Steps: map[string]setupStep{}, Permissions: setupPermissions(settings, provider)}
	permissions := setupDigest(view.Permissions)
	view.Steps["permissions"] = setupStep{Status: "pending", Fingerprint: permissions, Scope: "workstation"}
	if saved := settings.Setup[provider]["permissions"]; saved.Status == "success" && saved.Fingerprint == permissions {
		view.Steps["permissions"] = setupStep{Status: "success", Fingerprint: permissions, Scope: "workstation"}
	}
	for _, step := range []string{"skills", "sdd"} {
		saved := settings.Setup[provider][step]
		current := setupStep{Status: "pending", Fingerprint: setupDigest(saved), ProjectID: saved.ProjectID}
		if saved.Status == "skipped" {
			current.Status = "skipped"
		} else if saved.Status == "success" && setupAssetsValid(saved) {
			current.Status = "success"
			if step == "sdd" {
				config, err := d.fetchConfig(r.Context(), saved.ProjectID, "")
				if err != nil {
					current.Status = "unknown"
				} else {
					root, _, err := d.localProjectRoot(r.Context(), config)
					if err != nil || root != saved.Root || config.SpecFramework != saved.Framework {
						current.Status = "pending"
					}
				}
			}
		}
		current.Scope = "provider"
		if step == "sdd" && saved.ProjectID != "" {
			current.Scope, current.Root, current.Framework = "project", saved.Root, saved.Framework
		}
		current.Fingerprint = setupDigest([]any{saved, current.Status})
		view.Steps[step] = current
	}
	if companionMCPURL() != "" && (provider == "claude" || provider == "codex") {
		valid := companionMCPURL() == d.link.serverURL && d.link.token != ""
		status := "unknown"
		if valid {
			status = "success"
		}
		view.Steps["mcp"] = setupStep{Status: status, Scope: "session", Message: "Connexion isolée du compagnon ; les inscriptions utilisateur restent inchangées.", Configuration: &setupMCPView{Target: "companion", Transport: "stdio", Present: true, Valid: valid}, Fingerprint: setupDigest([]any{companionMCPURL(), valid})}
	} else {
		mcp := setupStep{Status: "pending", Scope: "provider"}
		entry, found, err := agentconfig.RegisteredMCPEntry(provider)
		choice := settings.MCPConnections[provider]
		target := d.link.serverURL
		key := d.currentMCPKey()
		if choice.Target == "local" {
			target, key = d.loopback.url, ""
		}
		executable, executableErr := os.Executable()
		valid := found && entry.Server == strings.TrimRight(target, "/") && entry.APIKey == key && (entry.Transport == "http" || entry.Transport == "stdio" && executableErr == nil && entry.Command == executable)
		if valid {
			matched, readErr := agentconfig.MCPEntryMatches(provider, executable, target, key, entry.Transport, choice.Target == "local")
			valid = matched
			if readErr != nil {
				err = readErr
			}
		}
		repair := false
		if provider == "claude" {
			views, readErr := d.claudeEntryViews(settings, executable)
			if readErr != nil {
				err = readErr
			}
			for _, item := range views {
				repair = repair || item.Stale
			}
		}
		if err != nil || d.link.serverURL == "" || d.currentMCPKey() == "" {
			mcp.Status = "unknown"
		} else if valid && !repair {
			mcp.Status = "success"
		}
		targetName := "remote"
		if choice.Target == "local" {
			targetName = "local"
		}
		mcp.Configuration = &setupMCPView{Target: targetName, Transport: entry.Transport, Present: found, Valid: valid && !repair, NeedsRepair: repair}
		// Fingerprint contains only safe identity/enums and comparison booleans.
		mcp.Fingerprint = setupDigest([]any{view.Server, choice.Target, entry.Transport, valid, repair})
		view.Steps["mcp"] = mcp
	}
	if d.link.serverURL == "" || d.currentMCPKey() == "" {
		for name, step := range view.Steps {
			step.Status = "unknown"
			view.Steps[name] = step
		}
	}
	view.Complete = true
	for _, step := range view.Steps {
		if step.Status != "success" && step.Status != "skipped" {
			view.Complete = false
		}
	}
	return view
}
func (d *agentDaemon) desktopSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	provider := r.URL.Query().Get("provider")
	if !setupProvider(provider) {
		http.Error(w, "Fournisseur non pris en charge", 400)
		return
	}
	if !d.prepareMu.TryLock() {
		http.Error(w, "Une préparation est en cours. Réessayez.", 409)
		return
	}
	defer d.prepareMu.Unlock()
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		http.Error(w, "Impossible de lire la configuration locale", 500)
		return
	}
	view := d.setupRead(r, settings, provider)
	if r.Method == http.MethodPost {
		var input struct {
			Step        string `json:"step"`
			Decision    string `json:"decision"`
			Fingerprint string `json:"fingerprint"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
			http.Error(w, "Décision invalide", 400)
			return
		}
		if !(input.Step == "permissions" && input.Decision == "retained" || (input.Step == "skills" || input.Step == "sdd") && input.Decision == "skipped") {
			http.Error(w, "Décision invalide", 400)
			return
		}
		current := view.Steps[input.Step]
		if input.Fingerprint == "" || input.Fingerprint != current.Fingerprint || current.Status == "unknown" {
			http.Error(w, "La configuration a changé. Actualisez avant de confirmer.", 409)
			return
		}
		_, err = agentconfig.UpdateSettings(d.localSettingsRoot(), func(settings *agentconfig.Settings) error {
			if input.Step == "permissions" && setupDigest(setupPermissions(*settings, provider)) != input.Fingerprint {
				return fmt.Errorf("permissions changed")
			}
			if input.Step != "permissions" {
				saved := settings.Setup[provider][input.Step]
				if setupDigest([]any{saved, current.Status}) != input.Fingerprint {
					return fmt.Errorf("decision changed")
				}
			}
			if settings.Setup == nil {
				settings.Setup = map[string]map[string]agentconfig.SetupOutcome{}
			}
			if settings.Setup[provider] == nil {
				settings.Setup[provider] = map[string]agentconfig.SetupOutcome{}
			}
			status := "skipped"
			if input.Decision == "retained" {
				status = "success"
			}
			settings.Setup[provider][input.Step] = agentconfig.SetupOutcome{Status: status, Fingerprint: input.Fingerprint}
			return nil
		})
		if err != nil {
			http.Error(w, "Impossible de conserver la décision. Actualisez et réessayez.", 409)
			return
		}
		settings, err = agentconfig.ReadSettings(d.localSettingsRoot())
		if err != nil {
			http.Error(w, "Impossible de relire la configuration", 500)
			return
		}
		view = d.setupRead(r, settings, provider)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(view)
}
