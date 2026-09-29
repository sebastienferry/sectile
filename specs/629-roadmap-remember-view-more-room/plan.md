# Plan #629 - Roadmap: remember the view and give the list more room

## Stack

React 19 + TypeScript web app (`web/`), Tailwind classes, `lucide-react`
icons, `node --test` unit tests (`web/tests/*.test.mjs`) and Playwright browser
component tests (`web/tests/*.browser.mjs`). One Go change in `internal/db`
and `internal/models`: a computed field on the macro list, no column, no
migration, no new route.

## Design

### 1. View preferences: one pure module

Create `web/src/lib/roadmapViewPrefs.ts`, built like
`roadmapDisplayMode.ts` (reuse its `StorageLike` and storage resolution; move
`resolveStorage` to a shared export of that file rather than copying it):

```ts
export type RoadmapTab = 'now' | 'next' | 'later' | 'unclassified' | 'hidden'

export const ROADMAP_TAB_KEY = 'sectile_roadmap_tab'
export const ROADMAP_PANEL_EXPANDED_KEY = 'sectile_roadmap_panel_expanded'
export const ROADMAP_PANEL_HIDDEN_KEY = 'sectile_roadmap_panel_hidden'
export const ROADMAP_DESC_OPEN_KEY = 'sectile_roadmap_description_open'
export const ROADMAP_FRAMING_OPEN_KEY = 'sectile_roadmap_framing_open'
export const roadmapSelectedKeyKey = (projectId: string) =>
  `sectile_roadmap_selected_key:${projectId}`

export function loadRoadmapTab(storage?: StorageLike): RoadmapTab        // unknown -> 'now'
export function saveRoadmapTab(tab: RoadmapTab, storage?: StorageLike): void
export function loadRoadmapFlag(key: string, fallback: boolean, storage?: StorageLike): boolean
export function saveRoadmapFlag(key: string, value: boolean, storage?: StorageLike): void
export function loadRoadmapSelectedKey(projectId: string, storage?: StorageLike): string | null
export function saveRoadmapSelectedKey(projectId: string, key: string | null, storage?: StorageLike): void
```

Flags are stored as `'1'`/`'0'`; anything else reads as the fallback. Every
read and write is wrapped so an unavailable storage is a no-op (FR3, US1.9).
`RoadmapTab` replaces the local `HorizonTab` alias if it is the same union;
otherwise `HorizonTab` is imported from here.

The panel width keeps its existing key and code.

### 2. RoadmapView state wiring

In `web/src/components/RoadmapView.tsx`:

- `tab`: initial state `loadRoadmapTab()`. Wrap `setTab` so every caller (tab
  click, search-jump effect) saves (FR4). The `displayMode` effect on `tab`
  stays and so applies the horizon default on restore.
- `selectedKey`: initial `null`; an effect on `currentProject?.id` loads
  `loadRoadmapSelectedKey(projectId)` into it. Wrap `setSelectedKey` so a
  click saves it for the current project; the delete path's
  `setSelectedKey(null)` saves `null` too. The existing fallback
  `visibleRows.find(...) || visibleRows[0]` is untouched, and it never writes,
  so a remembered key that is momentarily invisible is kept (FR3, US1.4).
- `isPanelExpanded`, `isDescExpanded`, `isFramingExpanded`: initial state from
  `loadRoadmapFlag`, setters that save.
- New `isPanelHidden` from `loadRoadmapFlag(ROADMAP_PANEL_HIDDEN_KEY, false)`.
  Three handlers carry the exclusivity rule (FR5):
  - `hidePanel()`: expanded false, hidden true;
  - `showPanel()`: hidden false;
  - `toggleExpanded()`: hidden false, expanded toggled. The existing expand
    button calls it.
- A small `usePersistedFlag(key, fallback)` hook local to the file may factor
  the load/save pairs; not required.

### 3. Layout

Let `panelShown = Boolean(selected) && !isPanelHidden` and
`expandedHere = panelShown && isPanelExpanded`.

- The toolbar block (from the "Barre d'outils" comment, around line 784,
  through the search, filter chips and sprint strip that precede the list) is
  rendered only when `!expandedHere` (FR6). Check every sibling rendered above
  the split container (pending push notice, hidden-matches hint) and hide
  those too.
