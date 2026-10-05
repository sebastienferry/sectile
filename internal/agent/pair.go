package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"tasks/internal/agentconfig"
)

// Pair runs `sectile-agent pair`: it spends a pairing code from the web
// profile, or the one the browser sign-in hands back, once, receives the
// workstation's API key and stores it beside the local settings, so the daemon
// then starts without any environment variable. The key this workstation held
// for the same server is revoked, and existing MCP registrations follow the
// new one. It returns the message to print on success.
func Pair(args []string) (string, error) {
	fs := flag.NewFlagSet("pair", flag.ContinueOnError)
	serverURL := fs.String("url", "", "Sectile server URL (e.g. https://sectile.example.com)")
	code := fs.String("code", "", "Pairing code from the web profile (optional: without it, sign in through the browser)")
	noBrowser := fs.Bool("no-browser", false, "Print the sign-in URL instead of opening the browser")
	label := fs.String("label", "", "Name shown for this workstation in the profile (defaults to hostname)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	resolvedURL := resolveServerURL(*serverURL)
	if resolvedURL == "" {
		return "", fmt.Errorf("--url is required (or set REMOTE_URL)")
	}
	if *label == "" {
		*label, _ = os.Hostname()
	}
	stored, _ := agentconfig.ReadConnection() // DeviceID survives ErrNoStoredConnection
	replaces := ""
	if stored.Server == strings.TrimRight(strings.TrimSpace(resolvedURL), "/") {
		replaces = stored.DeviceID // a device id means nothing to another server
	}
	pairingCode := strings.TrimSpace(*code)
	if pairingCode == "" {
		opener := openBrowser
		if *noBrowser {
			opener = nil
		}
		var err error
		if pairingCode, err = browserSignInCode(context.Background(), resolvedURL, opener, os.Stderr); err != nil {
			return "", err
		}
	}
	connection, err := exchangePairingCode(context.Background(), resolvedURL, pairingCode, *label, replaces)
	if err != nil {
		return "", err
	}
	if err := agentconfig.WriteConnection(connection); err != nil {
		return "", fmt.Errorf("store the API key: %w", err)
	}
	refreshed, warnings := refreshMCPAfterPair(connection)
	path, _ := agentconfig.SettingsPath()
	message := fmt.Sprintf("Paired with %s as workstation %s. The API key is stored in %s; start the agent with:\n  sectile-agent --url %s",
		connection.Server, connection.DeviceID, path, connection.Server)
	if len(refreshed) > 0 {
		message += "\nMCP configuration updated for: " + strings.Join(refreshed, ", ")
	}
	for _, warning := range warnings {
		message += "\nWarning: " + warning
	}
	if replaces != "" {
		message += "\nThe previous key of this workstation is revoked: restart a running agent so it uses the new one."
	}
	return message, nil
}

// refreshMCPAfterPair points the registrations for this server at the new key: the one they held is revoked now. A
// saved desktop choice, the managed Claude Code one included, is rewritten as the next agent start would rewrite it
// (#716); an existing Codex or Antigravity entry with no saved choice follows the key too (#717). An unmanaged Claude
// Code entry is left for Desktop to report and repair (ADR 0023), and a local choice carries no key. It returns the
// providers rewritten and, as warnings, those it could not rewrite: the key is already stored, so nothing here fails
// the pairing.
func refreshMCPAfterPair(connection agentconfig.Connection) ([]string, []string) {
	executable, err := os.Executable()
	if err != nil {
		return nil, []string{fmt.Sprintf("MCP configuration not refreshed: %v", err)}
	}
	if temporaryExecutable(executable) && !runningUnderTest() {
		return nil, nil
	}
	root, _ := os.Getwd()
	root = findRepoRoot(root)
	settings, err := agentconfig.ReadSettings(root)
	if err != nil {
		return nil, []string{fmt.Sprintf("MCP configuration not refreshed: %v", err)}
	}
	var refreshed, warnings []string
	rewritten := map[string]string{}
	for provider, choice := range settings.MCPConnections {
		if choice.Target == "local" {
			continue
		}
		if _, err := agentconfig.ConfigureMCP(provider, executable, connection.Server, connection.APIKey, choice.Transport, false); err != nil {
			warnings = append(warnings, fmt.Sprintf("MCP configuration for %s not refreshed: %v", provider, err))
			continue
		}
		refreshed = append(refreshed, provider)
		rewritten[provider] = mcpFingerprint(connection.Server, connection.APIKey, executable)
	}
	for _, provider := range unsavedMCPProviders(settings) {
		wrote, err := agentconfig.RefreshRegisteredMCPKey(provider, executable, connection.Server, connection.APIKey)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("MCP configuration for %s not refreshed: %v", provider, err))
		} else if wrote {
			refreshed = append(refreshed, provider)
		}
	}
	if len(rewritten) > 0 {
		// Recorded as written, so Desktop counts the Claude Code entry as managed and the next start leaves it alone.
		_, err = agentconfig.UpdateSettings(root, func(settings *agentconfig.Settings) error {
			for provider, fingerprint := range rewritten {
				if choice, ok := settings.MCPConnections[provider]; ok {
					choice.Written = fingerprint
					settings.MCPConnections[provider] = choice
				}
			}
			return nil
		})
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("MCP choices not recorded: %v", err))
		}
	}
	sort.Strings(refreshed)
	sort.Strings(warnings)
	return refreshed, warnings
}

// exchangePairingCode is the one call made without a credential: the code is
// the proof, and the server spends it. replaces names the device whose key the
// server revokes in the same step; empty, nothing is revoked.
func exchangePairingCode(ctx context.Context, server, code, label, replaces string) (agentconfig.Connection, error) {
	endpoint, err := url.Parse(server)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" || endpoint.User != nil {
		return agentconfig.Connection{}, fmt.Errorf("use an HTTP or HTTPS server URL without credentials")
	}
	server = strings.TrimRight(server, "/")
	fields := map[string]string{"code": strings.TrimSpace(code), "label": label}
	if replaces != "" {
		fields["deviceId"] = replaces
	}
	body, _ := json.Marshal(fields)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server+"/api/v1/agent/pair", bytes.NewReader(body))
	if err != nil {
		return agentconfig.Connection{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return agentconfig.Connection{}, fmt.Errorf("reach %s: %w", server, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode == http.StatusUnauthorized {
		return agentconfig.Connection{}, fmt.Errorf("invalid or expired pairing code: generate a new one from the web profile")
	}
	if resp.StatusCode != http.StatusCreated {
		return agentconfig.Connection{}, fmt.Errorf("the server refused the pairing request (HTTP %d)", resp.StatusCode)
	}
	var payload struct {
		Token    string `json:"token"`
		DeviceID string `json:"deviceId"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Token == "" {
		return agentconfig.Connection{}, fmt.Errorf("%s does not expose the Sectile agent API", server)
	}
	return agentconfig.Connection{Server: server, APIKey: payload.Token, DeviceID: payload.DeviceID}, nil
}

// resolveCredential picks the API key the daemon presents: the flag, then the
// environment, then the key stored by `sectile-agent pair` when it was issued
// by the same server. The stored key is not lent to another server: it would
// fail there anyway, and the error would point at the wrong thing.
func resolveCredential(flagToken, envToken, server string, stored agentconfig.Connection) string {
	if flagToken != "" {
		return flagToken
	}
	if envToken != "" {
		return envToken
	}
	if stored.APIKey != "" && (stored.Server == "" || stored.Server == strings.TrimRight(server, "/")) {
		return stored.APIKey
	}
	return ""
}
