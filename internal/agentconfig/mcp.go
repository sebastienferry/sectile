package agentconfig

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/pelletier/go-toml/v2"
)

// httpMCPProviders are the CLIs whose configuration can express a Streamable
// HTTP MCP server with a bearer header. They are pointed at the server
// directly, so their Sectile tools keep working while the local agent is
// stopped. Every other provider gets the stdio bridge against the same server.
var httpMCPProviders = map[string]bool{"claude": true, "cursor": true, "gemini": true}

// UsesHTTPMCP reports whether the provider is registered against the server's
// /mcp directly rather than through the stdio bridge.
func UsesHTTPMCP(provider string) bool {
	return httpMCPProviders[strings.ToLower(strings.TrimSpace(provider))]
}

// mcpEntry is the transport half of the Sectile registration for one provider.
// The workstation API key travels in it: the file is user-level and owner-only,
// and the key is revocable and expires, which is the trade-off ADR 0011 makes.
func mcpEntry(provider, executable, server, apiKey string) map[string]any {
	endpoint := server + "/mcp"
	headers := map[string]any{"Authorization": "Bearer " + apiKey}
	switch provider {
	case "claude":
		return map[string]any{"type": "http", "url": endpoint, "headers": headers}
	case "cursor":
		return map[string]any{"url": endpoint, "headers": headers}
	case "gemini":
		return map[string]any{"httpUrl": endpoint, "headers": headers}
	}
	return map[string]any{
		"command": executable,
		"args":    []string{"mcp", "--url", server},
		"env":     map[string]any{"SECTILE_AGENT_TOKEN": apiKey},
	}
}

// BootstrapMCP registers Sectile in the selected CLI's user-level
// configuration. Nothing is written inside the repository: a checkout must
// stay free of agent configuration. The reserved Sectile entry migrates to
// Sectile and native tool permissions remain in force. The registration
// addresses the server, never a local gateway, so it is independent of any
// agent process and survives its restarts.
func BootstrapMCP(provider, executable, server, apiKey string) (string, error) {
	if !filepath.IsAbs(executable) {
		return "", fmt.Errorf("MCP executable must be an absolute path")
	}
	endpoint, err := url.Parse(server)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" || endpoint.User != nil {
		return "", fmt.Errorf("MCP bootstrap requires the Sectile server URL")
	}
	if strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("MCP bootstrap requires the workstation API key")
	}
	server = strings.TrimRight(server, "/")
	provider = strings.ToLower(strings.TrimSpace(provider))
	loc, err := ResolveLocations(provider)
	if err != nil {
		return "", err
	}
	root, path := loc.Home, loc.MCPFile
	fs, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	defer fs.Close()
	data := map[string]any{}
	raw, err := fs.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	isTOML := strings.HasSuffix(path, ".toml")
	if len(raw) > 0 {
		if isTOML {
			err = toml.Unmarshal(raw, &data)
		} else {
			err = json.Unmarshal(raw, &data)
		}
		if err != nil {
			return "", fmt.Errorf("read existing MCP configuration %s: %w", path, err)
		}
	}
	if data == nil {
		return "", fmt.Errorf("MCP configuration %s must be an object", path)
	}
	if err := migrateMCPRegistration(data, provider, mcpEntry(provider, executable, server, apiKey)); err != nil {
		return "", fmt.Errorf("migrate MCP configuration %s: %w", path, err)
	}
	if err := checkExternalMCPPolicies(loc.Home, provider, filepath.Join(loc.Home, path)); err != nil {
		return "", err
	}
	if isTOML {
		raw, err = toml.Marshal(data)
	} else {
		raw, err = json.MarshalIndent(data, "", "  ")
	}
	if err != nil {
		return "", err
	}
	if err = fs.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	temp := filepath.Join(filepath.Dir(path), ".sectile-mcp-"+uuid.NewString()+".tmp")
	defer fs.Remove(temp)
	// Owner-only: the file now carries the workstation API key.
	if err = fs.WriteFile(temp, raw, 0600); err != nil {
		return "", err
	}
	if err = fs.Rename(temp, path); err != nil {
		return "", err
	}
	return filepath.Join(root, path), nil
}
