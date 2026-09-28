# Plan #582 - Project picker with recents and a project overview

## Stack

- Web client only: React + TypeScript + Tailwind (`web/`), state in
  `web/src/context/AppContext.tsx`, strings in `web/src/locales/shell.ts`.
- Tests: `node --test tests/*.test.mjs` for pure helpers, and a Playwright
  `tests/*.browser.mjs` regression on the real App with a faked API, in the
  style of `tests/sidebar-shortcut.browser.mjs`.
- No server, API or desktop change.

## Architecture

```
AppContext
  projectHistory: ProjectOpening[]        <- readProjectHistory() at startup
  setSelectedProjectId(id)                -> id !== 'all' ? recordProjectOpening
  isProjectOverviewOpen / projectOverviewQuery
  openProjectOverview(query?) / closeProjectOverview()

lib/projectHistory.ts   (pure + storage wrapper)
lib/projectPicker.ts    (pure: matching, sections, ordering, highlight, labels)

components/Sidebar.tsx
  -> components/ProjectPicker.tsx        the dropdown (menu, search, keyboard)
App.tsx
  -> components/ProjectOverviewModal.tsx the overview dialog
```

The picker's markup moves out of `Sidebar.tsx` (which is 1,073 lines long)
into `ProjectPicker.tsx`. The switcher button, the outside-click close and the
collapsed-sidebar button stay in `Sidebar.tsx`.

## Data contracts

### Open history (`lib/projectHistory.ts`)

```ts
export interface ProjectOpening { id: string; openedAt: string } // ISO 8601
export const PROJECT_HISTORY_KEY = 'sectile_recent_project_ids'
export const PROJECT_HISTORY_LIMIT = 50

/** Parses the stored JSON; anything malformed yields [] or drops the bad entries. */
export function parseProjectHistory(raw: string | null): ProjectOpening[]
/** Moves id to the front with openedAt = now, dedupes, caps at the limit. Pure. */
export function recordProjectOpening(history: ProjectOpening[], id: string, now: Date): ProjectOpening[]
/** localStorage read/write in try/catch. A failed write leaves the in-memory state as the only copy. */
export function readProjectHistory(): ProjectOpening[]
export function writeProjectHistory(history: ProjectOpening[]): void
```

- Storage value: a JSON array of `{ "id": "...", "openedAt": "2026-09-28T09:00:00.000Z" }`,
  most recent first. An entry without a string `id`, or whose `openedAt` is not
  a valid date, is dropped at parse time.
- Unknown ids are kept in storage, since the project list may not be loaded
  yet. They are filtered out against `projects` wherever the history is
  displayed.

### Picker model (`lib/projectPicker.ts`)

```ts
export type MatchField = 'name' | 'slug' | 'description' | 'repository' | 'tracker'
export const PICKER_RECENT_LIMIT = 3
export const PICKER_FAVORITE_LIMIT = 6
export const PICKER_RESULT_LIMIT = 6

/** Short repository labels: every repositories[].url (gitRemoteUrl first), else githubRepo / gitlabProject. */
export function projectRepositories(p: Project): string[]
/** Shortens a remote to owner/name: the path of repositoryIdentity(), without the host. */
export function repositoryLabel(remote: string): string
/** GitHub | GitLab | Jira | Local, from issueTracker. */
export function trackerLabel(t: IssueTracker): string
/** First matching field in the order name, slug, description, repository, tracker (label, then jiraProject); null if none. */
export function matchProject(p: Project, query: string): { field: MatchField; text: string } | null
/** Favorites first A–Z; others by history recency then A–Z (localeCompare, UI language). */
export function orderProjects(projects: Project[], history: ProjectOpening[], locale: string): Project[]

export interface PickerModel {
  recent: Project[]            // empty query only
  favorites: Project[]
  others: Project[]            // query only
  hiddenFavorites: number      // empty query: favorites beyond the limit
  hiddenMatches: number        // query: matches beyond PICKER_RESULT_LIMIT
}
export function pickerModel(projects: Project[], history: ProjectOpening[], query: string, locale: string): PickerModel

/** Overview list: every project, ordered, filtered by text (matchProject) and tracker. */
export function overviewProjects(projects: Project[], history: ProjectOpening[], query: string,
  tracker: IssueTracker | 'all', locale: string): Project[]

/** Splits text into [before, match, after] on the folded query; the match maps back to the original characters. */
export function highlightParts(text: string, query: string): { text: string; match: boolean }[]
/** Description excerpt of about 60 characters, starting a little before the match, with a leading ellipsis when cut. */
export function descriptionExcerpt(text: string, query: string): string
```

