package agentconfig

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ClaudeEntry is one "sectile" registration found in ~/.claude.json. The key never leaves this package.
type ClaudeEntry struct {
	Scope   string // "user" (top-level mcpServers) or "project" (projects[<path>].mcpServers)
	Project string
	URL     string
	key     string
}

// KeyMatches compares the entry's key with key in constant time. An empty key
// on either side never matches: an entry without a key is not "the daemon's".
func (e ClaudeEntry) KeyMatches(key string) bool {
	if e.key == "" || key == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(e.key), []byte(key)) == 1
}

// Keyless reports whether the entry carries no key, as a local-choice entry is written.
func (e ClaudeEntry) Keyless() bool { return e.key == "" }

// ReadClaudeEntries lists the Sectile registrations Claude Code reads from
// ~/.claude.json: the user-scope entry and every project-scope entry, which
// Claude Code prefers over the user one (#716). A missing file has none.
func ReadClaudeEntries() ([]ClaudeEntry, error) {
	data, fs, _, err := readClaudeFile()
	if fs != nil {
		fs.Close()
	}
	if err != nil || data == nil {
		return nil, err
	}
	var entries []ClaudeEntry
	if entry, ok := claudeEntry(data); ok {
		entry.Scope = "user"
		entries = append(entries, entry)
	}
	projects, _ := data["projects"].(map[string]any)
	paths := make([]string, 0, len(projects))
	for path := range projects {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		project, ok := projects[path].(map[string]any)
		if !ok {
			continue
		}
		if entry, ok := claudeEntry(project); ok {
			entry.Scope, entry.Project = "project", path
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

// RemoveClaudeProjectEntries deletes the project-scope "sectile" entries of the
// listed projects, so the managed user entry applies to them. Every other
// server, field and permission is kept. It returns the file it rewrote.
func RemoveClaudeProjectEntries(projects []string) (string, error) {
	data, fs, loc, err := readClaudeFile()
	if err != nil {
		return "", err
	}
	if fs != nil {
		defer fs.Close()
	}
	path := filepath.Join(loc.Home, loc.MCPFile)
	if data == nil || len(projects) == 0 {
		return path, nil
	}
	all, _ := data["projects"].(map[string]any)
	for _, name := range projects {
		project, _ := all[name].(map[string]any)
		servers, _ := project["mcpServers"].(map[string]any)
		delete(servers, "sectile")
	}
	if err := checkExternalMCPPolicies(loc.Home, "claude", path); err != nil {
		return "", err
	}
	if err := writeMCPFile(fs, loc.MCPFile, false, data); err != nil {
		return "", err
	}
	return path, nil
}

// readClaudeFile opens the home root and parses ~/.claude.json. A missing file
// returns nil data and no error; the caller closes the returned root.
func readClaudeFile() (map[string]any, *os.Root, Locations, error) {
	loc, err := ResolveLocations("claude")
	if err != nil {
		return nil, nil, loc, err
	}
	fs, err := os.OpenRoot(loc.Home)
	if err != nil {
		return nil, nil, loc, err
	}
	raw, err := fs.ReadFile(loc.MCPFile)
	if os.IsNotExist(err) {
		return nil, fs, loc, nil
	}
	if err != nil {
		fs.Close()
		return nil, nil, loc, err
	}
	data := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &data); err != nil {
			fs.Close()
			return nil, nil, loc, fmt.Errorf("read existing MCP configuration %s: %w", loc.MCPFile, err)
		}
	}
	if data == nil {
		fs.Close()
		return nil, nil, loc, fmt.Errorf("MCP configuration %s must be an object", loc.MCPFile)
	}
	return data, fs, loc, nil
}

// claudeEntry reads the "sectile" server of one mcpServers holder. The key is
// the bearer of an HTTP entry, else the stdio bridge's SECTILE_AGENT_TOKEN.
func claudeEntry(holder map[string]any) (ClaudeEntry, bool) {
	servers, _ := holder["mcpServers"].(map[string]any)
	server, ok := servers["sectile"].(map[string]any)
	if !ok {
		return ClaudeEntry{}, false
	}
	var entry ClaudeEntry
	if url, ok := server["url"].(string); ok {
		entry.URL = url
		headers, _ := server["headers"].(map[string]any)
		authorization, _ := headers["Authorization"].(string)
		entry.key = strings.TrimPrefix(authorization, "Bearer ")
		return entry, true
	}
	env, _ := server["env"].(map[string]any)
	entry.key, _ = env["SECTILE_AGENT_TOKEN"].(string)
	return entry, true
}
