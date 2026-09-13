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

// BootstrapMCP registers the daemon's stdio bridge in the target CLI's project
// configuration (user configuration for agy). Only TaskFlow's entry is replaced;
// bearer credentials are never written. Native tool permissions remain in force.
func BootstrapMCP(root, provider, executable, gateway string) (string, error) {
	if !filepath.IsAbs(executable) {
		return "", fmt.Errorf("MCP executable must be an absolute path")
	}
	endpoint, err := url.Parse(gateway)
	if err != nil || endpoint.Scheme != "http" || (endpoint.Hostname() != "127.0.0.1" && endpoint.Hostname() != "localhost" && endpoint.Hostname() != "::1") {
		return "", fmt.Errorf("MCP bootstrap requires a running loopback agent gateway")
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	path := ""
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "codex":
		path = ".codex/config.toml"
	case "claude":
		path = ".mcp.json"
	case "agy":
		// agy ignores workspace MCP files and reads this user-level registry.
		root, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = ".gemini/config/mcp_config.json"
	case "gemini":
		path = ".gemini/settings.json"
	case "cursor":
		path = ".cursor/mcp.json"
	case "vibe":
		path = ".vibe/config.toml"
	default:
		return "", fmt.Errorf("automatic MCP bootstrap is unsupported for provider %q; select a supported aiProvider", provider)
	}
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
		// The bridge reads TASKFLOW_AGENT_URL and TASKFLOW_AGENT_TOKEN at runtime.
		entry["args"] = []string{"mcp"}
	}
	if provider == "vibe" {
		entry["name"] = "taskflow"
		entry["transport"] = "stdio"
		var list []any
		if current, ok := data["mcp_servers"]; ok {
			existing, ok := current.([]any)
			if !ok {
				return "", fmt.Errorf("mcp_servers must be an array")
			}
			for _, item := range existing {
				server, ok := item.(map[string]any)
				if !ok {
					return "", fmt.Errorf("invalid MCP server entry")
				}
				if server["name"] != "taskflow" {
					list = append(list, item)
				}
			}
		}
		data["mcp_servers"] = append(list, entry)
	} else {
		key := "mcpServers"
		if provider == "codex" {
			key = "mcp_servers"
		}
		servers, ok := data[key].(map[string]any)
		if !ok && data[key] != nil {
			return "", fmt.Errorf("%s must be an object", key)
		}
		if servers == nil {
			servers = map[string]any{}
		}
		// Keep explicit permission and tool policies, replacing only transport fields.
		if previous, ok := servers["taskflow"].(map[string]any); ok {
			for key, value := range previous {
				switch key {
				case "command", "args", "env", "env_vars", "cwd", "url", "httpUrl", "serverUrl", "headers", "http_headers", "bearer_token_env_var", "type", "transport":
				default:
					entry[key] = value
				}
			}
		}
		servers["taskflow"] = entry
		data[key] = servers
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
	temp := filepath.Join(filepath.Dir(path), ".taskflow-mcp-"+uuid.NewString()+".tmp")
	defer fs.Remove(temp)
	if err = fs.WriteFile(temp, raw, 0600); err != nil {
		return "", err
	}
	if err = fs.Rename(temp, path); err != nil {
		return "", err
	}
	return filepath.Join(root, path), nil
}