- List: hidden when `expandedHere` (already the case with
  `isPanelExpanded`; switch its condition to `expandedHere`).
- Split handle: rendered when `panelShown && !isPanelExpanded`.
- Panel `aside`: rendered when `panelShown`; width logic unchanged.
- Rail: when `selected && isPanelHidden`, a `button` `w-6` on the right edge,
  `PanelRightOpen` icon, `title` and `aria-label` = `strings.panel.show`,
  `onClick={showPanel}`.
- Panel header: a `PanelRightClose` button "Masquer le panneau" right after
  the expand button, `onClick={hidePanel}`.

Because `expandedHere` needs `selected`, US3.3 (no epic left) brings the
toolbar back by construction.

### 4. Full-screen Markdown editor

`web/src/components/Markdown.tsx`, `MarkdownEditor` gets an optional prop
`maximizable?: boolean` (default false, FR8) and an optional
`maximizeTitle?: string` used as the overlay heading.

- When `maximizable`, a `Maximize2` icon button sits in the editor's tab bar
  (next to Écrire / Aperçu), `title` = `strings.maximize`.
- Clicking it sets local `isMaximized`. The overlay is rendered through
  `createPortal(…, document.body)`: a fixed full-viewport layer
  (`fixed inset-0 z-[…]` above the app's modals), a header with the heading
  and a `Minimize2` "Réduire" button, and a body split in two columns: the
  same toolbar + `textarea` bound to `value`/`onChange`, and the rendered
  preview of `value` using the component already used by the Aperçu tab.
- Escape: `useEscapeKey` listens on `document` in the capture phase
  (`web/src/hooks/useEscapeKey.ts:25`), so a listener on the overlay element
  would run too late. Register the overlay's handler on `window` in the
  capture phase (it runs before `document`), close the overlay there and call
  `stopPropagation()`, so the roadmap dialogs' `useEscapeKey` and any modal
  behind do not react.
- Focus moves to the textarea on open and back to the maximize button on
  close.
- The snippet toolbar code (`applySnippet`) works on a textarea ref: give the
  overlay its own ref and let `applySnippet` target the active one.

In `RoadmapView.tsx`, pass `maximizable` and the section heading to the two
framing editors only (Description, Framing notes). Their `onChange` already
sets the draft and the dirty flag, so US4.4 holds with no change.

### 5. Epic address from the server

`internal/models/models.go`: add to `MacroMeta`

```go
// ExternalURL is the macro's own page on its tracker, computed when the list
// is read. Empty when the tracker gives none.
ExternalURL string `json:"externalUrl,omitempty"`
```

`internal/db/macros.go`, `GetProjectMacros`:

- GitHub milestone projects: while iterating the milestone list, record
  `M-<n>` → `https://github.com/<CleanGithubRepo(proj.GithubRepo)>/milestone/<n>`
  in a map. Only keys present in the list get an address (AC5): a local
  `M-<n>` created when the milestone write failed has no milestone behind it.
  Add `HTMLURL string \`json:"html_url"\`` to `trackerapi.GithubMilestoneItem`
  and prefer it when non-empty (GitHub Enterprise hosts).
- Other trackers: after the scan, for macros without an address, look up a
  task of the same project whose key equals the macro key (a Jira epic is
  synced as a task of type Epic, kept out of the board by
  `containerIssueTypes`) and use `computeExternalURLUnsafe` on it. One query
  `SELECT … FROM tasks WHERE project_id = ? AND key IN (…)` for all remaining
  keys, not one per macro.
- GitLab: macros are labels (`parent:`/`macro:`), with no page of their own;
  they get an address only through the same task lookup, when a task carries
  the key.
- The field is computed, never stored: no column, no migration, no change to
  `saveMacroMetaFull` or the replay fixtures.

`web/src/types/index.ts`: `MacroMeta.externalUrl?: string`.
`web/src/lib/roadmap.ts`: `MacroRow` exposes `externalUrl` from `meta`.

### 6. Copy and open

In the panel header of `RoadmapView.tsx`:

- Replace `selected.tasks[0]?.externalUrl` by `selected.externalUrl` for the
  "Ouvrir dans le tracker" link, rendered only when set (FR10).
- New "Copier le lien" button (`Copy` icon, turns to `Check` for ~1.8 s after
  a success). Payload: `selected.externalUrl || \`${key}: ${title}\``.
  `navigator.clipboard.writeText`; missing API or rejection → error toast
  `strings.panel.copyFailed` with description
  `format(strings.panel.copyByHand, { text })`, longer duration (FR11).
  Success toast title `linkCopied` or `refCopied`, description the payload.
  Tooltip: `copyLinkTitle` or `copyRefTitle` with the key.
- The payload builder is a pure function in `web/src/lib/roadmap.ts`
  (`macroCopyPayload(row)` → `{ text, kind: 'link' | 'ref' }`) for unit tests.

### 7. Strings

`web/src/locales/planning.ts`, `planning.roadmap.panel` (fr, en, and the
type): `hide`, `show`, `copyLink`, `copyLinkTitle`, `copyRefTitle`,
`linkCopied`, `refCopied`, `copyFailed`, `copyByHand`.
`taskDetail.markdown` (its locale file): `maximize`, `restore`.
French texts: "Masquer le panneau", "Afficher le panneau", "Copier le lien",
"Copier le lien de {key}", "Copier la référence de {key} (pas d'adresse sur le
tracker)", "Lien copié", "Référence copiée", "Copie impossible", "Copiez-le à
la main : {text}", "Agrandir l'éditeur", "Réduire". English equivalents. Run
the translation checks of `docs/web-translation-checks.md`.

