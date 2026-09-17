# Design

## Context
`deriveRunIndicator(activities, taskId)` in `web/src/lib/remoteRunIndicator.ts` already answers, for
one task, "is something running on it, and in which state". `TaskCard` and `ListView` call it through
`RemoteRunBadge` to draw the glyph. `AppContext` holds `activities`, refreshed on its own poll, and
`tasks`, fetched from the server with every other filter folded into the query string.

`pinnedOnly` is the one boolean filter already in place: state in `AppContext`, persisted through
`persistFilter` into the per-project store, appended to the server query, toggled from `TaskFilters`,
cleared from `Header`. It is the shape to follow, with one deliberate departure — where the filter
runs.

## Goals
- One definition of "this task is active", shared with the badge, so the two cannot drift.
- The filter reachable where every other filter is, and applying to both views that show tasks.
- Live: a run that starts or ends moves its task in or out with no user gesture.

## Decisions

### D1. The predicate is derived from the runs, not stored on the task
Being active is not a property of a task, it is a property of the runs pointing at it. Deriving it
from `activities` is what makes it current: a run that ends removes its task on the next activities
refresh, with nothing to invalidate.

*Rejected:* a flag on the task, maintained by the server as runs start and stop. It duplicates state
that already exists, and every missed update leaves a task pinned as active for good.

### D2. The filter runs on the client, unlike `pinnedOnly`
`pinnedOnly` goes to the server because pinning is an indexed column of a task. The run state is not
a column at all, so filtering it server-side would mean joining activities into the task query for a
view concern. Two facts make the client side lossless here: `tasks` carries the whole filtered set —
there is no paging to defeat — and `activities` is already loaded for the badge.

*Rejected:* a `?active=1` parameter on the task query. It would put the run vocabulary into the task
API and, worse, make the filter refresh on the task poll rather than on the activities poll, so a run
would keep its task listed after it ended.

**This decision is the one to revisit if task paging is ever introduced**: a client-side filter over a
page would then show only the active tasks *of that page*, which reads as "nothing is running".

### D3. `activeTaskIds(activities, now)` returns a `Set`, beside `deriveRunIndicator`
The filter asks a question about many tasks at once; calling `deriveRunIndicator` per rendered card
would rescan every activity per card. One pass over `activities` builds the set, memoised on
`activities`, and each view membership-tests in constant time. It sits in the same module as
`deriveRunIndicator` and shares its constants, which is what keeps the two answers consistent — and
`web/tests/remoteRunIndicator.test.mjs` already covers that module.

### D4. A recently canceled run does not make its task active
`deriveRunIndicator` deliberately keeps a canceled run visible for `CANCELED_VISIBILITY_MS` so the
user sees the cancellation land. That is right for a badge on a card the user is looking at, and
wrong for a filter: the work has stopped. Active is therefore `waiting | running | queued`, and
`canceled` is excluded — the one place where this predicate and the badge's state differ, by
intention rather than by drift.

### D5. The filter applies where the views render tasks, not to `tasks` itself
Both views read `tasks` for two different purposes: rendering collections, and resolving a task by id
(drag and drop, the detail modal, the selection). Filtering the context's `tasks` would break the
second — a card dropped while the filter is on would resolve to nothing. The filter is applied to the
rendered collections only.

*Rejected:* exposing a second `visibleTasks` from the context. Both names would then be in scope at
every call site, and picking the wrong one fails silently.

### D6. Columns and groups stay when the filter empties them
No existing filter changes the structure of the board — a priority filter empties columns, it does
not remove them. An empty column under the filter is information: nothing is running at that stage.

### D7. The toggle is disabled when nothing is running, as the pin is
`TaskFilters` disables the pin with `disabled={!pinnedOnly && pinnedTasks.length === 0}`: unreachable
when it would yield nothing, always escapable once on. The same guard applies, and because the set is
live the toggle enables itself as soon as a run starts.

### D8. The predicate matches the badge's statuses, `pending` included in neither
`deriveRunIndicator` counts the statuses `running` and `queued`, and not `pending`, although
`runStateOf` in `shared/runStates.ts` folds `pending` in with `queued`. The filter follows the
indicator rather than the shared mapping, so that a filtered-in task always carries a visible badge.
Counting `pending` here would list tasks showing no mark at all, which reads as a bug.

*Left as is:* the gap belongs to the badge, not to the filter. Widening `deriveRunIndicator` to
`pending` would fix both at once and is worth its own change.

## Risks
- The filter is only as current as the activities poll. It is the same freshness the badge already
  has, so no surface becomes less accurate than the one beside it.
- A task whose run is active but which the server-side filters exclude never appears. This is stated
  as a requirement rather than fixed: a filter that widens its own result is the greater surprise.
