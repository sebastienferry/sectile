# Design

## Context
`SetWorkflowLabel(existingLabels, targetLabel)` is the single writer of workflow labels. It
drops every existing workflow label, keeps the others verbatim, then appends the target — and
today it copies the caller's prefix onto that target:

```go
if strings.HasPrefix(targetLabel, "#") {
    result = append(result, "#"+cleanTarget)
} else {
    result = append(result, cleanTarget)
}
```

`cleanTarget` is also not lowercased, so `SetWorkflowLabel(labels, "#New")` writes `#New`
today.

Thirteen call sites reach it. Nine already build `"#"+stage` themselves; three pass the bare
name (`db.go:1980`, `db.go:2139`, `db.go:4676`); `stage.go:90` forwards a label built by its
own caller. `GetStageLabelForStatus` returns unprefixed stage names and is compared as such in
several places (`db.go:2218`, `db.go:2598-2605`, `board.go:280`), so it is an internal stage
identifier, not a label.

Reading is already prefix-insensitive on both sides: `StageOfTask` (`board.go:325`) trims the
`#`, `workflowLabelVariants` (`db.go:1847`) lists both spellings so `StaleWorkflowLabels`
removes either from the tracker, and the front end de-prefixes in `AppContext.tsx`.

## Decisions

### Normalise in the sink, not at the call sites
Fixing the three bare call sites fixes today's instances; normalising in `SetWorkflowLabel`
removes the class. A fourteenth call site written next month cannot get the spelling wrong.
The call sites are aligned as well, but for readability — the guarantee lives in one function.

*Rejected:* correcting only the three call sites. It leaves the function's contract stating
that the caller decides, which is exactly the contract that produced the drift.

### `GetStageLabelForStatus` keeps returning the bare stage name
It is an internal stage identifier, compared against `oldStage`/`newStage` and against board
stage names. Prefixing it would force a trim at every comparison and move the same ambiguity
elsewhere. `SetWorkflowLabel` is the place where an identifier becomes a label, so that is
where the `#` is added.

### Non-workflow labels are left alone
`CustomerCase` and `#TeamTag` keep their casing and their prefix or absence of one. Only the
workflow label is normalised; the two behaviours are deliberately different and the existing
one is preserved.

### No backfill
The bare labels already posted on GitHub (#177, #178) are removed by their first transition,
since `StaleWorkflowLabels` deletes both spellings. A mass GitHub API pass would cost more than
it fixes.

### Casing is normalised with the prefix
Since the target is rewritten anyway, it is lowercased at the same time. `#New` in, `#new` out.
This extends the intent of commit cd0f908 (lowercase default workflow label) to every path
rather than just creation.

## Risks
- A test elsewhere asserting a bare workflow label would start failing. `chain_test.go`,
  `headless_test.go`, and `terminalrun_test.go` go through `StageOfTask`, which is
  prefix-insensitive, so they are expected to be unaffected — but the full suite is the check,
  not this prediction.
- Tracker sync posts `#new` where it posted `new`. GitHub creates the label on demand, so a
  repository will gain a `#new` label alongside the existing `new` one until the latter is
  emptied by transitions.

## Open questions
None. The clarification settled the normalisation site, the scope of the front-end change, the
absence of backfill, and the removal of `WorkflowLabels`.
