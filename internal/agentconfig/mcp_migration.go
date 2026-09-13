package agentconfig

import (
	"encoding/json"
	"fmt"
	"github.com/pelletier/go-toml/v2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

var genericMCPTools = []string{"get_task", "transition_stage", "add_comment", "list_tasks", "get_project_context", "list_projects", "start_run", "finish_run"}

// Migration is deliberately separate from serialization: any ambiguous policy
// aborts before the original configuration file is touched.
func migrateMCPRegistration(data map[string]any, provider string, transport map[string]any) error {
	key := "mcpServers"
	if provider == "codex" || provider == "vibe" {
		key = "mcp_servers"
	}
	for field, value := range data {
		if field != key && legacyMCPReference(value) {
			return fmt.Errorf("update legacy MCP references in %s manually before bootstrap; external policies are preserved", field)
		}
	}
	var legacy, canonical map[string]any
	var others []any
	var servers map[string]any
	if provider == "vibe" {
		if value, exists := data[key]; exists {
			list, ok := value.([]any)
			if !ok {
				return fmt.Errorf("%s must be an array", key)
			}
			for _, item := range list {
				server, ok := item.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid MCP server entry")
				}
				name, ok := server["name"].(string)
				if !ok || name == "" {
					return fmt.Errorf("MCP server name must be a nonempty string")
				}
				switch name {
				case "taskflow":
					if legacy != nil {
						return fmt.Errorf("duplicate taskflow registrations; reconcile manually")
					}
					legacy = server
				case "sectile":
					if canonical != nil {
						return fmt.Errorf("duplicate sectile registrations; reconcile manually")
					}
					canonical = server
				default:
					others = append(others, item)
				}
			}
		}
	} else {
		servers = map[string]any{}
		if value, exists := data[key]; exists {
			var ok bool
			servers, ok = value.(map[string]any)
			if !ok || servers == nil {
				return fmt.Errorf("%s must be an object", key)
			}
		}
		for _, name := range []string{"taskflow", "sectile"} {
			if value, exists := servers[name]; exists {
				server, ok := value.(map[string]any)
				if !ok || server == nil {
					return fmt.Errorf("%s registration must be an object", name)
				}
				if name == "taskflow" {
					legacy = server
				} else {
					canonical = server
				}
			}
		}
	}
	oldPolicy, err := preservedMCPFields(legacy, provider)
	if err != nil {
		return err
	}
	policy, err := preservedMCPFields(canonical, provider)
	if err != nil {
		return err
	}
	for field, value := range oldPolicy {
		if current, exists := policy[field]; exists && !reflect.DeepEqual(current, value) {
			return fmt.Errorf("conflicting taskflow/sectile field %s; reconcile manually without broadening permissions", field)
		}
		// When both entries exist, carrying an approval grant from only one entry
		// could broaden the other's defaults. Require explicit reconciliation.
		if _, exists := policy[field]; !exists && canonical != nil && mcpPermissionField(field) {
			return fmt.Errorf("taskflow-only policy %s conflicts with sectile defaults; reconcile manually", field)
		}
		policy[field] = value
	}
	if legacy != nil && canonical != nil {
		for field := range policy {
			if _, exists := oldPolicy[field]; !exists && mcpPermissionField(field) {
				return fmt.Errorf("sectile-only policy %s conflicts with taskflow defaults; reconcile manually", field)
			}
		}
	}
	for field, value := range transport {
		policy[field] = value
	}
	if provider == "vibe" {
		policy["name"] = "sectile"
		policy["transport"] = "stdio"
		data[key] = append(others, policy)
	} else {
		delete(servers, "taskflow")
		servers["sectile"] = policy
		data[key] = servers
	}
	return nil
}

func mcpTransportField(field string) bool {
	switch field {
	case "name", "command", "args", "env", "env_vars", "cwd", "url", "httpUrl", "serverUrl", "headers", "http_headers", "bearer_token_env_var", "type", "transport", "api_key_env", "api_key_header", "api_key_format", "oauth", "authProviderType":
		return true
	}
	return false
}

