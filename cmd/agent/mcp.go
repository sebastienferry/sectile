package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(clone)
}

func agentHTTPClient(token string) *http.Client {
	return &http.Client{Transport: bearerTransport{token: token, base: http.DefaultTransport}, Timeout: 2 * time.Minute,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// runMCPCommand relays MCP over stdio without opening a local database. All log
// output goes to stderr; stdout belongs exclusively to the protocol transport.
func runMCPCommand(ctx context.Context, args []string) error {
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
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "sectile-stdio", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: strings.TrimRight(*serverURL, "/") + "/mcp", HTTPClient: agentHTTPClient(*token)}, nil)
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
		case "get_task", "transition_stage", "add_comment", "list_tasks", "get_project_context", "list_projects", "start_run", "finish_run":
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
	if len(seen) != 8 {
		return fmt.Errorf("incompatible Sectile MCP catalog: expected eight tools; upgrade server and agent together")
	}
	return proxy.Run(ctx, &mcp.StdioTransport{})
}
