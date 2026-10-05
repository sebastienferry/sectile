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
var httpMCPProviders = map[string]bool{"claude": true}

// UsesHTTPMCP reports whether the provider is registered against the server's
// /mcp directly rather than through the stdio bridge.
func UsesHTTPMCP(provider string) bool {
	return httpMCPProviders[strings.ToLower(strings.TrimSpace(provider))]
}

// mcpEntry is the transport half of the Sectile registration for one provider.
// The workstation API key travels in it: the file is user-level and owner-only,
// and the key is revocable and expires, which is the trade-off ADR 0011 makes.
func mcpEntry(provider, executable, server, apiKey string) map[string]any {
	endpoint := MCPURL(server)
	headers := map[string]any{"Authorization": "Bearer " + apiKey}
	switch provider {
	case "claude":
		return map[string]any{"type": "http", "url": endpoint, "headers": headers}
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
	return ConfigureMCP(provider, executable, server, apiKey, "auto", false)
}

// ConfigureMCP writes an explicit transport while preserving unrelated settings.
// Local mode accepts only a literal loopback URL and never writes the pairing key.
func ConfigureMCP(provider, executable, server, apiKey, transport string, local bool) (string, error) {
	if !filepath.IsAbs(executable) {
		return "", fmt.Errorf("MCP executable must be an absolute path")
	}
	endpoint, err := url.Parse(server)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return "", fmt.Errorf("MCP bootstrap requires the Sectile server URL")
	}
	if transport != "auto" && transport != "http" && transport != "stdio" {
		return "", fmt.Errorf("invalid MCP transport")
	}
	if local && transport == "auto" {
		return "", fmt.Errorf("local MCP requires an explicit transport")
	}
	if local && (endpoint.Scheme != "http" || endpoint.Hostname() != "127.0.0.1" || endpoint.Port() == "" || endpoint.Path != "") {
		return "", fmt.Errorf("local MCP requires a loopback URL with a port")
	}
	if !local && strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("MCP bootstrap requires the workstation API key")
	}
	server = strings.TrimRight(server, "/")
	provider = strings.ToLower(strings.TrimSpace(provider))
	data, loc, err := readMCPConfig(provider)
	if err != nil {
		return "", err
	}
	root, path := loc.Home, loc.MCPFile
	fs, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	defer fs.Close()
	isTOML := strings.HasSuffix(path, ".toml")
	if err := migrateMCPRegistration(data, provider, selectedMCPEntry(provider, executable, server, apiKey, transport, local)); err != nil {
		return "", fmt.Errorf("migrate MCP configuration %s: %w", path, err)
	}
	if err := checkExternalMCPPolicies(loc.Home, provider, filepath.Join(loc.Home, path)); err != nil {
		return "", err
	}
	if err := writeMCPFile(fs, path, isTOML, data); err != nil {
		return "", err
	}
	return filepath.Join(root, path), nil
}

// MCPURL is the MCP endpoint of a Sectile server. Writers and readers share it,
// so a registered URL is compared by the same rule it was written with.
func MCPURL(server string) string { return strings.TrimRight(server, "/") + "/mcp" }

// writeMCPFile replaces path under fs atomically, owner-only: the file carries the workstation key.
func writeMCPFile(fs *os.Root, path string, isTOML bool, data map[string]any) error {
	var raw []byte
	var err error
	if isTOML {
		raw, err = toml.Marshal(data)
	} else {
		raw, err = json.MarshalIndent(data, "", "  ")
	}
	if err != nil {
		return err
	}
	if err = fs.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	temp := filepath.Join(filepath.Dir(path), ".sectile-mcp-"+uuid.NewString()+".tmp")
	defer fs.Remove(temp)
	// Owner-only: the file now carries the workstation API key.
	if err = fs.WriteFile(temp, raw, 0600); err != nil {
		return err
	}
	return fs.Rename(temp, path)
}