- Folding reuses `foldForSearch` from `lib/searchFold.ts`, so the picker
  agrees with the board's search (#447). `highlightParts` folds the text one
  character at a time to map folded offsets back to the original string, since
  stripping combining marks changes lengths.
- Result cap: favorites are placed before others, then the combined list is
  cut at 6. `hiddenMatches = total - shown`.
- Recent: history filtered to known, non-favorite projects, first 3.

### AppContext additions

```ts
projectHistory: ProjectOpening[]
isProjectOverviewOpen: boolean
projectOverviewQuery: string
openProjectOverview: (query?: string) => void
closeProjectOverview: () => void
```

- `setSelectedProjectId(id)`: when `id !== 'all'`, it records the opening in
  the state and in storage. The startup auto-selection and
  `leaveUnavailableView` call `setSelectedProjectIdState` directly, so they
  record nothing (FR3). The command palette and task deep links already call
  `setSelectedProjectId`, so they record without any change.
- A `storage` event listener on `PROJECT_HISTORY_KEY` is not added: the
  specification does not require live sync between tabs.

## UI

### ProjectPicker

- The width stays `w-72`. The `max-h-48 overflow-y-auto` list is removed,
  since the caps bound the height (at most 9 rows, or 6 while searching, plus
  section labels and actions). Every text line uses `truncate` and
  `min-w-0`. No element sets `overflow-*: auto/scroll`.
- Search input: `role="combobox"`, `aria-expanded`, `aria-controls` pointing
  to the listbox, and `aria-activedescendant` pointing to the highlighted
  option. It is focused on open (`autoFocus`, or an effect keyed on open).
  The placeholder names what search covers.
- The row list is `role="listbox"`. Each project row and the "more" link is
  `role="option"` with a stable `id`. The highlight index lives in the picker
  and resets when the query changes.
- Keys on the input: ArrowDown/ArrowUp wrap. Enter opens the highlighted
  option, or the first project when nothing is highlighted and the query is
  not blank. Escape clears a non-empty query, and otherwise closes and focuses
  the switcher button (a ref passed from `Sidebar`). Handled keys call
  `preventDefault` and `stopPropagation`, so the global handler in
  `AppContext` never sees them. `/` typed in the input stays text, as today:
  the global handler ignores active inputs.
- Row: the icon badge, the highlighted name, and a second line. The second
  line is a description excerpt for a description match, and otherwise
  `tracker · repository` (the slug when there is no repository), with the
  match highlighted. The star, the task count and the configure button are
  kept as they are.
- Footer entries: "All projects" (unchanged), "Browse projects…" with
  `projects.length`, and "New project…".
- "N more favorites / matches in the overview →" calls
  `openProjectOverview(query)` and closes the picker.

### ProjectOverviewModal

- It is rendered in `App.tsx` next to `ProjectModal`. It uses the backdrop and
  dialog pattern of `BoardViewModal`: `useBackdropDismiss`, `role="dialog"`,
  `aria-modal="true"` (which also silences global shortcuts through the
  `modalOpen` check), and Esc to close.
- Header: title, text filter (focused on open, initialized from
  `projectOverviewQuery`), tracker chips with counts, and a close button.
- Body: a responsive card grid (`grid-cols-[repeat(auto-fill,minmax(230px,1fr))]`)
  that scrolls inside the dialog. Only the menu is bound by the no-scroll
  rule.
