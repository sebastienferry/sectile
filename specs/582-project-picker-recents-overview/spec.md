# Specification #582 - Project picker with recents and a project overview

- Ticket: https://github.com/sebastienferry/sectile/issues/582
- Branch: `feat/582`
- Clarification: `docs/clarifications/582.md` (rounds 1 and 2, confirmed by
  the owner)
- Mockup: https://claude.ai/artifact/UTRuVDV8kWDcSSbv9EsM8g
- Framework: Spec Kit

## Summary

The web sidebar's project picker keeps a short menu: the projects recently
opened in this browser and the favorites. Search covers every project, with
matches on more fields and a bounded result list. A new "Browse projects…"
entry opens an overview of every project, where they can be filtered by text
and tracker. The picker can be used from the keyboard.

## Scope

In scope: the project picker of the expanded web sidebar, a new project
overview dialog, the browser-local history of opened projects, and the strings
they add to both locales.

Out of scope:

- Any API or server change: the history is client-side.
- A global shortcut to open the picker.
- The desktop app's project picker.
- The collapsed sidebar's project button, which keeps its current behavior.

## Definitions

- **Picker**: the dropdown menu opened from the project switcher at the top of
  the expanded sidebar.
- **Favorite**: a project whose `bookmarked` flag is set.
- **Open history**: the list, kept in this browser, of projects that were
  opened and when, most recent first.
- **Opening a project**: selecting a specific project as the current one,
  from the picker, the overview, the command palette or a task link. Selecting
  "All projects" and restoring the last selection when the app starts are not
  openings.
- **Recent**: a project in the open history that is not a favorite.
- **Overview**: the project overview dialog.
- **Searchable fields**: name, slug, description, repositories, tracker label
  (GitHub, GitLab, Jira, Local) and Jira project key.
- **Match**: a project for which a searchable field contains the query. Case
  and accents are ignored.

## User stories (prioritised)

### US1 - Short menu with recents and favorites (P1)

As a user, when I open the picker, I find the projects I work on without
typing and without a long list.

**Acceptance scenarios**

1. **Given** an open history of A, B, C, D (A most recent) with none of them
   a favorite, and favorites F1 and F2, **when** I open the picker, **then**
   it shows a "Recent" section with A, B, C in that order, then a "Favorites"
   section with F1 and F2 in A–Z order, and no other project.
2. **Given** B is a favorite and in the open history, **when** I open the
   picker with an empty search, **then** B is listed only under "Favorites",
   and the next non-favorite project in the history takes its place in
   "Recent".
3. **Given** I remove B from the favorites, **when** the picker is shown,
   **then** B can appear in "Recent" again, according to its place in the
   history.
4. **Given** an empty open history, **when** I open the picker, **then** the
   "Recent" section is not shown.
5. **Given** no favorite and an empty open history, **when** I open the
   picker, **then** it shows no project row and no "No favorite project"
   message. The "All projects", "Browse projects…" and "New project…" entries
   are shown.
6. **Given** 9 favorites, **when** I open the picker with an empty search,
   **then** it lists the first 6 favorites in A–Z order, followed by a
   "3 more favorites in the overview →" link, and the link opens the overview
   with no filter.
7. **Given** any number of projects, favorites and history entries, **when**
   the picker is open, **then** it never shows a horizontal or vertical
   scrollbar, and long names, repositories and descriptions end with an
   ellipsis.

### US2 - The history of opened projects (P1)

As a user, the "Recent" section follows what I actually opened in this
browser, including after a reload.

**Acceptance scenarios**

1. **Given** any way of opening project P (picker, overview, command palette,
   task link), **when** P is opened, **then** P moves to the top of the open
   history, with the current time.
2. **Given** I select "All projects", **when** the history is read, **then**
   it has not changed.
3. **Given** I reload the page, **when** I open the picker, **then** "Recent"
   shows the same projects as before the reload, and restoring the last
   selection at startup has not changed the history.
4. **Given** the history names a project that no longer exists, **when** the
   picker or the overview reads it, **then** that entry is ignored.
5. **Given** browser storage is unavailable or holds unreadable data, **when**
   I use the picker, **then** it works without an error, and the history lives
   only for the current page session.

### US3 - Search across every project (P1)

As a user, I find a project by a part of its name, description, repository or
tracker.

**Acceptance scenarios**

1. **Given** a query, **when** I type it, **then** the picker lists the
   matching favorites first, in A–Z order, under "Favorites", then the other
   matches under "Other projects", ordered by the open history (most recent
   first) and then A–Z. "Recent" is not shown while a query is typed.
2. **Given** the query `equipe`, **when** a project is described as
   "Équipe paiement", **then** it matches.
3. **Given** the query `gitlab`, **when** a project's tracker is GitLab,
   **then** it matches. **Given** the query `PE`, **when** a project's Jira
   project key is `PE`, **then** it matches.
4. **Given** a query matching a repository `git@github.com:acme/billing.git`
   (for example `acme/bil`), **when** the results are shown, **then** the
   project matches.
5. **Given** a match only in the description, **when** the row is shown,
   **then** its second line is an excerpt of the description with the matched
   text highlighted. For any other match, the second line shows the tracker and
   the repository, and the matched text is highlighted in the name, the
   repository or the tracker.