## Target files

| File | Change |
| --- | --- |
| `web/src/lib/roadmapDisplayMode.ts` | export the storage resolver |
| `web/src/lib/roadmapViewPrefs.ts` | new: tab, flags, selected key |
| `web/src/lib/roadmap.ts` | `externalUrl` on rows, `macroCopyPayload` |
| `web/src/components/RoadmapView.tsx` | persisted state, hide/show, chrome, copy, link, maximizable editors |
| `web/src/components/Markdown.tsx` | `maximizable` full-screen overlay |
| `web/src/types/index.ts` | `MacroMeta.externalUrl` |
| `web/src/locales/planning.ts`, markdown locale | strings fr/en |
| `internal/models/models.go` | `MacroMeta.ExternalURL` |
| `internal/trackerapi/github.go` | `GithubMilestoneItem.HTMLURL` |
| `internal/db/macros.go` | compute the address in `GetProjectMacros` |
| `CHANGELOG.md` | one `Added` line |

## Tests

- `web/tests/roadmapViewPrefs.test.mjs`: defaults with empty storage; unknown
  tab → `now`; flags round-trip and garbage → fallback; selected key per
  project (A and B independent); throwing storage → defaults, no throw.
- `web/tests/roadmap.test.mjs` (new; no unit test file covers
  `web/src/lib/roadmap.ts` yet): `macroCopyPayload` with and without address.
- `web/tests/roadmap-view.browser.mjs` (new, on the React fixture harness):
  tab and selection restored after remount; hide keeps hidden across epic
  selection, rail brings back; hide clears expanded; expanded removes the tab
  bar and toolbar; copy with a stubbed clipboard (success, rejection);
  "Ouvrir dans le tracker" absent without address.
- `web/tests/markdown-editor.browser.mjs` (new or existing): no button without
  `maximizable`; overlay opens, preview follows typing, Escape closes only the
  overlay (a parent Escape listener is not called).
- `internal/db/macros_test.go`: a GitHub project with a stubbed milestone list
  returns the milestone address for listed keys and none for a local `M-` key;
  a Jira-style project returns the epic task's browse URL; a macro with no
  matching task has none.

## Risks

- Escape ordering between the overlay and `useEscapeKey` users; covered by the
  browser test.
- The toolbar block is long and interleaved with conditionals; hiding it must
  not unmount state the list needs (all state lives in the component, so
  unmounting the toolbar JSX is safe).

## Implementation notes

- The storage keys are exported as `ROADMAP_*_STORAGE_KEY`, and the
  Description flag is stored under `sectile_roadmap_description_open`.
- The selected macro is not reloaded from an effect on project change: the
  state holds `{ projectId, key }`, and a render for another project reads that
  project's stored key directly. This avoids a `set-state-in-effect` lint
  warning and one extra render.
- `MarkdownEditor` resolves its two textareas by a surface name (`inline`,
  `maximized`) inside the event handlers, rather than passing ref objects to
  render helpers, which the `react(refs)` lint rule flags.
