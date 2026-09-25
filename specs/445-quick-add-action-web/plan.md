# #445: Implementation plan

References: [`spec.md`](./spec.md), [`tasks.md`](./tasks.md).

## Stack and scope

Web client only (`web/`, React 19 + Tailwind, TypeScript). No Go change, no
migration, no new endpoint.

## Target files

| File | Change |
| --- | --- |
| `web/src/lib/quickAdd.ts` (new) | Pure rules: macro options, macro pre-selection, follow-up type. |
| `web/src/components/QuickAddModal.tsx` | Two-column layout, `MarkdownEditor`, macro field, follow-up choice, destination removed. |
| `web/src/context/AppContext.tsx` | `createTask` accepts `macroKey` and attaches after creation. |
| `web/src/locales/translations.ts` | New `quickAdd.*` strings (fr, en). |
| `web/tests/quickAdd.test.mjs` (new) | Unit tests for `lib/quickAdd.ts`. |
| `web/tests/quick-add.browser.mjs` (new) | Browser regression for the dialog. |
| `CHANGELOG.md` | One `Changed` line. |

## Decisions

### Macro attachment: second call from the client

The ticket is created with `POST /api/tasks`, then attached with
`POST /api/tasks/{id}/macro`, the endpoint the detail modal already uses. It
calls `SetTaskMacro`, which writes the parent locally and queues the
`set_parent` tracker op.

Rejected: having `POST /api/tasks` run `SetTaskMacro` when `parentKey` is set.
It would change what `create_task` over MCP and the batch endpoint do with
`parentKey` today, which the clarification left out of scope, for no gain the
user can see. The client call keeps the attachment failure separate from the
creation, which is what FR5 asks.

`createTask` gains an optional `macroKey`. After a successful creation it posts
the attachment; a refusal or a network error adds a `warning` toast
("Ticket créé mais non rattaché à M-7") and the created task is still returned.
A successful attachment replaces the task in the board state with the one the
endpoint returns, so the card shows its macro at once. `parentKey` is not sent
on the creation itself, so the local row never claims a parent the tracker has
not been asked for.

### Macro options and pre-selection

`quickAddMacroOptions(macros)` keeps macros whose `closed` is not true, sorted
by key in natural order. `initialQuickAddMacro(filter, options)` returns the
option key that equals the filter, or whose title equals it (the board filter
can hold either, see `filteredTasks` in `AppContext`), and `''` otherwise, which
also covers `__no_macro__` and `none`.

The modal loads `fetchProjectMacros(taskProjectId)` in an effect keyed on the
project and the open state; a request counter drops a stale response. The
pre-selection is applied when the list arrives for the project the dialog opened
with; a project change resets the selection to `''` before the new list loads.

### Follow-up

`type QuickAddFollowUp = 'none' | 'rewrite' | 'clarify'`, state in the modal,
reset to `'none'` in the opening effect. Rendered as a radio group
(`role="radiogroup"`, native radio inputs) in the left column under the
description, since it is about the text.

After `createTask` resolves with a task:

- `rewrite`: close the dialog, `setSelectedTask(created)`, then
  `runSkill(created.id, 'rewrite_story', '')`, the call the detail modal's
  "Reformuler la story" button makes;
- `clarify`: close the dialog, then `runSkill(created.id, 'clarify')` with no
  mode or model, so the project's precedence applies;
- `none`: close the dialog.

`runSkill` already shows the queued toast and an error toast on failure.

### Layout

`max-w-4xl`, body `grid grid-cols-1 md:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]`,
the same proportions as the detail modal. The dialog gets `max-h` of the app
height with an internally scrolling body so the footer stays reachable. The
description uses `MarkdownEditor` with `minHeight={260}`. The dialog gains
`role="dialog"`, `aria-modal` and `aria-labelledby` for the tests and assistive
technology.

The existing `aria-label` of the project select (`t.boardViews.projectForNewTicket`)
is kept: `board-views.browser.mjs` finds the form through it.

### Destination

`source` state, `handleProjectChange` source juggling and the "Destination"
block are removed; `createTask` is called without `source`. The unused
`quickAdd.tracker` string is removed from both locales and the schema.

## Test strategy

- Unit: `node --test tests/quickAdd.test.mjs` for the pure rules.
- Browser: `quick-add.browser.mjs` reuses the `board-views.browser.mjs` harness
  shape (real `App`, fake `fetch` in the page) and asserts on the requests the
  fake records: creation body without `source`, attachment call, run-skill
  calls, macro list reload; plus layout (two columns wide, stacked narrow) and
  the reset of the follow-up.
- Regression: `board-views.browser.mjs` S15 still passes.
