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
	"strings"
	"time"

	"tasks/internal/agentconfig"
)

// Pair runs `sectile-agent pair`: it spends a pairing code from the web
// profile once, receives the workstation's API key and stores it beside the
// local settings, so the daemon then starts without any environment variable.
// It returns the message to print on success.
func Pair(args []string) (string, error) {
	fs := flag.NewFlagSet("pair", flag.ContinueOnError)
	serverURL := fs.String("url", "", "Sectile server URL (e.g. https://sectile.example.com)")
	code := fs.String("code", "", "Pairing code generated from the web profile")
	label := fs.String("label", "", "Name shown for this workstation in the profile (defaults to hostname)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	resolvedURL := resolveServerURL(*serverURL)
	if resolvedURL == "" {
		return "", fmt.Errorf("--url is required (or set REMOTE_URL)")
	}
	if strings.TrimSpace(*code) == "" {
		return "", fmt.Errorf("--code is required: generate a pairing code from the web profile")
	}
	if *label == "" {
		*label, _ = os.Hostname()
	}
	connection, err := exchangePairingCode(context.Background(), resolvedURL, *code, *label)
	if err != nil {
		return "", err
	}
	if err := agentconfig.WriteConnection(connection); err != nil {
		return "", fmt.Errorf("store the API key: %w", err)
	}
	path, _ := agentconfig.SettingsPath()
	return fmt.Sprintf("Paired with %s as workstation %s. The API key is stored in %s; start the agent with:\n  sectile-agent --url %s",
		connection.Server, connection.DeviceID, path, connection.Server), nil
}

// exchangePairingCode is the one call made without a credential: the code is
// the proof, and the server spends it.
func exchangePairingCode(ctx context.Context, server, code, label string) (agentconfig.Connection, error) {
	endpoint, err := url.Parse(server)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" || endpoint.User != nil {
		return agentconfig.Connection{}, fmt.Errorf("use an HTTP or HTTPS server URL without credentials")
	}
	server = strings.TrimRight(server, "/")
	body, _ := json.Marshal(map[string]string{"code": strings.TrimSpace(code), "label": label})
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