- Card: a `button`-like element (`role="button"`, `tabIndex=0`, Enter/Space)
  showing the badge, the name, a 2-line clamped description,
  `tracker · repository`, the task count, and "opened {elapsed}" from
  `shortElapsed(openedAt, t.shell.elapsed)` when the project is in the history.
  The star button calls `toggleProjectBookmark` and stops propagation.
- Opening a card calls `setSelectedProjectId(p.id)`, then
  `closeProjectOverview()`.

### Strings (`web/src/locales/shell.ts`, both locales, under `projectPicker`)

| Key | fr | en |
| --- | --- | --- |
| `recent` | Récents | Recent |
| `searchPlaceholder` (reworded) | Nom, description, dépôt… | Name, description, repository… |
| `noMatchFor` | Aucun projet ne contient « {query} ». | No project contains "{query}". |
| `searchCovers` | La recherche porte sur le nom, le slug, la description, le dépôt et le tracker. | Search covers the name, slug, description, repository and tracker. |
| `moreMatches` (plural) | {count} autre(s) résultat(s) dans la vue d'ensemble → | {count} more match(es) in the overview → |
| `moreFavorites` (plural) | {count} autre(s) favori(s) dans la vue d'ensemble → | {count} more favorite(s) in the overview → |
| `browse` | Parcourir les projets… | Browse projects… |
| `overviewTitle` | Vue d'ensemble des projets | Project overview |
| `overviewFilter` | Filtrer… | Filter… |
| `overviewAllTrackers` | Tous | All |
| `overviewEmpty` | Aucun projet ne correspond. | No project matches. |
| `openedAgo` | ouvert il y a {elapsed} | opened {elapsed} ago |
| `close` | Fermer | Close |
| `keyboardHint` | ↑ ↓ naviguer · Entrée ouvrir · Échap fermer | ↑ ↓ move · Enter open · Esc close |

`noFavorite` is removed, since nothing uses it any more. The plural forms use
the existing `{ one, other }` shape read by `plural()`. The exact wording may
be polished during implementation, as long as both locales carry every key.

## Target files

| File | Change |
| --- | --- |
| `web/src/lib/projectHistory.ts` | new |
| `web/src/lib/projectPicker.ts` | new |
| `web/src/components/ProjectPicker.tsx` | new, the dropdown extracted from `Sidebar.tsx` |
| `web/src/components/ProjectOverviewModal.tsx` | new |
| `web/src/components/Sidebar.tsx` | uses `ProjectPicker`, removes the inline menu and `searchBookmarked` / `searchOthers` |
| `web/src/context/AppContext.tsx` | history state and recording, overview state |
| `web/src/App.tsx` | renders `ProjectOverviewModal` |
| `web/src/locales/shell.ts` | new keys in `fr` and `en` |
| `web/tests/projectHistory.test.mjs` | new |
| `web/tests/projectPicker.test.mjs` | new |
| `web/tests/shellCatalog.test.mjs` | new English strings asserted |
| `web/tests/project-picker.browser.mjs` | new Playwright regression |
| `CHANGELOG.md` | one `Changed` line under `[Unreleased]` |

## Rejected alternatives

- **Overview as a new `ViewMode`**: rejected in clarification (3A). It would
  touch routing, the view persistence and the sidebar navigation for a
  selection dialog.
- **Server activity date**: rejected (1A). The API is out of scope, and
  `updatedAt` only dates configuration changes.
- **Keeping a scrollable list with a taller `max-h`**: breaks the no-scrollbar
  criterion, so the caps (2A) replace it.
- **Recording openings in each caller** (the picker, the palette, the deep
  link): easy to miss one. The single hook in `setSelectedProjectId` covers all
  of them.

## Risks

- `setSelectedProjectId` has other callers: `AppContext.tsx:1747` passes
  `'all'`, so it records nothing, and the palette records by design. Check
  every caller once more during implementation.
- On very short windows (under about 560 px of height), the 9-row menu may
  reach the bottom of the window. The menu itself still never scrolls. This is
  accepted, as in 2A.
