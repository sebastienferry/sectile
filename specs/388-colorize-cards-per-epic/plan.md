# Plan — colour board cards per epic

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
