# Plan #851 - Macro keys link to the tracker

## Stack

React web client only: `web/src/components/RoadmapView.tsx`,
`web/src/lib/epicAxes.ts`, the French and English locale catalog
`web/src/locales/planning.ts`. Sectile Desktop renders the same component, so
it needs no change of its own. No server change, no migration, no new tracker
call: `MacroMeta.externalUrl` (`web/src/types/index.ts`) already carries the
macro's tracker page, and `MacroRow` (`EpicRow`, `web/src/lib/roadmap.ts`)
already exposes it.

## Pattern

Follow the task key on `TaskCard.tsx` (line 925):

```tsx
{externalUrl ? (
  <a href={externalUrl} target="_blank" rel="noreferrer" onClick={e => e.stopPropagation()} ...>
    {task.key}
  </a>
) : <span ...>{task.key}</span>}
```

A render helper inside `RoadmapView.tsx`, next to `renderMacroRow`, keeps
the four sites identical. It is a plain function rather than a component
declared in the render body, which would remount on every render:

```tsx
// The macro key, as a link to its tracker page when the tracker gives one (#851).
const renderMacroKey = (macroKey: string, url: string | undefined, className: string, style?: React.CSSProperties) =>
  url ? (
    <a href={url} target="_blank" rel="noreferrer" draggable={false}
      onClick={e => e.stopPropagation()}
      onMouseDown={e => e.stopPropagation()}
      title={format(strings.panel.openRemoteTitle, { key: macroKey })}
      className={`${className} hover:underline focus-visible:outline-2 focus-visible:outline-[var(--accent-color)]`}
      style={style}>
      {macroKey}
    </a>
  ) : <span className={className} style={style}>{macroKey}</span>
```

- Each site passes its current classes and inline style unchanged, so colour,
  size and font stay the same (US1.5).
- `stopPropagation` on `click` keeps `onRowClick` from selecting or toggling
  the multi-selection (FR3). On `mousedown` it keeps the row's Shift handler
  from calling `preventDefault`, which would otherwise suppress a Shift click
  on the link; a modifier click on the key opens the link as the browser does.
- `draggable={false}` makes the browser start the drag on the draggable row
  (`rowDragProps`) instead of dragging the link's URL (FR4).
- The anchor's own `title` overrides the row's `strings.grouping.dragTitle`
  tooltip while the key is hovered.

## Sites in `RoadmapView.tsx`

| Site | Current markup | Source of the URL |
| --- | --- | --- |
| `renderMacroRow` (line 1400) | `<span className="text-[11px] font-mono font-bold" style={{ color: 'var(--accent-color)' }}>{row.key}</span>` | `row.externalUrl` |
| `renderCondensedRow` (line 1530) | `<span className="shrink-0 text-[10.5px] font-mono font-bold" ...>{row.key}</span>` | `row.externalUrl` |
| Panel header (line 2046) | `<span className="text-[11px] font-mono font-bold" ...>{selected.key}</span>` | `selected.externalUrl` |
| Seed table (line 3788) | `<span className="font-mono font-bold text-[var(--accent-color)] mr-2">{line.key}</span>` | `line.externalUrl` (new) |

Line numbers are those of `301f4599`.

## Seed table data

`SeedLine` (`web/src/lib/epicAxes.ts`) carries only `key`, `title` and the
proposed axes. Add an optional field, copied by `seedProposals` from the row:

```ts
/** The macro's page on its tracker, absent when the tracker gives none. */
externalUrl?: string
```

`seedProposals` sets it only when `row.externalUrl` is non-empty, so the
existing `epicAxes.test.mjs` expectations on lines without a URL are
unchanged.

## Panel button removal

- Delete the `{selected.externalUrl && (<a ...><ExternalLink /> <span>{strings.panel.link}</span></a>)}`
  block (lines 2086-2098).
- Remove `link` from the `panel` group of both catalogs in
  `web/src/locales/planning.ts` (lines 192 and 703) and from the catalog's
  type if it is declared separately.
- Keep `openRemoteTitle`, now used by the key links.
- Remove the `ExternalLink` import from `RoadmapView.tsx` only if no other use
  remains (`grep` before deleting).

## Tests

- `web/tests/epicAxes.test.mjs`: `seedProposals` copies `externalUrl` when the
  row has one and leaves it out otherwise.
- `web/tests/roadmap-view.browser.mjs` already holds `M-3` with an
  `externalUrl` and `M-4` without. Extend it:
  - the full-row and condensed-row keys of `M-3` are links with the URL,
    `target="_blank"`, `rel="noreferrer"` and the French tooltip; `M-4`'s key
    is not a link (FR1, FR2, FR5);
  - clicking `M-3`'s key while `M-1` is selected leaves `M-1` selected, and a
    Shift click on it leaves the multi-selection unchanged (FR3); intercept
    the popup (`context.on('page')` or `page.route`) so no real tab loads;
  - the panel header key of `M-3` is a link and the panel shows no "Lien"
    button (FR6);
  - the existing assertion at line 104 (`panel().getByRole('link').first()`
    points at the macro URL) keeps passing, since the header key is now the
    panel's first link; tighten it to target the key by name if it becomes
    ambiguous.
- `web/tests/roadmap-epic-axes.browser.mjs` (seed table): a seed line whose
  macro has a URL shows its key as a link, and clicking it changes no checkbox
  (US1.4). Add an `externalUrl` to one fixture macro if none has one.
- Drag: in `roadmap-grouping.browser.mjs`, a drag started on a linked key
  moves the row as a drag on the row does (FR4).
- `tsc`, `oxlint` and the locale catalog test (both catalogs hold the same
  keys) after the string removal.

Browser tests need Playwright from the main checkout (`PLAYWRIGHT_MODULE`) and
run unsandboxed.

## Rejected alternatives

- **An icon next to the key instead of a linked key**: adds a control to
  every row; the task cards already set the "key is the link" convention.
- **Keeping the panel's "Lien" button**: the owner chose to remove it as a
  duplicate (clarification round 2).
- **Looking the URL up from the rows inside the seed table** instead of
  carrying it on `SeedLine`: couples the modal to the row list; the line
  already copies the title the same way.
