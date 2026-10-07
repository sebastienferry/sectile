package taskmcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/tracker"
)

type macroListInput struct {
	ProjectID string `json:"projectId"`
}

type macroCreateInput struct {
	ProjectID string `json:"projectId"`
	Title     string `json:"title"`
	Horizon   string `json:"horizon,omitempty"`
}

type macroUpdateInput struct {
	ProjectID string `json:"projectId"`
	MacroKey  string `json:"macroKey"`
	db.MacroMetadata
}

func macroWriteResult(macro *models.MacroMeta, note string, err error) (*mcp.CallToolResult, any, error) {
	if err != nil && macro == nil {
		return nil, nil, err
	}
	out := map[string]any{"macro": macro}
	if note != "" {
		out["labelNote"] = note
	}
	if err != nil {
		out["localSaved"] = true
		out["trackerError"] = err.Error()
		return &mcp.CallToolResult{IsError: true, StructuredContent: out, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil, nil
	}
	return nil, out, nil
}

func addMacroResourceTools(server *mcp.Server, database *db.DB, resolve CallerResolver) {
	mcp.AddTool(server, &mcp.Tool{Name: "list_macros", Description: "List all macros of an explicit project, including closed macros, todos, write flags and tracker copy status. A known empty project returns an empty array; an unknown project fails."},
		func(ctx context.Context, req *mcp.CallToolRequest, in macroListInput) (*mcp.CallToolResult, any, error) {
			projectID := strings.TrimSpace(in.ProjectID)
			project, err := database.GetProjectByID(projectID)
			if err != nil {
				return nil, nil, err
			}
			if projectID == "" || project == nil {
				return nil, nil, fmt.Errorf("projet non trouvé")
			}
			macros, err := database.GetProjectMacros(projectID)
			if err != nil {
				return nil, nil, err
			}
			if macros == nil {
				macros = []models.MacroMeta{}
			}
			return nil, map[string]any{"macros": macros}, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "create_macro", Description: "Create a macro in an explicit project as the identified caller, using existing tracker rules. Requires a nonblank title; horizon defaults to now. Returns macro, or an error with macro, localSaved and trackerError when local persistence succeeds but the tracker operation reports failure."},
		func(ctx context.Context, req *mcp.CallToolRequest, in macroCreateInput) (*mcp.CallToolResult, any, error) {
			caller := callerOf(resolve, req)
			if err := requireCaller(caller); err != nil {
				return nil, nil, err
			}
			macro, err := database.CreateMacro(tracker.WithActingUser(ctx, caller.UserID), in.ProjectID, in.Title, in.Horizon, nil)
			return macroWriteResult(macro, "", err)
		})
	mcp.AddTool(server, &mcp.Tool{Name: "update_macro", Description: "Edit metadata of an existing project macro as the identified caller. Omitted fields are preserved; empty strings clear optional fields and false reopens. At least one field is required. Todos use update_macro_todos. Returns macro and any labelNote describing queued or local-only writes; reported tracker failures retain the saved macro with localSaved and trackerError."},
		func(ctx context.Context, req *mcp.CallToolRequest, in macroUpdateInput) (*mcp.CallToolResult, any, error) {
			caller := callerOf(resolve, req)
			if err := requireCaller(caller); err != nil {
				return nil, nil, err
			}
			macro, note, err := database.UpdateMacroMetadata(tracker.WithActingUser(ctx, caller.UserID), in.ProjectID, in.MacroKey, in.MacroMetadata)
			return macroWriteResult(macro, note, err)
		})
}
