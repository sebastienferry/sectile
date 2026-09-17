# One spelling for workflow labels, whatever posts them

## Why
A workflow label is written two ways depending on which code path posts it. A task created
through `POST /api/tasks` or the MCP `create_task` tool gets the bare `new`; a task moved by a
stage transition gets `#clarified`, `#specified`, and so on. The spelling reaches the tracker:
GitHub issues #177 and #178 really carry a label named `new`, next to the `#reviewed` and
`#clarified` labels the transitions created.

The cause is that `SetWorkflowLabel` (`internal/db/db.go:1870`) honours whatever prefix the
caller passed, so the spelling is decided by thirteen call sites instead of one. Three of them
pass the bare form: `CreateTask` (`db.go:1980`), the clone path (`db.go:2139`), and
`db.go:4676`, which forwards `GetStageLabelForStatus`, whose return value is unprefixed by
design. A fourth is in the front end: `QuickAddModal.tsx:77` seeds the add form with `'new'`.

Nothing is broken by it — `StageOfTask` and `StaleWorkflowLabels` both trim the `#`, so a bare
label still resolves and is removed at the first transition. What is left is a tracker
accumulating two label names for one concept, visible in the repository's label list, and every
label filter having to know both.

## What Changes
- `SetWorkflowLabel` normalises the workflow label it writes: always `#<stage>`, lowercase,
  whether the caller passed `new` or `#New`. Non-workflow labels keep their own spelling and
  casing untouched.
- The three call sites that pass the bare form are aligned, so the intent is readable at the
  call site as well as enforced at the sink.
- The add form's default label becomes `#new`, and its badge styling stops depending on the
  absence of a prefix.
- A test pins the normalised output for a bare `targetLabel`, so a future call site cannot
  reintroduce the bare form without failing.
- The dead `WorkflowLabels` variable (`db.go:1806`), an unreferenced duplicate of
  `workflowLabelVariants`, is removed.

## Impact
- Affected specs: `task-creation-label`
- Affected code: `internal/db/db.go`, `web/src/components/QuickAddModal.tsx`,
  `internal/db/labels_test.go`
- No database migration, no tracker backfill, no change to the status-to-stage mapping.
