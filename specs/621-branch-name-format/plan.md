# Plan #621 - Choose the format of the task branch name

Implements `spec.md`. One renderer in `internal/models`, shared by the server
(validation, runner fallback) and the agent (worktree branch), because the
agent binary must not link `internal/db`. One nullable-in-spirit column on
`projects`, carried like `pushStageCommits` (#564).

## Stack and constraints

- Go server and agent (`internal/models`, `internal/db`, `internal/agentconfig`,
  `internal/agent`, `internal/runner`, `internal/taskmcp`, `internal/handlers`),
  React web (`web/src`).
- Runtime strings (UI labels, server error messages) are French, the web modal
  also has its English locale; code, comments and docs are in English.
- The default must render byte for byte what `taskWorktreeBranch` renders
  today: existing agent tests must pass unchanged.

## Data contract

`branchNameFormat`: string, empty = default (`feat/{key_lower}`).

- `models.Project.BranchNameFormat` (`json:"branchNameFormat"`).
- `models.CreateProjectRequest.BranchNameFormat string`
  (`json:"branchNameFormat,omitempty"`).
- `models.UpdateProjectRequest.BranchNameFormat *string`
  (`json:"branchNameFormat,omitempty"`).
- `agentconfig.Config.BranchNameFormat string`
  (`json:"branchNameFormat,omitempty"`): additive, an older agent ignores it,
  an older server sends nothing (default).
- `get_project_context`: `"branchNameFormat"`.
- Web `Project.branchNameFormat?: string`.

Storage: migration 33 `projects.branch_name_format`:
`ALTER TABLE projects ADD COLUMN branch_name_format TEXT NOT NULL DEFAULT '';`
Added to the two project `SELECT`s, the `INSERT` and the `UPDATE` in
`internal/db/db.go`, and to `docs/API_AND_DATA_SPEC.md`. The migration tests
that drop the newest columns to rebuild an older schema
(`migrations_test.go`, `activerun_test.go`) drop it too.

## Renderer (`internal/models/branchformat.go`)

```go
const DefaultBranchNameFormat = "feat/{key_lower}"

// TaskBranchName renders format (empty = default) for a task key and title.
func TaskBranchName(format, key, title string) (string, error)

// ValidateBranchNameFormat checks a format before it is stored. The error
// is the French message shown to the user.
func ValidateBranchNameFormat(format string) error

// ValidBranchName is a pure-Go subset of `git check-ref-format --branch`.
func ValidBranchName(name string) bool
```

- `{key_lower}`: today's rule, moved from `taskWorktreeBranch`: lower-case,
  every rune outside `[a-z0-9-]` becomes `-`, then `-` trimmed at both ends.
  `#621` -> `621`, `AUC-1234` -> `auc-1234`.
- `{key}`: the same rule without lower-casing (`[A-Za-z0-9-]`), so
  `{key_lower}` is always `strings.ToLower({key})`. This refines T5 of the
  clarification: the key placeholders keep today's key rule instead of
  `SanitizeBranchName`, which also lets `_`, `.` and `/` through and would
  break the byte-for-byte default.
- An empty key slug is an error ("task key cannot produce a branch name"),
  as today.
- `{title}`: the slug rule of `MacroBranchName` (lower-case, runs of
  non-alphanumerics become one `-`, trimmed, cut at 30 characters then
  trimmed). `MacroBranchName` is refactored to call the same helper
  (`titleSlug`); its output does not change.
- Empty `{title}`: rendered empty, and the run of separators (`-`, `_`, `.`)
  immediately before the placeholder in the rendered text is dropped with it.
  If the placeholder starts the format, the run immediately after it is
  dropped instead. `feat/{key}-{title}` -> `feat/AUC-1234`,
  `{title}-{key}` -> `AUC-1234`.
- Parsing: scan the format once; `{name}` must be a known placeholder; a `{`
  without `}` or a stray `}` is an error.
- The render is not re-sanitized: the literal parts are the owner's choice and
  are validated at save time; the agent's `git check-ref-format --branch`
  stays the final gate at creation.

`ValidateBranchNameFormat` (trimmed format, empty is valid):

1. parse errors -> `format de nom de branche invalide : accolade non fermée`
   / `placeholder inconnu {x}` (placeholders listed);
2. no `{key}` nor `{key_lower}` -> `le format de nom de branche doit contenir
   {key} ou {key_lower}`;
3. render for `AUC-1234` / "Sample title"; not `ValidBranchName` or one of
   `main`, `master`, `HEAD` -> `le format de nom de branche donne « <render> »,
   qui n'est pas un nom de branche Git valide`.

`ValidBranchName` rules (git-check-ref-format(1), `--branch`): non-empty; no
ASCII control character, space, `~ ^ : ? * [ \`; no `..`, no `@{`, not `@`;
does not start with `-` or `/`, does not end with `/` or `.`; no `//`; no
path component starting with `.` or ending with `.lock`.

## Server

1. `internal/db/db.go` `CreateProject` / `UpdateProject`: trim, validate with
   `models.ValidateBranchNameFormat`, wrap in a new sentinel
   `ErrInvalidBranchNameFormat` (`internal/db/repositories.go`, next to
   `ErrInvalidSpecArtifacts`) so the French message is kept and
   `repositoryErrorStatus` answers 400 for it. Refused before any write; on
   update the stored value is unchanged.
2. `internal/db/agentconfig.go`: `BranchNameFormat: p.BranchNameFormat`.
3. `internal/taskmcp/server.go`: `"branchNameFormat"` in the project context.
4. `internal/db/db.go`: delete the unreferenced `GenerateTaskBranchName`
   (and its test, if any) - T1.
5. `internal/runner/runner.go` fallback (`task.BranchName == nil`): use
   `models.TaskBranchName("", task.Key, task.Title)`, falling back to the
   key on error. `PrepareAI` has no production caller today and receives no
   project, so the default format is the honest answer; the launches that
   run go through the agent, whose `{branchName}` is the created branch
   (`agent_template.go` expands `c.Branch`).

## Agent

1. `internal/agent/worktree_paths.go`: `taskWorktreeBranch(task, format)`
   keeps the `safeWorktreeName` guard and the assigned-branch shortcut, then
   calls `models.TaskBranchName(format, task.Key, task.Title)`.
   `localTaskPath(ctx, root, task, format)` passes it on.
2. `ensureLocalWorktree(ctx, root, task, useWorktrees, format)`; callers pass
   `config.BranchNameFormat`: `prepareWorkspace` (`agent_config.go`),
   `repositoryWorktree` (`repositories.go`), and the `workspace_info` /
   target lookups in `agent_operations.go` and the desktop launch in
   `agent_desktop.go`.
3. Assigned branches: unchanged path, so US4 holds by construction; the
   recorded branch is still written back by `EnsureTaskWorktree` / the
   `patchTask` of `agent.go`.

## Web

1. `web/src/types/index.ts`: `branchNameFormat?: string` on `Project`.
2. `web/src/lib/branchNameFormat.ts` (new): `renderBranchNameFormat(format,
   key, title)` mirroring the Go renderer for the live example, and
   `BRANCH_NAME_PRESETS`. Unit test `web/tests/branchNameFormat.test.ts`
   (node --test), with the same cases as the Go table test.
3. `web/src/components/ProjectModal.tsx`, workflow tab, under the PR stage
   block: a text input (`id="branchNameFormat"`, placeholder
   `feat/{key_lower}`), preset buttons, and the example
   `AUC-1234 -> <render>` (or the renderer's error). State loaded from and
   reset like `pushStageCommits`, sent in the create and update payloads.
4. `web/src/locales/projectSettings.ts`: `workflow.branchFormatTitle`,
   `branchFormatHelp`, `branchFormatExample`, `branchFormatPresets` in `fr`
   and `en`.
5. The server refusal is shown by the modal's existing save error handling.

## Documentation and skills

- `CHANGELOG.md` `[Unreleased]` `Added`: "Projects can choose the format of the
  branch Sectile creates for a task, for example `{key}` for `AUC-1234`, from
  the project settings (#621)."
- `docs/CAPABILITIES.md`: the setting, its placeholders and rules; "the
  assigned work branch (`feat/<n>`)" -> "the assigned work branch".
- `docs/API_AND_DATA_SPEC.md`: the column and the JSON field.
- `docs/contracts/server-agent-v1.md` (if it lists the config fields): the
  additive field.
- `internal/skills/fragments/clarify/guard.md`: "reuse the assigned feat/<n>
  branch" -> "reuse the assigned work branch". Regenerate the skill goldens
  (`UPDATE_GOLDEN=1 go test ./internal/skills/ -run TestGoldenSkillParity`) and
  the plugin test data from Linux (WSL): the skills tests are not reliable on
  Windows.

## Rejected alternatives

- Presets only: settled against in Q2.
- `SanitizeBranchName` for `{key}`: changes the default output for keys with
  `_` or `.`, and lets `/` from a key create path components.
- Server-side rendering of the branch: the agent creates the worktree and
  already owns `taskWorktreeBranch`; moving it would change the protocol.
- Running `git check-ref-format` on the server: the server may run without
  Git; a pure-Go subset is enough for a save-time check, the agent keeps the
  real one.

## Test plan

- `internal/models`: table tests for `TaskBranchName` (default, `{key}`,
  `{key_lower}`, `{title}` bounded, empty title with both positions, `#621`,
  unknown placeholder, unbalanced braces, empty key slug), for
  `ValidateBranchNameFormat` (each refusal, the three presets accepted,
  spaces-only accepted) and `ValidBranchName`; `MacroBranchName` tests
  unchanged.
- `internal/agent`: the existing `taskWorktreeBranch` tests unchanged with an
  empty format; new cases for `{key}` and an assigned branch that wins over a
  format.
- `internal/db`: round trip through `CreateProject`, `UpdateProject`,
  `GetProjectByID`, `AgentConfig`; a refused update leaves the stored value;
  migration adds the column with `''`.
- `internal/handlers`: a refused format answers 400 with the French message.
- `internal/taskmcp`: the context carries `branchNameFormat`.
- `internal/runner`: the fallback `{branchName}` is `feat/test-1` for
  `TEST-1`.
- Web: `branchNameFormat.test.ts`; `npm run build` type-checks the modal.
