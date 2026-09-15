package agentconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const contextStart = "<!-- taskflow:project-context:start -->"
const contextEnd = "<!-- taskflow:project-context:end -->"

// checkoutMCPFiles are the registration files the previous release could write
// inside a repository. They are read to remove the managed entry from them.
var checkoutMCPFiles = []string{".mcp.json", ".codex/config.toml", ".gemini/settings.json", ".cursor/mcp.json", ".vibe/config.toml"}

// retireCheckout removes what earlier releases installed in the repository:
// managed skills and commands, the managed MCP registration and the project
// context block. A file the user has edited is preserved and reported. The
// checkout manifest is emptied afterwards, so this runs once per checkout.
func retireCheckout(work *os.Root, config Config) ([]string, error) {
	backups := []string{}
	manifest := map[string]string{}
	raw, err := work.ReadFile(".taskflow/agent-manifest.json")
	if err != nil && !os.IsNotExist(err) {
		return backups, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return backups, err
		}
	}
	for p := range manifest {
		if !managedLegacyPath(p) {
			return backups, fmt.Errorf("invalid managed skill path %q", p)
		}
	}
	for p, recorded := range manifest {
		raw, err := work.ReadFile(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return backups, err
		}
		if digest(raw) != recorded {
			backups = append(backups, "Edited skill kept in the checkout: "+p)
			continue
		}
		dst := filepath.Join(".taskflow/skill-backups", digest(raw), p)
		if err := atomicWrite(work, dst, raw); err != nil {
			return backups, err
		}
		backups = append(backups, dst)
		if err := work.Remove(p); err != nil {
			return backups, err
		}
	}
	if len(manifest) > 0 {
		if err := atomicWrite(work, ".taskflow/agent-manifest.json", []byte("{}\n")); err != nil {
			return backups, err
		}
	}
	if err := retireContextBlock(work); err != nil {
		return backups, err
	}
	for _, path := range checkoutMCPFiles {
		if err := retireCheckoutMCP(work, path); err != nil {
			return backups, err
		}
	}
	return backups, nil
}

// retireContextBlock removes the managed section from AGENTS.md and leaves every
// personal instruction around it untouched. Project identity now reaches the agent
// through the MCP interface.
func retireContextBlock(work *os.Root) error {
	raw, err := work.ReadFile("AGENTS.md")
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	content := string(raw)
	start, end := strings.Index(content, contextStart), strings.Index(content, contextEnd)
	if start < 0 && end < 0 {
		return nil
	}
	if start < 0 || end < start {
		return fmt.Errorf("incomplete project context block in AGENTS.md")
	}
	end += len(contextEnd)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	content = strings.TrimRight(content[:start], "\n") + "\n" + content[end:]
	if strings.TrimSpace(content) == "" {
		return work.Remove("AGENTS.md")
	}
	return atomicWrite(work, "AGENTS.md", []byte(content))
}

// retireCheckoutMCP drops the managed registration from a repository file while
// preserving every other server the user configured there.
func retireCheckoutMCP(work *os.Root, path string) error {
	raw, err := work.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	isTOML := strings.HasSuffix(path, ".toml")
	data := map[string]any{}
	if isTOML {
		err = toml.Unmarshal(raw, &data)
	} else {
		err = json.Unmarshal(raw, &data)
	}
	// A file Sectile cannot parse is a file it must not rewrite.
	if err != nil || data == nil {
		return nil
	}
	removed := false
	for _, key := range []string{"mcpServers", "mcp_servers"} {
		switch servers := data[key].(type) {
		case map[string]any:
			for _, name := range []string{"sectile", "taskflow"} {
				if _, exists := servers[name]; exists {
					delete(servers, name)
					removed = true
				}
			}
			if len(servers) == 0 {
				delete(data, key)
			}
		case []any:
			kept := []any{}
			for _, item := range servers {
				server, ok := item.(map[string]any)
				if ok && (server["name"] == "sectile" || server["name"] == "taskflow") {
					removed = true
					continue
				}
				kept = append(kept, item)
			}
			if len(kept) == 0 {
				delete(data, key)
			} else {
				data[key] = kept
			}
		}
	}
	if !removed {
		return nil
	}
	if len(data) == 0 {
		return work.Remove(path)
	}
	if isTOML {
		raw, err = toml.Marshal(data)
	} else {
		raw, err = json.MarshalIndent(data, "", "  ")
	}
	if err != nil {
		return err
	}
	return atomicWrite(work, path, raw)
}
