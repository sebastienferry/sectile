package agentmcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPBridgeRejectsIncompatibleCatalog(t *testing.T) {
	for _, name := range []string{"sectile_get_task", "sectile_get_task", "get_task"} {
		t.Run(name, func(t *testing.T) {
			upstream := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1"}, nil)
			upstream.AddTool(&mcp.Tool{Name: name, InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				t.Error("incompatible upstream tool executed")
				return &mcp.CallToolResult{}, nil
			})
			server := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return upstream }, nil))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err := Run(ctx, []string{"--url", server.URL})
			if err == nil || !strings.Contains(err.Error(), "incompatible Sectile MCP catalog") {
				t.Fatalf("catalog accepted or wrong error: %v", err)
			}
		})
	}
}
