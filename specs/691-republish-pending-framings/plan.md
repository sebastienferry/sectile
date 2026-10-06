# Plan #691 - Roadmap: republish every pending framing copy

## Stack and boundaries

- Go server (`internal/db`, `internal/handlers`): the listing, the batch op,
  the route.
- Web (`web/src`): the context actions, the roadmap button, the strings.
- No migration: the pending state is computed from the `framing_comment` and
  `framing_mirror_*` columns of `macros` (migration 45).
- No desktop, agent or MCP change.

## Data contracts

`GET /api/projects/{id}/macros/framing-mirror` (and `epics/`):

```json
[{"projectId": "…", "key": "PE-12", "title": "…"}]
```

`POST` on the same path, something pending, 202:

```json
{"queued": 2, "skipped": 1, "activity": {"id": "…", "action": "Cadrages de roadmap ➔ tracker", "…": "…"}}
```

Nothing pending, 200:

```json
{"queued": 0, "skipped": 1, "activity": null}
```

Tracker op (in memory, `SkillJob.Op`):

```go
TrackerOp{Kind: TrackerOpEpicFramingBulk, ProjectID: id, EpicKeys: []string{"PE-12", "PE-14"}}
```

## Target files

### `internal/db/macrotodosmirror.go`

- `pendingMacroCopies(part, projectID) (pending []models.MacroMeta, skipped int, err error)`:
  one query of `key, title, framing_comment, todos` and the part's state
  columns over the project's `macros` rows, one `todosMirrorScope` for the
  project, and for each row `eligibilityOf`; a copied row is pending when
  `macroCopyStatus` says it is not up to date, skipped when the part is empty
  and was never copied (`hash == "" && ref == ""`). Keys sorted as the query
  returns them (`ORDER BY key`).
- `PendingFramingCopies(projectID)` exposes the framing variant.
- `PushPendingFramingCopies(ctx, projectID, keys, steps)`: writes each key in
  turn through `pushMacroCopy(ctx, framingCopy, projectID, key, false)`,
  returning the written count, the failures and the refusal tracking that
  `refusalOrFailures` needs. `pushMacroCopy` already stores each failure on
  the macro (`recordMacroCopyFailure`) and keeps the cause in the chain
  (`%w`), so `isTrackerWriteRefusal` still recognizes a missing token.

### `internal/db/trackerops.go`

- `TrackerOpEpicFramingBulk TrackerOpKind = "epic_framing_bulk"`.
- `TrackerOp.EpicKeys []string`: the keys of the batch, listed at POST time.
- `buildTrackerOpJob`: action "Cadrages de roadmap ➔ tracker", summary
  "Recopie de N cadrage(s) en file d'attente", one step "Cible : KEY, KEY…".
- `processTrackerOpJob`: dispatch to `runEpicFramingBulkOp`.
- `runEpicFramingBulkOp`: one `✅`/`❌` step per epic, output
  "N cadrage(s) recopié(s)[, M échec(s) : …]", error through
  `refusalOrFailures("aucun cadrage recopié", …)` when nothing went through.
  "Déjà à jour" counts as gone through: nothing is left to write.

### `internal/handlers/handlers.go`

- Beside `push-horizons`: `parts[2] == "framing-mirror"` with
  `isMacroSegment(parts[1])` and `len(parts) == 3`, so the per-macro route
  `macros/{key}/framing-mirror` (four parts) is not caught.
- GET: `PendingFramingCopies`. POST: list, then `EnqueueTrackerOp` with
  `h.actingContext(r)` when the list is non-empty.
- A macro key spelled `framing-mirror` cannot exist: it is not a Jira key.

### Web

- `web/src/context/AppContext.tsx`: `pendingFramingCopies(projectId)` and
  `publishPendingFramings(projectId)`, shaped like `pendingHorizonPushes` and
  `pushPendingHorizons` (macros route first, epics fallback, toast, refresh
  of the activities).
- `web/src/components/RoadmapView.tsx`: a `pendingFramings` count read on the
  same triggers as `pendingPushes` (project, `activeJobCount`), through a ref;
  a button beside "labels à pousser", same style, `FileText` icon, shown when
  the count is > 0, re-read after the click.
- `web/src/locales/planning.ts`: `pendingFramings`, `pendingFramingsTitle`
  (plural forms, FR and EN).
- `web/src/locales/operations.ts`: `framingsPublishQueued`,
  `framingsPublishNothing`, `framingsPublishFailed`, `framingsPublishRefused`
  and the counts description (FR and EN).

### Documentation

- `docs/API_AND_DATA_SPEC.md`: the route, beside the per-macro one.
- `CHANGELOG.md`: one `Added` line.

## Rejected alternatives

- One `epic_framing` activity per epic: concurrent Jira writes with no cap
  and one Activities row per epic (Q2, settled by the owner).
- Recomputing the list when the activity runs: the counts in the toast would
  no longer describe the activity. The keys are frozen at POST time and each
  write still rechecks eligibility and the hash.
- Reusing `GetProjectMacros` for the listing: it may read GitHub milestones
  and fills fields the listing does not need; a direct query is enough on a
  Jira project, where every epic has a row.
