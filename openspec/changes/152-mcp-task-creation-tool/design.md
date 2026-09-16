# Design

## Decisions

### The tool delegates to `db.CreateTask`, it does not reimplement creation
`db.CreateTask` already resolves the project's tracker, allocates the key, calls
`ts.CreateIssue` when the tracker supports `CapCreate`, derives the external URL
and applies the `new` workflow label (`internal/db/db.go:1846-2010`). A second
creation path would drift from the HTTP adapter the first time either side gains a
field. The tool is a typed argument surface over that function and nothing else.

### `projectId` is required and never inferred
Every other read tool accepts an optional `projectId` because reading the wrong
project costs a retry. Creating in the wrong project costs a ticket on someone
else's board, and the mistake is invisible until a human finds it. The schema
marks `projectId` as required with `minLength: 1`, which is also what the skill
instructions already demand: "use the full task ID for mutations and an explicit
project ID for creation".

### `requireRemoteCreation` is forced, not exposed
The flag is a guard, not a trigger: `internal/db/db.go:1927` rejects creation when
the resolved source is neither `local`, `github` nor `linear`, while the actual
remote call is gated separately on `ts.Supports(tracker.CapCreate)`. Forcing it to
`true` means a Jira-backed project returns `remote task creation is not supported
for tracker "jira"` instead of silently producing a ticket that exists only in
SQLite. Projects on the `local` tracker are unaffected: their resolved source is
`local`, which the guard admits. Exposing the flag would only let a caller opt into
the silent-local outcome the ticket asks us to prevent.

### No status argument, and no bespoke consent flag
`openspec/specs/task-creation-label` requires a created task to land in
`to_clarify` carrying `new` as its sole workflow label. Accepting a status would
let an agent file a ticket directly into a later stage and skip the workflow the
board exists to enforce. The same reasoning covers consent: `add_comment` and
`transition_stage` already write to the tracker with no confirmation step, so a
gate on `create_task` alone would be inconsistent, and a boolean the calling agent
sets for itself is not a consent signal. The guard that does work is the required
`projectId`, because it cannot be satisfied by an agent that does not know what it
is filing against.

### The stdio proxy learns the ninth tool rather than tolerating any catalog
`cmd/agent/mcp.go:62-79` rejects an unknown name and any size other than eight.
Relaxing that check to "at least the eight known names" would let a mismatched pair
run half-broken, which is exactly what the check was added to prevent. The
whitelist and the count move to nine, and an agent older than its server keeps
failing with `incompatible Sectile MCP catalog: upgrade server and agent together`.

### `genericMCPTools` stays at eight
`internal/agentconfig/mcp_migration.go:14` maps legacy `taskflow_*` permission
references onto current names. `create_task` never had a `taskflow_` predecessor,
so an entry there would describe a migration that cannot exist.

## Rejected alternatives

- **A `quick_add` tool that takes only a title.** Smaller surface, but it must then
  infer the project, which is the failure this change is designed to avoid.
- **Reusing `transition_stage`'s free-form note to request creation.** Keeps the
  catalog at eight and satisfies the naming specification unchanged, but turns a
  typed workflow tool into an untyped command channel.
- **Exposing the whole `models.CreateTaskRequest`.** Seventeen fields including
  `status`, `source` and `externalUrl`, each of which lets a caller contradict the
  board's own invariants. Rejected in favour of the seven that describe a ticket.
- **A confirmation argument such as `confirm: true`.** Looks like a guard-rail,
  provides none: the agent that wants to create the task is the agent that sets the
  flag. A real gate belongs in the session's permission layer, not in the payload.