func mcpPermissionField(field string) bool {
	// Unknown fields may carry provider-specific policies. Only the established
	// operational timeout fields are safe to merge across missing defaults.
	switch field {
	case "startup_timeout_sec", "startup_timeout_ms", "tool_timeout_sec", "timeout":
		return false
	}
	return true
}

func preservedMCPFields(server map[string]any, provider string) (map[string]any, error) {
	out := map[string]any{}
	for field, value := range server {
		if mcpTransportField(field) {
			continue
		}
		toolList := (provider == "codex" && (field == "enabled_tools" || field == "disabled_tools")) ||
			(provider == "gemini" && (field == "includeTools" || field == "excludeTools")) ||
			(provider == "agy" && field == "disabledTools")
		if toolList {
			list, ok := value.([]any)
			if !ok {
				return nil, fmt.Errorf("%s must be an array of tool names", field)
			}
			mapped := make([]any, 0, len(list))
			for _, item := range list {
				name, ok := item.(string)
				if !ok {
					return nil, fmt.Errorf("%s must contain only tool names", field)
				}
				converted, err := migrateMCPToolReference(name)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", field, err)
				}
				mapped = append(mapped, converted)
			}
			out[field] = mapped
		} else {
			if legacyMCPReference(value) || legacyMCPReference(field) {
				return nil, fmt.Errorf("unsupported legacy policy in %s; update references manually before bootstrap", field)
			}
			out[field] = value
		}
	}
	return out, nil
}

func migrateMCPToolReference(name string) (string, error) {
	for _, tool := range genericMCPTools {
		if name == "taskflow_"+tool {
			return tool, nil
		}
	}
	// Preserve an existing all-tools value; provider pattern semantics differ,
	// so prefixed globs and regular expressions require manual reconciliation.
	if name == "*" {
		return "*", nil
	}
	if legacyMCPReference(name) || strings.ContainsAny(name, "*?[]") || strings.HasPrefix(name, "re:") {
		return "", fmt.Errorf("unsupported tool pattern; update it manually before bootstrap")
	}
	return name, nil
}

func legacyMCPReference(value any) bool {
	switch v := value.(type) {
	case string:
		return v == "taskflow" || strings.Contains(v, "taskflow_") || strings.Contains(v, "mcp__taskflow")
	case []any:
		for _, item := range v {
			if legacyMCPReference(item) {
				return true
			}
		}
	case map[string]any:
		for key, item := range v {
			if legacyMCPReference(key) || legacyMCPReference(item) {
				return true
			}
		}
	}
	return false
}

// Policies in separate files cannot be rewritten atomically with registration.
// Detect known provider settings containing old references and request manual
// reconciliation. Their bytes and all credentials stay untouched.
func checkExternalMCPPolicies(root, provider, target string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	var paths []string
	switch provider {
	case "claude":
		paths = []string{filepath.Join(root, ".claude/settings.json"), filepath.Join(root, ".claude/settings.local.json"), filepath.Join(home, ".claude/settings.json")}
	case "codex":
		paths = []string{filepath.Join(home, ".codex/config.toml")}
	case "gemini":
		paths = []string{filepath.Join(home, ".gemini/settings.json")}
	case "cursor":
		paths = []string{filepath.Join(home, ".cursor/mcp.json"), filepath.Join(root, ".cursor/permissions.json"), filepath.Join(home, ".cursor/permissions.json")}
	case "vibe":
		paths = []string{filepath.Join(home, ".vibe/config.toml")}
	}
	for _, path := range paths {
		if filepath.Clean(path) == filepath.Clean(target) {
			continue
		}
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read MCP policy file %s: %w", path, err)
		}
		var settings map[string]any
		if strings.HasSuffix(path, ".toml") {
			err = toml.Unmarshal(raw, &settings)
		} else {
			err = json.Unmarshal(raw, &settings)
		}
		if err != nil || settings == nil {
			return fmt.Errorf("invalid MCP policy file %s; repair it before bootstrap", path)
		}
		if legacyMCPReference(settings) {
			return fmt.Errorf("legacy MCP references in %s; update registrations and policies manually before bootstrap", path)
		}
	}
	return nil
}
