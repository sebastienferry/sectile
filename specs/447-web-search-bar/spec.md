# Search that ignores case and accents

Scope restated from the clarification (`docs/clarifications/447.md`, three
rounds, confirmed by the owner). Implementation choices are in `plan.md`.

## User stories

### P1: The header search finds a ticket however it is typed

As someone looking for a ticket from the search bar of the web header, I want
`bug`, `Bug` and `BUG` to return the same tickets, and `equipe`, `Equipe` and
`ÉQUIPE` to find a ticket titled `Équipe`, so that I do not have to remember
how the ticket's author capitalised or accented a word.

### P2: The activities search answers the same way

As someone searching the activities view, I want the same rule: case and
accents do not matter, so that a run is found by the words of its summary
however they are spelt.

### P3: Every view agrees with the board

As someone switching between the board, the roadmap, triage and the filter
pickers with the same text in the search bar, I want every view to keep the
same tickets, so that a view that filters in the browser does not hide what the
board shows.

## Functional requirements

1. **Case is ignored.** A search term matches a text that differs from it only
   in the case of its letters. `bug`, `Bug` and `BUG` return the same results.
2. **Diacritics are ignored.** A search term matches a text that differs from it
   only by accents or other diacritics on Latin letters. `equipe`, `Equipe`,
   `équipe` and `ÉQUIPE` all find `Équipe`, and `Équipe` finds `equipe`.
   1 and 2 combine: any mix of case and accents on either side matches.
3. **Substring match is kept.** A search still matches anywhere inside a field,
   as today; no ranking, no fuzzy or typo-tolerant match.
4. **Fields searched are unchanged.**
   - Header search on tickets: key, title, description, labels, assignee, parent
     key, parent title.
   - Activities search on the server: skill name, summary, output, task key,
     task title.
5. **Characters typed are literal.** `%`, `_` and `!` in the search text match
   those characters only: `50%` finds a ticket containing `50%` and not one
   containing `500`; `a_b` does not find `axb`.
6. **Engine.** Requirements 1, 2 and 5 hold on PostgreSQL, which the shared
   server runs. On SQLite, the search keeps its current behaviour (case ignored
   for A-Z only, accents significant); SQLite is to be retired and is not a
   requirement of this ticket. Requirement 5 applies on SQLite too.
7. **Browser-side searches follow the same rule.** The searches that filter in
   the browser ignore case and diacritics as well:
   - the roadmap and macro rows (the header search on the roadmap view);
   - the triage view's search;
   - the activities view's search box;
   - the filter panel's value pickers (sprint, team, assignee, macro), including
     their built-in entries ("Non assigné", "Sans macro").
8. **Deployment prerequisite.** On PostgreSQL, the server needs the `unaccent`
   extension in its database. When the database role cannot create it, the
   server refuses to start and its error names the `unaccent` extension. There
   is no fallback to a case-only search.
9. **No text shown to the user changes.** Labels, placeholders and messages of
   the search inputs are unchanged.
10. `CHANGELOG.md` carries one `Fixed` line under `[Unreleased]`.

## Acceptance scenarios

All server scenarios run on PostgreSQL.

- **Given** a ticket titled `Fix Bug in sync`, **when** the header search is
  `bug`, `Bug` or `BUG`, **then** the ticket is in the results each time.
- **Given** a ticket titled `Équipe plateforme`, **when** the header search is
  `equipe`, `Equipe`, `équipe` or `ÉQUIPE`, **then** the ticket is in the
  results each time.
- **Given** a ticket titled `equipe mobile`, **when** the header search is
  `ÉQUIPE`, **then** the ticket is in the results.
- **Given** a ticket whose only match is its parent title `Refonte Écran`,
  **when** the header search is `ecran`, **then** the ticket is in the results.
- **Given** a ticket whose description contains `Réseau` and nothing else
  matches, **when** the search is `RESEAU`, **then** the ticket is found.
- **Given** tickets titled `Réduire de 50%` and `Réduire de 500 ms`, **when**
  the header search is `50%`, **then** only the first is returned.
- **Given** tickets titled `a_b` and `axb`, **when** the header search is `a_b`,
  **then** only `a_b` is returned.
- **Given** an activity whose summary is `Clarification terminée`, **when**
  `GET /api/activities?q=TERMINEE` is called, **then** the activity is returned.
- **Given** the activities view listing that same activity, **when** the search
  box holds `terminee`, **then** the activity stays listed.
- **Given** the roadmap with a macro titled `Écran d'accueil`, **when** the
  header search is `ECRAN`, **then** that macro's row is shown.
- **Given** the triage view with a ticket titled `Équipe`, **when** its search
  holds `equipe`, **then** the ticket stays listed.
- **Given** the assignee picker of the filter panel with a person named
  `Hélène`, **when** the picker's query is `helene`, **then** `Hélène` is
  proposed; **and when** the query is `non assigne`, **then** "Non assigné" is
  proposed while unassigned tickets exist.
- **Given** a PostgreSQL database where the server's role may create
  extensions, **when** the server starts on it, **then** it starts and the
  `unaccent` extension exists afterwards; a second start changes nothing.
- **Given** a PostgreSQL database where `unaccent` cannot be created, **when**
  the server starts, **then** it stops with an error that names `unaccent`.
- **Given** a SQLite database, **when** the header search is `Bug`, **then**
  the results are those it returned before this change.

## Out of scope

- Fuzzy or typo-tolerant search, ranking, full-text indexing.
- Searching comments.
- The label filter of the filter panel (`label` parameter), which keeps its
  current matching.
- Accent and case folding on SQLite.
- Non-Latin scripts: letters outside what `unaccent` maps to ASCII (Greek,
  Cyrillic, …) keep their exact case on PostgreSQL. This follows from the
  settled approach and is recorded here so nobody reads it as a defect of the
  implementation.

## Open requirements

None. Every product question was settled during clarification.

One finding differs from the clarification report, without reopening any
decision: the web activities view does not send its search to
`GET /api/activities`. It filters the activities it already loaded in the
browser (`ActivitiesView.tsx`). Requirement 7 covers that box; the server-side
activities search (requirement 4) still gets the fix, for API and agent callers.
