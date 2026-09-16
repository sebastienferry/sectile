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

// BootstrapMCP registers the daemon's stdio bridge in the selected CLI's
// user-level configuration. Nothing is written inside the repository: a checkout
// must stay free of agent configuration. The reserved Sectile entry migrates to
// Sectile; bearer credentials are never written. Native tool permissions remain
// in force.
func BootstrapMCP(provider, executable, gateway string) (string, error) {
	if !filepath.IsAbs(executable) {
		return "", fmt.Errorf("MCP executable must be an absolute path")
	}
	endpoint, err := url.Parse(gateway)
	if err != nil || endpoint.Scheme != "http" || (endpoint.Hostname() != "127.0.0.1" && endpoint.Hostname() != "localhost" && endpoint.Hostname() != "::1") {
		return "", fmt.Errorf("MCP bootstrap requires a running loopback agent gateway")
	}
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
	entry := map[string]any{"command": executable, "args": []string{"mcp", "--url", gateway}}
	if provider == "agy" {
		// Keep the shared registration independent of a particular agent process.
		// The bridge reads SECTILE_AGENT_URL and SECTILE_AGENT_TOKEN at runtime.
		entry["args"] = []string{"mcp"}
	}
	if err := migrateMCPRegistration(data, provider, entry); err != nil {
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
	if err = fs.WriteFile(temp, raw, 0600); err != nil {
		return "", err
	}
	if err = fs.Rename(temp, path); err != nil {
		return "", err
	}
	return filepath.Join(root, path), nil
}
