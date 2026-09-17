# Filter the board and the backlog down to the tasks an agent is working on

## Why
A run's state is already visible one task at a time: `RemoteRunBadge` puts a glyph on the card in the
board and on the row in the list. What neither view can do is answer the question that glyph raises —
*what is running right now?* — without the user scanning three hundred cards for a coloured mark.

The board already carries the answer to the reverse question: `pinnedOnly` narrows both views to the
tasks the user chose to follow. Nothing narrows them to the tasks that are actually moving, and the
activities view does not fill the gap: it lists runs, not tasks, so it drops the board's columns, the
stages, and every other filter the user has set.

## What Changes
- Derive the set of tasks carrying a live remote run once, in `web/src/lib/remoteRunIndicator.ts`,
  from the same runs `deriveRunIndicator` already reduces for the badge.
- Add an `activeOnly` filter to `AppContext`, shaped like `pinnedOnly`: a boolean, persisted per
  project, cleared from the header like every other filter.
- Add its toggle to `TaskFilters`, the toolbar the board and the list already share.
- Apply it to what the board and the list render: the columns and the groups keep their shape and
  show fewer cards, rather than disappearing.

## Impact
- New: `activeTaskIds()` in `web/src/lib/remoteRunIndicator.ts`, an `activeOnly` filter in
  `web/src/context/AppContext.tsx`, a toggle in `web/src/components/TaskFilters.tsx`.
- Changed: `web/src/components/BoardView.tsx`, `web/src/components/ListView.tsx`,
  `web/src/components/Header.tsx`.
- Unchanged: the run lifecycle, the activity statuses the server stores, the badge itself, the
  server-side task query, and every other filter.

## Non-goals
- Filtering by a *particular* run state. The states are already filterable, one by one, in the
  activities view; a second state picker on the board would duplicate it.
- Serving the filter from the API. The run state is not a column of a task, and `tasks` is not
  paginated, so a client-side filter loses nothing. Revisit only if paging arrives.
- Extending the filter to the roadmap, the timeline or the triage view. #166 names the board and the
  backlog, and those three views are not reachable from the interface today.
