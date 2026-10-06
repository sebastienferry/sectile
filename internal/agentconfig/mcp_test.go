package agentconfig

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"tasks/internal/testhome"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

const (
	testServer = "http://sectile.example.test:8090"
	testKey    = "sectile_0123456789abcdef"
)

func readRegistration(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{}
	if strings.HasSuffix(path, "toml") {
		err = toml.Unmarshal(raw, &data)
	} else {
		err = json.Unmarshal(raw, &data)
	}
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The registration addresses the server with the workstation key, refreshes
// both when they change, and leaves everything else in the file alone.
func TestBootstrapMCPPreservesConfigAndRefreshesServerAndKey(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "agy"} {
		t.Run(provider, func(t *testing.T) {
			testhome.Temp(t)
			path, err := BootstrapMCP(provider, "/opt/sectile", testServer, testKey)
			if err != nil {
				t.Fatal(err)
			}
			data := readRegistration(t, path)
			data["unrelated_setting"] = "keep"
			key := "mcpServers"
			if provider == "codex" {
				key = "mcp_servers"
			}
			data[key].(map[string]any)["other"] = map[string]any{"command": "other-server"}
			var raw []byte
			if strings.HasSuffix(path, "toml") {
				raw, err = toml.Marshal(data)
			} else {
				raw, err = json.Marshal(data)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err = BootstrapMCP(provider, "/opt/new sectile", "https://moved.example.test/", "sectile_rotated"); err != nil {
				t.Fatal(err)
			}
			result := readRegistration(t, path)
			if result["unrelated_setting"] != "keep" {
				t.Fatal("unrelated settings lost")
			}
			raw, _ = json.Marshal(result)
			// HTTP providers address /mcp; stdio providers pass the bare server
			// URL to the bridge. Both must carry the new server and key.
			for _, want := range []string{"other-server", "https://moved.example.test", "sectile_rotated"} {
				if !strings.Contains(string(raw), want) {
					t.Fatalf("missing %q in %s", want, raw)
				}
			}
			for _, stale := range []string{"sectile.example.test", testKey, "8091"} {
				if strings.Contains(string(raw), stale) {
					t.Fatalf("stale server or credential %q in %s", stale, raw)
				}
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if perm := info.Mode().Perm(); perm&0o077 != 0 {
				t.Fatalf("registration file readable by others: %o", perm)
			}
		})
	}
}

// Providers that can speak Streamable HTTP are pointed at the server directly;
// the others run the stdio bridge against that same server with the key in its
// environment. Neither mentions a local gateway.
func TestBootstrapMCPTransportPerProvider(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "agy"} {
		t.Run(provider, func(t *testing.T) {
			testhome.Temp(t)
			path, err := BootstrapMCP(provider, "/opt/sectile", testServer+"/", testKey)
			if err != nil {
				t.Fatal(err)
			}
			data := readRegistration(t, path)
			var entry map[string]any
			if provider == "codex" {
				entry = data["mcp_servers"].(map[string]any)["sectile"].(map[string]any)
			} else {
				entry = data["mcpServers"].(map[string]any)["sectile"].(map[string]any)
			}
			if UsesHTTPMCP(provider) {
				if entry["url"] != testServer+"/mcp" {
					t.Fatalf("url = %v, want the server /mcp", entry["url"])
				}
				if entry["command"] != nil || entry["args"] != nil {
					t.Fatalf("HTTP registration still runs a command: %v", entry)
				}
				headers, _ := entry["headers"].(map[string]any)
				if headers["Authorization"] != "Bearer "+testKey {
					t.Fatalf("headers = %v", entry["headers"])
				}
				if provider == "claude" && entry["type"] != "http" {
					t.Fatalf("claude registration type = %v", entry["type"])
				}
				return
			}
			if entry["command"] != "/opt/sectile" {
				t.Fatalf("command = %v", entry["command"])
			}
			args, _ := json.Marshal(entry["args"])
			if string(args) != `["mcp","--url","`+testServer+`"]` {
				t.Fatalf("args = %s", args)
			}
			env, _ := entry["env"].(map[string]any)
			if env["SECTILE_AGENT_TOKEN"] != testKey {
				t.Fatalf("env = %v", entry["env"])
			}
		})
	}
}

func TestBootstrapMCPRejectsInvalidConfigWithoutOverwriting(t *testing.T) {
	root := t.TempDir()
	testhome.Set(t, root)
	path := filepath.Join(root, ".claude.json")
	raw := []byte("invalid json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := BootstrapMCP("claude", "/bin/sectile", testServer, testKey); err == nil {
		t.Fatal("invalid configuration accepted")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(raw) {
		t.Fatal("existing config damaged")
	}
	if _, err := BootstrapMCP("custom", "/bin/sectile", testServer, testKey); err == nil {
		t.Fatal("unsupported provider silently accepted")
	}
	if _, err := BootstrapMCP("codex", "/bin/sectile", "", testKey); err == nil {
		t.Fatal("missing server accepted")
	}
	if _, err := BootstrapMCP("codex", "/bin/sectile", "ftp://sectile", testKey); err == nil {
		t.Fatal("non-HTTP server accepted")
	}
	if _, err := BootstrapMCP("codex", "/bin/sectile", testServer, " "); err == nil {
		t.Fatal("missing key accepted")
	}
	if _, err := BootstrapMCP("codex", "sectile", testServer, testKey); err == nil {
		t.Fatal("relative executable accepted")
	}
}

// Exercise the installed provider's reader rather than only our JSON writer.
func TestBootstrapMCPVisibleToAgy(t *testing.T) {
	agy, err := exec.LookPath("agy")
	if err != nil {
		t.Skip("agy is not installed")
	}
	testhome.Temp(t)
	root := t.TempDir()
	if _, err := BootstrapMCP("agy", "/usr/bin/true", testServer, testKey); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(agy, "mcp", "list")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("agy mcp list: %v: %s", err, output)
	}
	if !strings.Contains(string(output), "sectile") || !strings.Contains(string(output), "/usr/bin/true") {
		t.Fatalf("agy did not discover Sectile: %s", output)
	}
}

// writeProviderMCPFile writes a provider's MCP configuration file under home.
func writeProviderMCPFile(t *testing.T, home, provider, content string) string {
	t.Helper()
	loc, err := ResolveLocations(provider)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, loc.MCPFile)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// staleMCPFixtures are registrations written with an earlier key, in the
// shape each provider holds them, beside an unrelated server and setting.
func staleMCPFixtures(command string) map[string]string {
	return map[string]string{
		"claude": `{"keep":"yes","mcpServers":{"other":{"command":"other-server","env":{"TOKEN":"other-key"}},` +
			`"sectile":{"type":"http","url":"` + testServer + `/mcp","headers":{"Authorization":"Bearer sectile_old"}}}}`,
		"codex": "keep = \"yes\"\n\n[mcp_servers.other]\ncommand = \"other-server\"\nenv = { TOKEN = \"other-key\" }\n\n" +
			"[mcp_servers.sectile]\nurl = \"" + testServer + "/mcp\"\nhttp_headers = { Authorization = \"Bearer sectile_old\" }\n",
		"agy": `{"keep":"yes","mcpServers":{"other":{"command":"other-server","env":{"TOKEN":"other-key"}},` +
			`"sectile":{"command":` + strconv.Quote(command) + `,"args":["mcp","--url","` + testServer + `"],"env":{"SECTILE_AGENT_TOKEN":"sectile_old"}}}}`,
	}
}

// A registration written with an earlier key follows the new one, keeping
// its transport, and nothing else in the file changes (#717).
func TestRefreshRegisteredMCPKeyRewritesAStaleKey(t *testing.T) {
	transports := map[string]string{"claude": "http", "codex": "http", "agy": "stdio"}
	for provider, transport := range transports {
		t.Run(provider, func(t *testing.T) {
			home := testhome.Temp(t)
			command := filepath.Join(home, "bin", "sectile-agent")
			path := writeProviderMCPFile(t, home, provider, staleMCPFixtures(command)[provider])

			entry, found, err := RegisteredMCPEntry(provider)
			if err != nil || !found || entry.APIKey != "sectile_old" || entry.Server != testServer || entry.Transport != transport {
				t.Fatalf("registered entry = %+v, %v, %v", entry, found, err)
			}
			wrote, err := RefreshRegisteredMCPKey(provider, command, testServer+"/", "sectile_new")
			if err != nil || !wrote {
				t.Fatalf("refresh = %v, %v", wrote, err)
			}
			entry, found, err = RegisteredMCPEntry(provider)
			if err != nil || !found || entry.APIKey != "sectile_new" || entry.Server != testServer || entry.Transport != transport {
				t.Fatalf("refreshed entry = %+v, %v, %v", entry, found, err)
			}
			data := readRegistration(t, path)
			if data["keep"] != "yes" {
				t.Fatalf("unrelated setting lost: %v", data)
			}
			raw, _ := json.Marshal(data)
			for _, want := range []string{"other-server", "other-key"} {
				if !strings.Contains(string(raw), want) {
					t.Fatalf("missing %q in %s", want, raw)
				}
			}
			if strings.Contains(string(raw), "sectile_old") {
				t.Fatalf("stale key left behind: %s", raw)
			}
			// The same key again is not a change: nothing is written.
			if wrote, err := RefreshRegisteredMCPKey(provider, command, testServer, "sectile_new"); err != nil || wrote {
				t.Fatalf("refresh with the current key = %v, %v", wrote, err)
			}
		})
	}
}

// Refreshing only follows a registration the user already has: it never
// registers Sectile with a provider that does not list it.
func TestRefreshRegisteredMCPKeyCreatesNothing(t *testing.T) {
	for _, provider := range MCPProviders {
		t.Run(provider, func(t *testing.T) {
			home := testhome.Temp(t)
			loc, err := ResolveLocations(provider)
			if err != nil {
				t.Fatal(err)
			}
			if wrote, err := RefreshRegisteredMCPKey(provider, filepath.Join(home, "sectile-agent"), testServer, "sectile_new"); err != nil || wrote {
				t.Fatalf("refresh without a file = %v, %v", wrote, err)
			}
			if _, err := os.Stat(filepath.Join(home, loc.MCPFile)); !os.IsNotExist(err) {
				t.Fatalf("a configuration file was created: %v", err)
			}
			content := `{"mcpServers":{"other":{"command":"other-server"}}}`
			if provider == "codex" {
				content = "[mcp_servers.other]\ncommand = \"other-server\"\n"
			}
			path := writeProviderMCPFile(t, home, provider, content)
			if wrote, err := RefreshRegisteredMCPKey(provider, filepath.Join(home, "sectile-agent"), testServer, "sectile_new"); err != nil || wrote {
				t.Fatalf("refresh without an entry = %v, %v", wrote, err)
			}
			if raw, _ := os.ReadFile(path); string(raw) != content {
				t.Fatalf("file changed: %s", raw)
			}
		})
	}
}

// A local registration carries no key and addresses the loopback; one for
// another server holds that server's key. Neither is this key's to rewrite.
func TestRefreshRegisteredMCPKeyLeavesAnotherServerAndLocalEntriesAlone(t *testing.T) {
	fixtures := map[string]string{
		"local":   `{"mcpServers":{"sectile":{"type":"http","url":"http://127.0.0.1:8091/mcp"}}}`,
		"foreign": `{"mcpServers":{"sectile":{"type":"http","url":"https://other.example.test/mcp","headers":{"Authorization":"Bearer sectile_other"}}}}`,
	}
	for name, content := range fixtures {
		t.Run(name, func(t *testing.T) {
			home := testhome.Temp(t)
			path := writeProviderMCPFile(t, home, "claude", content)
			if wrote, err := RefreshRegisteredMCPKey("claude", filepath.Join(home, "sectile-agent"), testServer, "sectile_new"); err != nil || wrote {
				t.Fatalf("refresh = %v, %v", wrote, err)
			}
			if raw, _ := os.ReadFile(path); string(raw) != content {
				t.Fatalf("file changed: %s", raw)
			}
		})
	}
}

// Over stdio the entry keeps the command it was written with, so a
// registration pointing at an installed agent is not moved to whichever
// binary happened to refresh it.
func TestRefreshRegisteredMCPKeyKeepsTheStdioCommand(t *testing.T) {
	home := testhome.Temp(t)
	installed := filepath.Join(home, "installed", "sectile-agent")
	writeProviderMCPFile(t, home, "agy", staleMCPFixtures(installed)["agy"])

	if wrote, err := RefreshRegisteredMCPKey("agy", filepath.Join(home, "elsewhere", "sectile-agent"), testServer, "sectile_new"); err != nil || !wrote {
		t.Fatalf("refresh = %v, %v", wrote, err)
	}
	entry, found, err := RegisteredMCPEntry("agy")
	if err != nil || !found || entry.Command != installed || entry.APIKey != "sectile_new" || entry.Transport != "stdio" {
		t.Fatalf("entry = %+v, %v, %v", entry, found, err)
	}
}
