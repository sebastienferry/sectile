# Plan #647 - A macro skill writes the macro's todos through MCP

## Stack

Go: `internal/taskmcp`, `internal/db` (macros), `internal/models`,
`internal/agentmcp`, `internal/mcptest`, `internal/skills`.

## Design

- `db.UpdateMacroTodos(projectID, macroKey string, lines []models.MacroTodo,
  mode string) ([]models.MacroTodo, error)`:
  - resolves the macro with `macroOf`;
  - validates every line (non-empty text, known id in replace, no id in
    append);
  - builds the new list: in replace, a line with an id starts from the
    stored line and overrides what the input sets (`done` is a pointer in the
    tool input so "unset" is distinguishable); a line without an id gets
    `uuid`; stored lines with a `storyKey` that are absent are reported in the
    refusal;
  - saves with `SaveMacroMeta(projectID, key, nil, nil, nil, &todos)`, which
    leaves every other field alone and writes nothing to the tracker.
- The tool input uses its own line type
  (`{id, text, done *bool, storyKey *string, targetProjectId *string,
  targetTrackerProject *string}`) mapped to `MacroTodo`.
- `MacroWorkspace.Todos` loses `omitempty`, and `PrepareMacroWorktree` turns a
  nil list into an empty one.
- Bridge: the allowed list gains `update_macro_todos`, the count becomes
  fourteen.
- `internal/mcptest/contract.go`: the contract calls the new tool.
- Skill: `refine_macro/steps.md` gets a step to deposit the confirmed slicing
  with the tool; golden files regenerated.

## Target files

- `internal/db/macros.go` (or a new `internal/db/macrotodos.go`) and tests
- `internal/taskmcp/server.go`, `internal/models/macrobranch.go`,
  `internal/db/macroruns.go`
- `internal/agentmcp/mcp.go` and its test, `internal/mcptest/contract.go`
- `internal/skills/fragments/refine_macro/steps.md`, golden files
- `CHANGELOG.md`
