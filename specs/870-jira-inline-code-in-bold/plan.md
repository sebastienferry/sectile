# Plan #870 - Jira refuses a comment whose Markdown has inline code inside bold text

## Stack

Go server: `internal/trackerapi` (converter), `internal/db` (tracker job),
`internal/taskmcp` (MCP tool), `internal/skills` (rendered skill contract).
No migration, no web or desktop change.

## 1. Converter (`internal/trackerapi/adf.go`)

`parseInline` emits inline code with the marks of its enclosing span plus
`code`. The `code` branch keeps only the `link` marks of that span:

```go
emit(code, append(linkMarks(marks), adfMark{Type: "code"}))
```

`linkMarks` returns a fresh slice holding the `link` marks of its argument.
`parseInline` is the only place creating marked text, so no post-pass over the
document is needed.

Rejected: splitting the node. The code span is one run of text; there is no
valid ADF form making it bold.

## 2. Waiting for the stage job (`internal/db`)

`transitionTaskStage` returns the activity of the `TrackerOpStage` job; the
worker closes that activity through `finishTrackerOp` with `completed` or
`failed` and the error text. The wait observes that row:

```go
// AwaitTrackerOp polls the activity until it leaves queued/running, the bound
// expires or ctx ends.
func (d *DB) AwaitTrackerOp(ctx context.Context, activityID string, bound time.Duration) (*models.TaskActivity, error)
```

It polls `GetActivityByID` every 100 ms. Polling the row rather than a
completion channel works whichever server instance runs the job, and needs no
change to the queue. It returns the last row read; a read error is returned.

## 3. MCP tool (`internal/taskmcp/server.go`)

After a successful transition, `transition_stage` calls
`AwaitTrackerOp(ctx, activity.ID, stageSyncWait)` with
`stageSyncWait = 15 * time.Second` (a package variable, so tests can shorten
it) and maps the row:

| Activity status | `trackerSync` | Extra field |
| --- | --- | --- |
| `completed` | `done` | |
| `failed` / `canceled` | `failed` | `trackerError`: the activity's error |
| anything else, or a read error | `queued` | |

`activity` in the result is the row last read. The tool description documents
the three values and says the stage stays applied on `failed`.

## 4. Skill contract

`internal/skills/fragments/contracts/transition.md`, in the standalone
paragraph: read `trackerSync` in the result; on `failed`, report
`trackerError` and post the same note with `add_comment`; on `queued`, say the
report is still being synchronized. The golden files under
`internal/skills/testdata` are regenerated.

## Target files

- `internal/trackerapi/adf.go`, `internal/trackerapi/adf_test.go`
- `internal/db/trackerops.go`, a test in `internal/db`
- `internal/taskmcp/server.go`, `internal/taskmcp/jira_test.go`
- `internal/skills/fragments/contracts/transition.md`, golden files
- `CHANGELOG.md`
