package agent

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"tasks/internal/terminal"
)

// EmbeddedConfig is supplied by the Local composition root. It intentionally
// has no pairing, environment, or home-directory fallback.
type EmbeddedConfig struct {
	ServerURL    string
	Token        string
	ProjectID    string
	DeviceID     string
	DesktopInfo  string
	DesktopToken string
	SettingsRoot string
	RepoRoot     string
	TerminalApp  string
	PairedAt     time.Time
	OnRestart    func()
}

// RunEmbedded runs the existing workstation gateway and execution engine in
// the caller's process. Cancellation stops its connection and flushes its run
// store. Restart requests are handed to OnRestart; this function never execs
// a second process or installs signal handlers.
func RunEmbedded(parent context.Context, cfg EmbeddedConfig) error {
	if parent == nil {
		return errors.New("embedded agent requires a context")
	}
	endpoint, err := url.Parse(cfg.ServerURL)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return errors.New("embedded agent requires an explicit HTTP backend URL")
	}
	if cfg.Token == "" || cfg.DeviceID == "" || cfg.DesktopToken == "" || !filepath.IsAbs(cfg.DesktopInfo) || !filepath.IsAbs(cfg.SettingsRoot) {
		return errors.New("embedded agent requires explicit credentials, device, discovery path and settings root")
	}
	if cfg.RepoRoot != "" && !filepath.IsAbs(cfg.RepoRoot) {
		return errors.New("embedded agent repository root must be absolute")
	}
	if localAgentAvailable(cfg.DesktopInfo) {
		return fmt.Errorf("an agent already owns desktop discovery at %s", cfg.DesktopInfo)
	}
	projectID := cfg.ProjectID
	if projectID == "" {
		projectID = "all"
	}
	terminalApp := strings.TrimSpace(cfg.TerminalApp)
	if terminalApp == "" {
		terminalApp = detectDefaultTerminal()
	}
	d := &agentDaemon{
		loopback: loopbackServer{desktopInfo: cfg.DesktopInfo, desktopToken: cfg.DesktopToken},
		link: serverLink{
			serverURL: strings.TrimRight(cfg.ServerURL, "/"), token: cfg.Token,
			pairedAt: cfg.PairedAt, projectID: projectID, deviceID: cfg.DeviceID,
		},
		terminal: terminalChoice{app: terminalApp, explicit: cfg.TerminalApp != "", manager: terminal.NewManager()},
		repoRoot: cfg.RepoRoot, settingsRoot: cfg.SettingsRoot,
		disableMCPRefresh: true,
		done:              make(chan struct{}),
	}
	d.store = openRunStore(filepath.Join(filepath.Dir(cfg.DesktopInfo), "runs"))
	d.restoreRuns()
	d.pruneRuns()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	if cfg.OnRestart != nil {
		d.restartAgent = cancel
	}
	err = d.runDaemon(ctx)
	d.queue.mu.Lock()
	restart := d.queue.restartRequested
	d.queue.mu.Unlock()
	if restart && cfg.OnRestart != nil {
		cfg.OnRestart()
	}
	return err
}
