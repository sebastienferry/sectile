# #432: Technical plan

References: [`spec.md`](spec.md), [`docs/clarifications/432.md`](../../docs/clarifications/432.md).

## Stack

React 19 + TypeScript (web), Go (server). No migration, no new dependency.

## Server

- `internal/db/macros.go`: `CreateStoryFromMacroTodo` (and `CreateStoryFromEpicTodo`)
  return `(*models.MacroMeta, *models.Task, error)` instead of the bare key.
- `internal/handlers/handlers.go`: the todo branch of `POST /macros/{key}/story` answers
  `{macro, epic, storyKey, task}`, matching the typed branch that already returns `task`.

## Web

- `web/src/types/index.ts`: `ToastLink { label; onOpen(); externalUrl? }`,
  `ToastMessage.link?: ToastLink`.
- `web/src/lib/toastTimer.ts` (new, pure): `toastDuration(toast)` (explicit duration,
  else 8000 with a link, else 3500) and `createDismissTimer(duration, onElapsed)` with
  `pause()`, `resume()`, `cancel()`, keeping the remaining time across pauses. Pauses are
  counted by reason (hover, focus) so one ending does not resume while the other holds.
- `web/src/components/ToastContainer.tsx`: `ToastItem` drives the timer, pauses on
  `mouseenter`/`focus` (capture, `onFocus`/`onBlur` bubble in React), renders the link as a
  button (`onOpen()` then remove) and, when `externalUrl` is set, an `ExternalLink` anchor
  (`target="_blank" rel="noopener noreferrer"`, `aria-label` from translations).
- `web/src/context/AppContext.tsx`: `addToast` uses `toastDuration`; `createTask`,
  `createStoryUnderMacro`, `createStoryFromMacroTodo` attach
  `link: { label: t.toasts.openTask + ' ' + key, onOpen: () => setSelectedTask(task),
  externalUrl: task.externalUrl }` when the response carries a task.
- `web/src/locales/translations.ts`: `toasts.openTask`, `toasts.openInTracker`.

## Rejected alternatives

- Refetching the todo story by key on the client: an extra round trip and a race with
  `fetchTasks`, when the server already holds the task.
- A generic `actions[]` on toasts: nothing else needs it yet.

## Tests

- `web/tests/toastTimer.test.mjs`: durations, pause/resume keeps the remaining time,
  overlapping pauses, cancel.
- `internal/handlers`: the todo branch returns `task` with the story key.