6. **Given** 10 matches, 8 of them favorites, **when** the results are shown,
   **then** exactly 6 rows are listed (the first 6 favorites), followed by a
   "4 more matches in the overview →" link. The link opens the overview with
   its text filter set to the query.
7. **Given** 6 matches or fewer, **when** the results are shown, **then** no
   "more matches" link is shown.
8. **Given** a query with no match, **when** the results are shown, **then**
   the picker says that no project contains the query and that search covers
   the name, slug, description, repository and tracker.

### US4 - Keyboard use (P2)

As a keyboard user, I pick a project without the mouse once the picker is
open.

**Acceptance scenarios**

1. **Given** the picker is closed, **when** I open it, **then** its search
   field has the focus.
2. **Given** the picker is open, **when** I press ↓ or ↑, **then** the
   highlight moves through the project rows and the "more" link, and wraps
   around at either end. A highlighted row is visible and announced as the
   active option.
3. **Given** a highlighted row, **when** I press Enter, **then** that project
   opens and the picker closes. **Given** a highlighted "more" link, **when**
   I press Enter, **then** the overview opens as that link would.
4. **Given** no highlighted row and a non-empty query with at least one match,
   **when** I press Enter, **then** the first listed project opens.
5. **Given** a non-empty query, **when** I press Esc, **then** the query is
   cleared and the picker stays open. **Given** an empty query, **when** I
   press Esc, **then** the picker closes and the switcher button gets the
   focus.
6. **Given** the picker is open, **when** I press Tab, **then** the focus
   reaches the "All projects", "Browse projects…" and "New project…" entries.
7. **Given** focus outside any text field, **when** I press `/`, **then** the
   global search gets the focus, as it does today.

### US5 - Project overview (P1)

As a user who does not remember a project's name, I browse every project and
open one.

**Acceptance scenarios**

1. **Given** the picker is open, **when** I choose "Browse projects…"
   ("Parcourir les projets…" in French), which shows the number of projects,
   **then** the picker closes and the overview opens over the current view.
2. **Given** the overview is open, **then** it shows a card for every project:
   icon, name, description, tracker, repository, number of tasks, and
   "opened N ago" when the project is in the open history. It shows nothing in
   place of that last item for a project never opened in this browser.
3. **Given** the overview, **then** cards list favorites first in A–Z order,
   then the other projects ordered by the open history and then A–Z.
4. **Given** I type in the overview's text filter, **when** the cards are
   shown, **then** only projects matching on the searchable fields remain.
5. **Given** the tracker filter (All, GitHub, GitLab, Jira, Local), each with
   its project count, **when** I choose one, **then** only projects of that
   tracker remain. The tracker filter and the text filter combine.
6. **Given** filters that match no project, **then** the overview says that
   no project matches.
7. **Given** a card, **when** I click its star, **then** the project is added
   to or removed from the favorites, and the card and the picker reflect it,
   without opening the project.
8. **Given** a card, **when** I click it, or focus it and press Enter or
   Space, **then** that project opens and the overview closes.
9. **Given** the overview is open, **when** I press Esc, click its close
   button, or complete a click outside the dialog, **then** it closes and the
   current project does not change.
10. **Given** the overview opens, **then** its text filter has the focus.

### US6 - Unchanged entries (P1)

1. **Given** the picker, **then** "All projects" keeps its label, its task
   count and its behavior: it shows the tasks of every project on the board.
2. **Given** the picker, **then** "New project…" and each row's favorite star,
   task count and configure button keep their current behavior.

## Functional requirements

- **FR1** The picker shows, with an empty search, at most 3 recents and at
  most 6 favorites, and no other project row (US1).
- **FR2** A project never appears in two sections of the picker (US1).
- **FR3** Every opening of a specific project records it at the top of the
  open history with the time of the opening. Selecting "All projects" and the
  startup restore record nothing (US2).
- **FR4** The open history persists in the browser across reloads, holds at
  most 50 projects, ignores unknown projects, and falls back to memory when
  storage fails (US2).
- **FR5** Search matches the searchable fields, ignoring case and accents, the
  same way the board's free-text search folds text (US3).
- **FR6** The picker lists at most 6 matching rows. When matches are left out,
  a link says how many and opens the overview filtered by the query (US3).
- **FR7** The picker never scrolls, horizontally or vertically (US1).
- **FR8** Once open, the picker can be operated entirely from the keyboard,
  and existing global shortcuts keep their behavior (US4).
- **FR9** The overview lists every project and can be filtered by text and by
  tracker. It toggles favorites, and it opens a project (US5).
- **FR10** Every new string exists in French and in English (all US).
- **FR11** `CHANGELOG.md` gets a line under `[Unreleased]` → `Added` or
  `Changed` for the new picker and overview.

## Edge cases

- A project whose description is empty: the card's description area is empty,
  and search never matches on it.
- A project with no repository: the row and the card show the slug in place
  of the repository.
- A local project (tracker `local`): the tracker is labelled Local.
- The current project is highlighted in whichever section lists it, as today.
- A favorite toggled from the picker or the overview moves between sections at
  once, without closing the menu.
- An opening recorded in another tab appears on this tab's next read of the
  history. Live synchronization between tabs is not required.

## Success criteria

- Every project can be reached from the picker without typing, in two
  actions: "Browse projects…", then its card.
- The ticket's acceptance criteria, as adjusted by the clarification, all
  hold. The overview cards show the last opening in this browser, not a server
  activity date.
