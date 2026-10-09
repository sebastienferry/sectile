# Plan #809 - Web UI Roadmap: Clickable macro ID

## Stack

- Web client, React and TypeScript: `web/src`.
- No agent, server, database, tracker, MCP or Desktop change. No child process
  is started, so the `agentexec.Hidden` rule does not apply.

## Data contract

None changes.

- Roadmap rows and the side panel already carry the macro address:
  `MacroRow.externalUrl` (`web/src/lib/roadmap.ts`, filled from
  `MacroMeta.externalUrl`).
- The Timeline only holds `Task` objects (`parentKey`, `parentTitle`,
  `projectId`). It reads addresses from the existing
  `fetchProjectMacros(projectId): Promise<MacroMeta[]>`
  (`web/src/context/AppContext.tsx`, `GET /projects/{id}/macros`), which
  already resolves to `[]` on any failure.

The server computes `externalUrl`: a GitHub milestone URL, a Jira
`<site>/browse/KEY`, nothing for a local `M-<n>` or a GitLab macro. The client
never derives one from a key.

## Design decisions

### D1 - One shared component, `MacroKeyRef`

New file `web/src/components/MacroKeyRef.tsx`:

```tsx
interface Props {
  macroKey: string
  /** The macro's tracker address; empty or missing renders plain text. */
  url?: string
  /** Classes of the key text, so each surface keeps its size and colour. */
  className?: string
  /** Icon size of the external-link and copy icons; 11 by default. */
  iconSize?: number
}
export function MacroKeyRef(props: Props): JSX.Element
```

It renders an inline-flex `<span data-macro-key={macroKey}>` holding:

- with `url`: `<a href={url} target="_blank" rel="noreferrer"
  title={openRemoteTitle}>` wrapping the key and an
  `ExternalLink` icon, as `renderTaskRef` does for the task key in
  `TaskDetailModal.tsx`;
- without `url`: `<span>{macroKey}</span>`;
- then `<button type="button">` with the `Copy` icon,
  `title` and `aria-label` set to `format(t.taskDetail.header.copyKey, {key})`.

It takes `addToast` and `t` from `useApp()`, like `CopyStepPromptButton`.
The copy goes through `copyText` (`web/src/lib/clipboard.ts`); on success it
sets a local `copied` state that swaps the icon for an emerald `Check` and
clears it after 2000 ms (timer cleared on unmount), then shows the
`t.taskDetail.toasts.keyCopiedTitle` / `keyCopiedDescription` toast; on
failure it shows `copyFailedTitle` / `copyFailedDescription` as an error.

Rejected: a component that receives the strings and the toast callback as
props. Every caller would pass the same five values.

### D2 - Event isolation inside rows and cards

The outer `<span>` of `MacroKeyRef` stops propagation of `click` and
`mousedown`. This keeps a click from reaching `onRowClick` (selection,
Ctrl/Cmd pick, Shift range) on the Roadmap and `setSelectedTask` on the
Timeline, and keeps the Roadmap's Shift `onMouseDown` from swallowing the link.
The link still opens through the browser's default action, since only
propagation is stopped.

Drags need more, because the browser fires `dragstart` on the nearest
draggable element: a press on the copy button of a draggable row would start a
drag of the row itself, with the row as the event target. The outer `<span>`
is therefore `draggable` itself, with an `onDragStart` that calls
`preventDefault` and `stopPropagation`. A drag begun on the button now starts
on the span and is cancelled; one begun on the link (draggable by default)
bubbles to the span and is cancelled too. Neither reaches the row's or the
card's `onDragStart`.

Rejected: checking `event.target` inside `onRowClick` and the Timeline
handlers. It spreads the rule over five call sites and two components.

### D3 - The Roadmap uses it in three places

In `web/src/components/RoadmapView.tsx`:

- `renderMacroRow`: the `<span className="text-[11px] font-mono font-bold">
  {row.key}</span>` becomes `<MacroKeyRef macroKey={row.key}
  url={row.externalUrl} className="text-[11px] font-mono font-bold" />`, the
  accent colour kept on the key;
- `renderCondensedRow`: same, keeping `shrink-0 text-[10.5px]`;
- the side panel header: same for `selected.key`, and the `<a>` "Lien" block
  that follows the "Copier" button is deleted. `copyMacroLink`,
  `macroCopyPayload` and the "Copier" button are untouched.

`strings.panel.link` becomes unused: remove the `link` key from both locales
of `web/src/locales/planning.ts` and from its type, if the type is declared
separately. `openRemoteTitle` stays and moves in use to `MacroKeyRef`, read as
`t.planning.roadmap.panel.openRemoteTitle`.

The attach-candidate list and the axes seed modal are not touched.

### D4 - The Timeline resolves addresses per project

New pure helpers in `web/src/lib/macroLinks.ts`:

```ts
/** The projects whose macro list the Timeline needs: those of tickets with a macro, sorted. */
export function macroProjectIds(tasks: Task[], fallbackProjectId?: string): string[]
/** Index key of a macro address: one project can reuse another's macro key. */
export function macroUrlKey(projectId: string, macroKey: string): string
/** Builds the address index from each project's macro list; macros without externalUrl are left out. */
export function macroUrlIndex(byProject: Record<string, MacroMeta[]>): Map<string, string>
```