// readMCPConfig decodes the provider's user-level MCP configuration. A
// missing or empty file is an empty object.
func readMCPConfig(provider string) (map[string]any, Locations, error) {
	loc, err := ResolveLocations(provider)
	if err != nil {
		return nil, Locations{}, err
	}
	path := loc.MCPFile
	fs, err := os.OpenRoot(loc.Home)
	if err != nil {
		return nil, Locations{}, err
	}
	defer fs.Close()
	data := map[string]any{}
	raw, err := fs.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, Locations{}, err
	}
	if len(raw) > 0 {
		if strings.HasSuffix(path, ".toml") {
			err = toml.Unmarshal(raw, &data)
		} else {
			err = json.Unmarshal(raw, &data)
		}
		if err != nil {
			return nil, Locations{}, fmt.Errorf("read existing MCP configuration %s: %w", path, err)
		}
	}
	if data == nil {
		return nil, Locations{}, fmt.Errorf("MCP configuration %s must be an object", path)
	}
	return data, loc, nil
}

// MCPProviders are the CLIs whose user-level MCP registration Sectile writes.
var MCPProviders = []string{"claude", "codex", "agy"}

// RegisteredMCP is the user-level `sectile` entry a provider already holds.
type RegisteredMCP struct {
	Server, APIKey, Transport, Command string
}

// RegisteredMCPEntry reads the top-level `sectile` entry: found is false when
// the file or the entry is missing. An entry in neither the HTTP nor the stdio
// shape is found but empty.
func RegisteredMCPEntry(provider string) (RegisteredMCP, bool, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	data, _, err := readMCPConfig(provider)
	if err != nil {
		return RegisteredMCP{}, false, err
	}
	key := "mcpServers"
	if provider == "codex" {
		key = "mcp_servers"
	}
	servers, _ := data[key].(map[string]any)
	value, found := servers["sectile"]
	if !found {
		return RegisteredMCP{}, false, nil
	}
	entry, _ := value.(map[string]any)
	if endpoint := firstString(entry, "url", "serverUrl"); endpoint != "" {
		registered := RegisteredMCP{Server: strings.TrimSuffix(strings.TrimRight(endpoint, "/"), "/mcp"), Transport: "http"}
		for _, field := range []string{"headers", "http_headers"} {
			headers, _ := entry[field].(map[string]any)
			if authorization, ok := headers["Authorization"].(string); ok {
				registered.APIKey = strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
				break
			}
		}
		return registered, true, nil
	}
	if command, ok := entry["command"].(string); ok && command != "" {
		registered := RegisteredMCP{Command: command, Transport: "stdio"}
		args, _ := entry["args"].([]any)
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "--url" {
				registered.Server, _ = args[i+1].(string)
				break
			}
		}
		env, _ := entry["env"].(map[string]any)
		registered.APIKey, _ = env["SECTILE_AGENT_TOKEN"].(string)
		return registered, true, nil
	}
	return RegisteredMCP{}, true, nil
}

func firstString(entry map[string]any, fields ...string) string {
	for _, field := range fields {
		if value, ok := entry[field].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

// RefreshRegisteredMCPKey rewrites an existing registration that addresses
// server with another key, keeping its transport (and, over stdio, its
// command). It never creates one, and leaves a local or foreign entry alone.
// It reports whether it wrote.
func RefreshRegisteredMCPKey(provider, executable, server, apiKey string) (bool, error) {
	entry, found, err := RegisteredMCPEntry(provider)
	server = strings.TrimRight(server, "/")
	if err != nil || !found || entry.APIKey == "" || strings.TrimSpace(apiKey) == "" || entry.APIKey == apiKey || strings.TrimRight(entry.Server, "/") != server {
		return false, err
	}
	if entry.Transport == "stdio" && filepath.IsAbs(entry.Command) {
		executable = entry.Command
	}
	_, err = ConfigureMCP(provider, executable, server, apiKey, entry.Transport, false)
	return err == nil, err
}

func selectedMCPEntry(provider, executable, server, apiKey, transport string, local bool) map[string]any {
	if transport == "auto" {
		return mcpEntry(provider, executable, server, apiKey)
	}
	if local {
		apiKey = ""
	}
	if transport == "stdio" {
		entry := map[string]any{"command": executable, "args": []string{"mcp", "--url", server}, "env": map[string]any{"SECTILE_AGENT_TOKEN": apiKey}}
		return entry
	}
	entry := map[string]any{"url": MCPURL(server)}
	headerField := "headers"
	switch provider {
	case "claude":
		entry["type"] = "http"
	case "codex":
		headerField = "http_headers"
	case "agy":
		delete(entry, "url")
		entry["serverUrl"] = MCPURL(server)
	}
	if !local {
		entry[headerField] = map[string]any{"Authorization": "Bearer " + apiKey}
	}
	return entry
}
