# Plan — colour board cards per epic

The "Review revision" section at the end supersedes the stack, the bar
geometry, the components and the data contracts below where they differ.

## Stack

Web front end only: React 19 + TypeScript + Tailwind under `web/`. Tests are
node `--test` suites under `web/tests/*.test.mjs` importing `.ts` sources
directly. Gate: `make test` web part — `npm test && npx tsc --noEmit &&
npx oxlint src` in `web/`.

## Derivation

`web/src/lib/epicColor.ts`, pure and free of React:

- `epicColor(parentKey?: string | null): AccentDefinition | null` — trims the
  key, returns `null` when empty, otherwise
  `ACCENT_COLORS[fnv1a(key) % ACCENT_COLORS.length]`.
- `fnv1a` is the 32-bit FNV-1a hash over the UTF-16 code units, computed with
  `Math.imul` and `>>> 0` so it stays an unsigned 32-bit integer.
- `epicColorHex(parentKey)` returns the palette `hex`, or `null`. Colours are
  applied as inline styles, as `accents.ts` already does, because Tailwind
  cannot generate classes it never sees and index.css remaps the raw Tailwind
  families.

### Why a hash, and why these rejected alternatives

- **Index over the sorted set of visible epics** gives distinct colours up to
  twelve epics, but the colour of an epic would then depend on which other
  epics are visible: a filter, a project switch or another view would repaint
  it, which breaks P2. Rejected.
- **A stored colour per epic** needs a `MacroMeta` field, an API and a picker;
  ruled out by the clarification.
- **Hash** is stable per key, everywhere, with no shared state. Collisions are
  accepted by the clarification.

### Why an absolutely positioned bar

The card border and ring already express running, queued, selected and
dragging. A `border-left` would override the left side of that border; an
inset `box-shadow` would be overwritten by Tailwind's `ring`, which is itself
a `box-shadow`. The bar is therefore a child `span`,
`absolute left-0 top-0 bottom-0 w-[3px] pointer-events-none rounded-l-[inherit]`,
inside a container that is already `relative` or becomes so.

## Components

`web/src/components/EpicMarker.tsx`:

- `EpicBar({ parentKey })` — the absolute left bar, `aria-hidden`, renders
  nothing without a parent.
- `EpicDot({ parentKey })` — a 6px round dot, `role="img"` with
  `aria-label="Épic <key>"` (French, the interface's language), renders nothing
  without a parent.

## Target files

| File | Change |
| --- | --- |
| `web/src/lib/epicColor.ts` | new: hash, colour lookup, styles |
| `web/src/components/EpicMarker.tsx` | new: `EpicBar`, `EpicDot` |
| `web/src/components/TaskCard.tsx` | bar on the root (both densities), dot before parent key (expanded) |
| `web/src/components/ListView.tsx` | bar on the row (`<tr>` first cell), dot in the parent badge |
| `web/src/components/SprintTimelineView.tsx` | dot on the chips, bar + dot on the list rows, in the three places each form is rendered |
| `web/src/components/RoadmapView.tsx` | bar on `renderMacroRow` and `renderCondensedRow` (`relative` added) |
| `web/tests/epicColor.test.mjs` | new node test |
| `CHANGELOG.md` | `Added` entry under `[Unreleased]` |

A `<tr>` cannot be `position: relative` reliably across browsers, so in the
Backlog the bar goes inside the first `<td>`, which becomes `relative`.

## Data contracts

None changed. The helper reads `Task.parentKey` and `MacroRow.key`, which is the
trimmed `parentKey` the tasks are grouped by (`web/src/lib/roadmap.ts`).

## Review revision

Two changes asked by the owner after the first review of PR #399.

### A per-project setting, off by default

- Backend: `projects.epic_colors INTEGER NOT NULL DEFAULT 0`, added by
  numbered migration 3 (`internal/db/migrations.go`) and nowhere else: since
  #339 the `CREATE TABLE`, `applyLegacyMigrations` and `lateColumns` describe
  the frozen baseline, which a database already stamped never replays.
  `models.Project.EpicColors` (`json:"epicColors"`), `CreateProjectRequest`
  (`bool`) and `UpdateProjectRequest` (`*bool`, nil leaves it alone); both
  project read paths, the insert and the update carry the column.
- A boolean column rather than a value in `enabled_views` (ADR 0024): the
  colour is a display option, not a view, and folding it into the list of
  views would put it in the sidebar logic that list drives.
- Web: `Project.epicColors`; `epicColorsEnabled(projects, projectId, fallback)`
  in `epicColor.ts` and the `useEpicColors()` hook in `EpicMarker.tsx` answer
  per task, so "all projects" paints each task by its own project. The
  checkbox sits in the project settings, General tab, under the workspace
  views.

### Rendering: a short, thicker bar

The full-height bar on the left edge met the card's rounded corners, whose
radius differs between shapes, and overflowed them. A tinted epic key was tried
next and rejected by the owner, who preferred a bar that is shorter and thicker.

- `EpicBar`: `absolute left-[3px] top-[22%] bottom-[22%] m-0 w-[5px]
  rounded-full`. Inset on every side, it never touches a corner, whatever the
  radius; `m-0` still guards against `space-y-*`. `EpicDot` is removed.
- It goes on every surface of the original plan. Three of them have a left
  padding under 8px (the bar's right edge): the condensed card and the timeline
  list row (`px-1.5`) and the small timeline chip (`px-2`). Each switches to
  `pl-3` only when its bar is shown, so a project with the setting off keeps its
  layout to the pixel.