A ticket without `projectId` uses `fallbackProjectId` (`currentProject?.id`).

In `web/src/components/SprintTimelineView.tsx`:

- take `fetchProjectMacros` from `useApp()`;
- `const projectIdsKey = macroProjectIds(tasks, currentProject?.id).join('\n')`
  (memoised on `tasks` and `currentProject?.id`);
- an effect on `projectIdsKey` fetches every project in parallel with
  `Promise.all`, then sets `macroUrls` to `macroUrlIndex(...)`. A
  `cancelled` flag set in the cleanup drops a stale answer. An empty key
  fetches nothing and sets an empty map;
- `macroUrlOf(task)` reads
  `macroUrls.get(macroUrlKey(task.projectId || currentProject?.id || '', task.parentKey))`.

The fetch is not repeated when tickets change without changing the set of
projects, so a sync that only updates tickets costs no request. A macro whose
address appears later (a milestone created after the list was read) is picked
up on the next change of project set or view mount; this is acceptable for a
link that falls back to plain text.

Rejected: a shared macro cache in `AppContext`. Only the Timeline needs it
here, and the Roadmap and the ticket detail already hold their own lists.
Rejected: deriving the address from the key as `trackerUrlForKey` does, which
the clarification excluded.

### D5 - Timeline placement

In the three layouts that render the `parentTitle` chip (sprint list rows,
sprint cards, backlog cards), insert before the chip:

```tsx
{task.parentKey?.trim() && (
  <MacroKeyRef macroKey={task.parentKey} url={macroUrlOf(task)}
    className="text-[9px] font-mono font-bold text-[var(--text-muted)]" iconSize={10} />
)}
```

The chip itself is unchanged and still shows only when `parentTitle` is set.
The chip display mode, which shows no macro, is not touched.

### D6 - The ticket detail keeps its own code

`TaskDetailModal.renderTaskRef` is not moved to `MacroKeyRef`: its parent link
has its own titles (parent type, title, tracker name), sits next to the "open
in roadmap" button, and keys its copied state per task.
`tests/task-ref-copy.browser.mjs` stays green as its guard.

## Target files

| File | Change |
| --- | --- |
| `web/src/components/MacroKeyRef.tsx` | new (D1, D2) |
| `web/src/lib/macroLinks.ts` | new (D4) |
| `web/src/components/RoadmapView.tsx` | three keys, "Lien" removed (D3) |
| `web/src/components/SprintTimelineView.tsx` | address index and three keys (D4, D5) |
| `web/src/locales/planning.ts` | `panel.link` removed in both locales (D3) |
| `web/tests/macroLinks.test.mjs` | new unit tests |
| `web/tests/macro-key-ref.browser.mjs` | new browser test of the component |
| `web/tests/roadmap-view.browser.mjs` | updated (US1, US3, US4) |
| `web/tests/sprint-timeline-macro-key.browser.mjs` | new browser test (US5) |
| `CHANGELOG.md` | `Added` line (AC-8) |

## Test plan

- `macroLinks.test.mjs` (`npm test`): `macroProjectIds` ignores tickets
  without a parent key, deduplicates and sorts, applies the fallback;
  `macroUrlIndex` skips macros without `externalUrl` and keeps two projects'
  `M-1` apart.
- `macro-key-ref.browser.mjs`, built on the `task-ref-copy.browser.mjs`
  harness (Vite, `useApp` replaced by `window.ctx`, clipboard permissions
  granted): link with `href`, `target`, `rel` when `url` is set, none
  otherwise; `Copier M-7` copies `M-7` and toasts `Identifiant copié`; a
  refused clipboard toasts `Copie impossible`; a click on the button or the
  link does not reach a parent `onClick` spy; inside a `draggable` parent, a
  `dragTo` from the button and from the link fires no parent `onDragStart`
  spy (a synthetic `dragstart` dispatched on the child would not show the
  browser's choice of drag source).
- `roadmap-view.browser.mjs`: rows of `M-3` and `M-1` per AC-1 and AC-2;
  Ctrl-click and plain click on the row's copy button leave the panel and the
  pick as they were (AC-4); panel header per AC-5. The existing selector
  `aside span.font-mono` and the `getByRole('link').count() === 0` /
  `.first()` assertions are adjusted to the new markup, and the existing
  "Copier" assertions keep passing with `exact: true`.
- `sprint-timeline-macro-key.browser.mjs`: renders `SprintTimelineView` with
  a `fetchProjectMacros` spy returning `M-3` with an address and `M-1`
  without; checks AC-6 in list and cards modes and in the backlog, and AC-7
  through the spy's call count. If mounting the whole view is impractical,
  the test covers the list rows and the backlog cards and says why.
- Run `npx tsc --noEmit`, `npx oxlint`, `npm test` and the browser tests
  (`PLAYWRIGHT_MODULE` pointing at the main checkout's Playwright).

## Documentation

- `CHANGELOG.md`, `## [Unreleased]` → `### Added`: one line on the macro key
  that opens the macro and copies its key in the Roadmap and Timeline, noting
  that the panel's "Link" button is folded into it. No README or `docs/` page
  describes these buttons.
