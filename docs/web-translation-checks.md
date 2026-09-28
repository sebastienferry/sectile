# Web translation checks

The web interface speaks French or English, chosen in **Profile → Appearance →
Language**. This page is the manual check matrix to replay before releasing a
change that touches interface text, and the rules the automated checks
enforce. It was written for the translation audit of 2026-09-26 (#526 to #534,
with #455 for quick-add).

## What must follow the language, and what must not

| Follows the UI language | Never translated |
| --- | --- |
| Labels, headings, buttons, menus | Task titles, descriptions, comments |
| Tooltips (`title`), accessible names (`aria-label`), placeholders | Tracker column names, statuses, labels, issue types |
| Confirmations, empty and loading states, toasts | Project, team, member, sprint and provider names |
| Counts ("1 task", "3 tasks") and dates | Keys (`#526`), identifiers, URLs, command lines |
| Sectile's fallback headings ("Unclassified") | Skill Markdown, prompts, AI and command output |
| Known server activity messages (ADR 0035) | Raw server errors, quoted verbatim |

The page language (`<html lang>`) and the tab title follow the UI language too.

## How the language is chosen

- Signed in: the personal setting, applied at once when changed, and
  remembered by the browser.
- Signed out: the language this browser last used; otherwise the browser's
  language (French when it starts with `fr`, English for anything else). The
  sign-in screen has a French/English switch.
- Dates, times and numbers use the UI language (`fr-FR` or `en-US`), never the
  browser's. A date without a time (a sprint start) is a calendar day and
  never moves with the time zone; a timestamp is shown in the viewer's time
  zone. A missing or unreadable date shows `-`.

## Automated checks

Run from `web/`:

- `npm test` includes `translationCatalog.test.mjs` (same keys in both
  languages, no empty value, same `{placeholders}`), `i18n.test.mjs`
  (interpolation, plural forms for 0, 1 and many, dates in two time zones,
  signed-out language), one catalog test per surface, and
  `activityText.test.mjs` (server activity templates).
- `node tests/i18n-shell.browser.mjs` renders representative components with
  the English catalog and checks their accessible names, next to the French
  harnesses that already exist (see `tests/browserRoot.mjs` for Playwright).

## Manual matrix

Replay each row in English, then in French, at a normal width and at a narrow
width (about 400 px, or the display scale at 125 %). Switch the language from
the profile while the screen is open: nothing may stay in the previous
language.

| Surface | Where | What to look at |
| --- | --- | --- |
| Shell | Sidebar, project picker, header, pinned bar, status bar, display scale menu, command palette (`Cmd+K`) | Entries, tooltips, favorites, "New project", counts |
| Board and backlog | Board and List views of a project | Column actions, hide Done, add a task, card action menu, batch selection count, filters and their placeholders; tracker column names unchanged |
| Quick-add (#455) | "Add task" | Labels, placeholders, validation |
| Task detail | A task, in the drawer and in the centered modal | Fields and lookups, type fallback, copy feedback, specification view and full screen, rewrite preview, PR actions, comments and their dates, clone dialog |
| Project configuration | Project settings, every tab, and a new-project form (do not save) | Tabs, provider descriptions, repository help, optional views, execution policy, column editor, delete confirmation |
| Planning | Roadmap, Triage, Team (enable them in the project's optional views) | Tabs, horizons (NOW / NEXT / FUTURE in both languages), filters, empty states, selection counts, team summaries |
| Sprints | Timeline on a test project | Dates, closed sprints toggle, create/edit/start/close/reopen/delete dialogs, unfinished-task counts |
| Skills | Skills view of a project | Execution modes and their help, custom/divergent/missing indicators, editor feedback, update date |
| Activities and sync | Activities, Sync, a failed read (stop the server briefly) | Activity labels and dates, known server messages, degraded banner and toasts in one language, raw error quoted |
| Sign-in and profile | Signed-out screen, Profile (all tabs), API keys, workstations, MCP connection | Language switch, notices, errors, expiry and copy feedback |
| Admin | Admin view, users panel | Stats, last activity, user actions |

For each row, also check that the tab title and `document.documentElement.lang`
match the language (browser console).
