package agent

import (
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/url"
	"os"
	"strings"
	"tasks/internal/agenthttp"
)

const companionMCPName = "sectile_companion"

// The native launcher supplies this process-only endpoint. Never register it in
// user or project provider files: those may belong to another desktop instance.
func companionMCPURL() string {
	return os.Getenv("SECTILE_COMPANION_MCP_URL")
}

func (d *agentDaemon) checkCompanionMCP() conversationMCP {
	ctx, cancel := context.WithTimeout(context.Background(), sectileMCPCheckTimeout)
	defer cancel()
	status := conversationMCP{Status: "failed", Source: "transport"}
	if companionMCPURL() != d.link.serverURL {
		return status
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "sectile-companion-check", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: companionMCPURL() + "/mcp", HTTPClient: agenthttp.Client(d.link.token)}, nil)
	if err != nil {
		return status
	}
	defer session.Close()
	if _, err := session.ListTools(ctx, nil); err == nil {
		status.Status = "connected"
	}
	return status
}

func conversationMCPName() string {
	if companionMCPURL() != "" {
		return companionMCPName
	}
	return sectileMCPName
}

func companionMCPArgs(provider string) []string {
	endpoint := companionMCPURL()
	if endpoint == "" || (provider != "claude" && provider != "codex") {
		return nil
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Path != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		// Fail closed during provider configuration parsing rather than falling
		// back to the shared registration when a launcher is misconfigured.
		return []string{"--invalid-companion-mcp-endpoint"}
	}
	executable, err := os.Executable()
	if err != nil {
		return []string{"--unavailable-companion-mcp-executable"}
	}
	if provider == "claude" {
		config, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{companionMCPName: map[string]any{
			"type": "stdio", "command": executable, "args": []string{"mcp", "--url", endpoint},
		}}})
		return []string{"--strict-mcp-config", "--mcp-config", string(config)}
	}
	quote := func(value string) string { data, _ := json.Marshal(value); return string(data) }
	// Codex merges config tables, so replacing sectile's transport can retain
	// incompatible fields from Electron. Disable that entry for this process and
	// add a distinct server; the private key travels only through its environment.
	entry := "mcp_servers." + companionMCPName + "={command=" + quote(executable) + ",args=[\"mcp\",\"--url\"," + quote(endpoint) + "],env_vars=[\"SECTILE_AGENT_TOKEN\",\"SECTILE_DEVICE_ID\",\"SECTILE_RUN_ID\",\"SECTILE_PROJECT_ID\"],startup_timeout_sec=30}"
	return []string{"-c", "mcp_servers.sectile.enabled=false", "-c", entry}
}

func companionMCPFlags(provider string) string {
	args := companionMCPArgs(provider)
	for i := range args {
		args[i] = quoteShell(args[i])
	}
	return strings.Join(args, " ")
}
