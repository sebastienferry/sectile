package agentmcp

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"tasks/internal/agenthttp"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// bridgeKeepAlive keeps the upstream session alive and makes a dead server
// visible. The server closes sessions that fall silent, because a bridge that
// is killed outright never gets to announce its departure; pinging well inside
// that window is how a live conversation says it is still there.
const bridgeKeepAlive = 60 * time.Second

// clientLabel describes this bridge to the server, so an operator reading the
// session list sees which client is connected rather than an opaque identifier.
// Deployments name their own clients; the host and process are the fallback.
func clientLabel() string {
	if label := strings.TrimSpace(os.Getenv("SECTILE_MCP_CLIENT")); label != "" {
		return label
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown-host"
	}
	return fmt.Sprintf("%s/%d", host, os.Getpid())
}

// runMCPCommand relays MCP over stdio without opening a local database. All log
// output goes to stderr; stdout belongs exclusively to the protocol transport.
// Run executes the stdio MCP bridge until its context ends.
func Run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	endpoint := os.Getenv("SECTILE_AGENT_URL")
	if endpoint == "" {
		endpoint = os.Getenv("SECTILE_SERVER_URL")
	}
	if endpoint == "" {
		endpoint = "http://127.0.0.1:8090"
	}
	serverURL := fs.String("url", endpoint, "Agent gateway or Sectile server URL")
	token := fs.String("token", os.Getenv("SECTILE_AGENT_TOKEN"), "Agent bearer token (prefer SECTILE_AGENT_TOKEN)")
	label := fs.String("client", clientLabel(), "Name reported to the server for this client session")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	client := mcp.NewClient(
		&mcp.Implementation{Name: "sectile-stdio", Title: *label, Version: "1.0.0"},
		&mcp.ClientOptions{KeepAlive: bridgeKeepAlive},
	)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: strings.TrimRight(*serverURL, "/") + "/mcp", HTTPClient: agenthttp.Client(*token)}, nil)
	if err != nil {
		return fmt.Errorf("connect to Sectile MCP: %w", err)
	}
	defer session.Close()
	seen := map[string]bool{}
	proxy := mcp.NewServer(&mcp.Implementation{Name: "sectile", Version: "1.0.0"}, nil)
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return err
		}
		switch tool.Name {
		case "get_task", "transition_stage", "add_comment", "list_tasks", "get_project_context", "list_projects", "start_run", "finish_run", "create_task", "update_task":
		default:
			return fmt.Errorf("incompatible Sectile MCP catalog: upgrade server and agent together")
		}
		if seen[tool.Name] {
			return fmt.Errorf("incompatible Sectile MCP catalog: duplicate tool %s", tool.Name)
		}
		seen[tool.Name] = true
		proxy.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return session.CallTool(ctx, &mcp.CallToolParams{Name: req.Params.Name, Arguments: req.Params.Arguments})
		})
	}
	if len(seen) != 10 {
		return fmt.Errorf("incompatible Sectile MCP catalog: expected ten tools; upgrade server and agent together")
	}
	return proxy.Run(ctx, &mcp.StdioTransport{})
}
