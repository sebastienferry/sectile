# Tasks #629 - Roadmap: remember the view and give the list more room

Ordered checklist. Each group is one commit and leaves the tree buildable.

## 1. Epic address on the macro list (FR9)

- [x] T1.1 `internal/trackerapi/github.go`: add `HTMLURL` (`html_url`) to
  `GithubMilestoneItem`.
- [x] T1.2 `internal/models/models.go`: add computed `ExternalURL`
  (`externalUrl,omitempty`) to `MacroMeta`.
- [x] T1.3 `internal/db/macros.go` `GetProjectMacros`: milestone map for listed
  `M-<n>` keys; one batched task lookup by key for the others through
  `computeExternalURLUnsafe`; nothing stored.
- [x] T1.4 `internal/db/macrourls_test.go`: listed milestone gets its address, local
  `M-` key gets none, Jira-style epic task gives its browse URL, unmatched key
  gets none. Run the `internal/db` tests (SQLite; PostgreSQL when a DSN is set).

## 2. Open and copy the epic (FR10, FR11)

- [x] T2.1 `web/src/types/index.ts`: `MacroMeta.externalUrl?`.
- [x] T2.2 `web/src/lib/roadmap.ts`: `externalUrl` on `MacroRow`,
  `macroCopyPayload(row)`.
- [x] T2.3 `RoadmapView.tsx`: "Ouvrir dans le tracker" from `selected.externalUrl`,
  hidden without; "Copier le lien" button with toasts and hand-copy fallback.
- [x] T2.4 Strings `copyLink`, `copyLinkTitle`, `copyRefTitle`, `linkCopied`,
  `refCopied`, `copyFailed`, `copyByHand` (fr, en, type).
- [x] T2.5 Unit test of `macroCopyPayload`.

## 3. Remembered view (FR1-FR4)

- [x] T3.1 Export the storage resolver from `roadmapDisplayMode.ts`.
- [x] T3.2 Create `web/src/lib/roadmapViewPrefs.ts` and
  `web/tests/roadmapViewPrefs.test.mjs`.
- [x] T3.3 `RoadmapView.tsx`: persisted `tab` through a saving `setTab` (click and
  search jump); per-project `selectedKey` loaded on project change and saved on
  click and delete; persisted `isPanelExpanded`, `isDescExpanded`,
  `isFramingExpanded`.

## 4. Hide the panel and expanded view (FR5, FR6)

- [x] T4.1 Persisted `isPanelHidden`, `hidePanel`, `showPanel`, `toggleExpanded`
  with the exclusivity rule.
- [x] T4.2 Header button "Masquer le panneau"; right-edge rail "Afficher le
  panneau"; split handle and panel conditions on `panelShown`.
- [x] T4.3 Render the toolbar, filter chips, sprint strip and notices above the
  split only when `!expandedHere`.
- [x] T4.4 Strings `hide`, `show` (fr, en, type).

## 5. Full-screen framing editor (FR7, FR8)

- [x] T5.1 `Markdown.tsx`: `maximizable` and `maximizeTitle` props, maximize
  button in the tab bar, portalled overlay with editor and live preview side by
  side, snippet toolbar on the active textarea, Escape and Réduire close only the
  overlay, focus in and back.
- [x] T5.2 `RoadmapView.tsx`: `maximizable` on the Description and Framing notes
  editors only.
- [x] T5.3 Strings `maximize`, `restore` in `taskDetail.markdown` (fr, en, type).

## 6. Browser tests, docs, checks (FR12, FR13, AC7)

- [x] T6.1 `web/tests/roadmap-view.browser.mjs`: restore after remount, hide and
  rail, hide clears expanded, expanded hides tabs and toolbar, copy success and
  refusal, no tracker link without address.
- [x] T6.2 Full-screen editor checks: no button by default, overlay preview
  follows typing, Escape closes only the overlay. Folded into
  `roadmap-view.browser.mjs`, which mounts a plain `MarkdownEditor` beside the
  roadmap, rather than a separate file.
- [x] T6.3 `CHANGELOG.md` `[Unreleased]` → `Added`: one line, for example "The
  roadmap reopens on the tab, epic, panel and sections you left, can hide its
  details panel or give it the whole view, opens the framing in a full-screen
  editor, and copies or opens the epic's own tracker link (#629)."
- [x] T6.4 `docs/USER_GUIDE.md`: the roadmap section mentions hiding the panel,
  the full-screen framing editor and the copy button, if it describes the panel.
  Nothing to change: the guide has no roadmap section.
- [x] T6.5 Run: `npx tsc --noEmit`, `npx oxlint`, `node --test web/tests`, the
  browser tests, `npx vite build` (restore `internal/webui` `.gitkeep`
  afterwards), `go test ./internal/db/... ./internal/trackerapi/...`.
- [ ] T6.6 Manual check in a browser: every story of `spec.md` in French and
  English, on a throwaway server and database copy. Not done: the stories are
  covered by `roadmap-view.browser.mjs` against the real component, in French;
  the English strings are checked by the type of the catalog only.
