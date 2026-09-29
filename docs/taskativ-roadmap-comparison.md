# Taskativ and Sectile roadmap features

This study lists the Roadmap and Timeline features of Taskativ, Sectile's
predecessor, and says for each one whether Sectile offers it. It looks in one
direction only: what Taskativ has that Sectile lacks or covers in part. Each gap
points to the Sectile issue that already tracks it, or to a proposed ticket.
Written for #619 on 2026-09-29, against Sectile `origin/main` at `acdf4d18` and
Taskativ at `e6dc72f`.

## Summary

58 rows: 19 `covered`, 15 `partial`, 24 `missing`. Of the 39 gaps, 4 are
already tracked or decided by a Sectile issue (#426, #430) and 35 are grouped
into 11 new tickets, #626 to #636.

The three gaps ranked highest:

1. **Epic labels are not stored.** Sectile drops an epic's tracker labels at
   sync, so it cannot show them, filter on them, or build any label-based axis
   on top of them (rows 25, 26).
2. **Epics have no priority or quarter of their own.** The roadmap shows the
   highest priority among an epic's children and knows no quarter, so it
   cannot answer "which epic comes first" or "what lands this quarter"
   (rows 27 to 31).
3. **The roadmap cannot be grouped or reordered by those axes.** No sections
   by priority or quarter, no drag to classify, no multi-selection (rows 32
   to 34).

## Status meanings

- `covered`: a Sectile user can do what the Taskativ feature lets them do,
  even when the interface differs.
- `partial`: part of it; the Gap column says which part is missing.
- `missing`: no counterpart; the Evidence column says where it would belong.

Evidence paths are relative to the Sectile repository. Taskativ sources are
openspec directories (`changes/<id>`, `changes/archive/<id>`, `specs/<name>`)
or Taskativ files, relative to the Taskativ repository.

## Comparison

### Roadmap layout and navigation

| # | Feature | Taskativ source | Sectile | Evidence | Gap | Ticket |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | The Timeline and the Roadmap are two separate views, each with its own sidebar entry and palette command. | `changes/taskativ-36-split-timeline-and-roadmap-views`, `web/src/components/SprintTimeline.tsx` | covered | `web/src/components/Sidebar.tsx:446`, `:462`; `web/src/components/CommandPalette.tsx:208`, `:219`; `web/src/App.tsx:105` | Sectile also makes both views optional per project (`web/src/lib/optionalViews.ts:18`). | #31, #273, #394 (closed, delivered) |
| 2 | A global search does not narrow the tickets the Timeline reads. | `changes/taskativ-36-split-timeline-and-roadmap-views` | partial | `web/src/context/AppContext.tsx:1230` | Only the Roadmap is exempt from the server-side search; a search on the Timeline still narrows its tickets. | #636 (open, P11) |
| 3 | Horizon tabs NOW, NEXT, LATER, Unclassified and Hidden, with counts and a suggested horizon on each row. | `web/src/components/RoadmapView.tsx` | partial | `web/src/components/RoadmapView.tsx:93`, `:148`, `:628`, `:788` | The active tab and the selected epic are forgotten when the user leaves the view. | #629 (open, P4) |
| 4 | Classify an epic in one click, from the expanded row and from the condensed row. | `changes/taskativ-85-condensed-horizon-chips` | covered | `web/src/components/RoadmapView.tsx:624`, `:705`; `web/src/lib/roadmapDisplayMode.ts:19` | - | #345 (closed, delivered) |
| 5 | A condensed row mode, remembered between visits. | `changes/taskativ-52-roadmap-condensed-mode` | covered | `web/src/components/RoadmapView.tsx:421`, `:1014`; `web/src/lib/roadmapDisplayMode.ts:36`, `:68` | - | #345, #423 (closed, delivered) |
| 6 | Hide or show done epics, with their count. | `web/src/components/RoadmapView.tsx` | covered | `web/src/components/RoadmapView.tsx:385`, `:820` | - | #273 (closed, delivered) |
| 7 | A title or key search narrows the roadmap, counts the matches, jumps to the tab that holds them and explains matches hidden by other filters. | `web/src/components/RoadmapView.tsx` | covered | `web/src/components/RoadmapView.tsx:386`, `:434`, `:1037`, `:1086` | - | #273, #447 (closed, delivered) |
| 8 | Global filter chips are shown on the roadmap and can be cleared there. | `web/src/components/RoadmapView.tsx` | covered | `web/src/components/RoadmapView.tsx:284`, `:876` | - | #273 (closed, delivered) |
| 9 | Show placement anomalies only. | `web/src/components/RoadmapView.tsx` | covered | `web/src/components/RoadmapView.tsx:152`, `:428`, `:994` | Offered in the Execution display mode only. | #273 (closed, delivered) |
| 10 | A strip of active and upcoming sprints on the NOW and NEXT tabs. | `web/src/components/RoadmapView.tsx` | covered | `web/src/components/RoadmapView.tsx:1060` | - | #273 (closed, delivered) |
| 11 | Create an empty epic from the toolbar. | `web/src/components/RoadmapView.tsx` | covered | `web/src/components/RoadmapView.tsx:980` | - | #273 (closed, delivered) |
| 12 | An expanded epic row shows its key, priority, maturity with the number of lagging tickets, anomalies, sprint counters and open over total tickets. | `web/src/components/RoadmapView.tsx` | partial | `web/src/components/RoadmapView.tsx:571` | The maturity badge does not say how many tickets lag behind it, and the row shows no label badges (see row 25). | #629 (open, P4) |
| 13 | A resizable details panel that can expand to the full width. | `web/src/components/RoadmapView.tsx` | partial | `web/src/components/RoadmapView.tsx:218`, `:307`, `:1106`, `:1137` | The width is remembered, the expanded state is not, and the view's chrome stays visible when expanded. | #629 (open, P4) |
| 14 | Hide the details panel to give the list the full width, and bring it back without selecting the epic again. | `changes/taskativ-53-roadmap-hide-panel` | missing | No counterpart: the panel always shows when an epic is selected (`web/src/components/RoadmapView.tsx:1120`); it would belong in the panel header next to `:1137`. | - | #629 (open, P4) |
| 15 | The collapsed or expanded state of the panel sections is remembered. | `web/src/components/RoadmapView.tsx` | partial | `web/src/components/RoadmapView.tsx:219`, `:220` | The description and framing sections collapse, but their state is forgotten between visits. | #629 (open, P4) |
| 16 | Copy an epic's link or key from its panel, and open it in the tracker. | `web/src/components/RoadmapView.tsx` | partial | `web/src/components/RoadmapView.tsx:1153` | Opening in the tracker exists; copying the link or the key does not. | #629 (open, P4) |
| 17 | Maximize the framing editor into a full-screen editor with its preview beside it, closed by Escape. | `changes/taskativ-16-description-editor-maximize-button` (roadmap part) | missing | No counterpart: `web/src/components/RoadmapView.tsx:1748` renders the editor without a maximize option. | - | #629 (open, P4) |
| 18 | From a ticket, open the roadmap on its parent epic; from an epic, open the ticket views filtered on it and come back. | `changes/taskativ-82-roadmap-backlog-link` | missing | No counterpart: the parent filter exists (`web/src/context/AppContext.tsx:613`) but neither the roadmap nor the ticket card reaches the other. | - | #630 (open, P5) |

### Horizons and the tracker

| # | Feature | Taskativ source | Sectile | Evidence | Gap | Ticket |
| --- | --- | --- | --- | --- | --- | --- |
| 19 | The horizon is written to the epic as an exclusive label, read back from the tracker (the tracker wins), and failed pushes are listed and pushed again on demand. | Taskativ `internal/db/epics.go` | covered | `internal/db/macrohorizons.go:121`, `:203`, `:250`, `:296`; `web/src/components/RoadmapView.tsx:848` | - | #345, #357 (closed, delivered) |

### Epic slicing shown in the roadmap panel

| # | Feature | Taskativ source | Sectile | Evidence | Gap | Ticket |
| --- | --- | --- | --- | --- | --- | --- |
| 20 | Each slicing line can aim its story at another project on the same tracker; the target is frozen once the story exists. | `changes/taskativ-29-target-project-in-epic-todos` | covered | `web/src/components/RoadmapView.tsx:1846`; `internal/db/macros.go:431` | - | #426 (closed, delivered) |
| 21 | Produce the slicing from the repository's specification (`tasks.md`, `spec.md`, user stories), keeping lines already validated. | `changes/archive/2026-09-09-taskativ-69-slicing-from-sdd` | covered | `web/src/components/RoadmapView.tsx:1984`; `internal/db/sddslicing.go:229`; `internal/db/sddentries.go:190` | - | #423 (closed, delivered) |
| 22 | Specifications can live in another repository, and a ticket key in an entry title attaches the line to that story. | `changes/taskativ-78-spec-repo-and-key-attach` | covered | `internal/agent/agent_macro_dispatch.go:23`; `internal/db/sddentries.go:59` | - | #426 (closed, delivered) |
| 23 | Each slicing line shows where it came from (specification, scenarios, typed by hand). | `changes/taskativ-77-realign-spec-on-slicing` (visible part) | partial | `internal/models/models.go:327`; `web/src/types/index.ts:141` | The origin is stored but never shown on the line. | #634 (open, P9) |
| 24 | Turn the ticked slicing lines into stories in one gesture, with a report per line; lines that already have a story are skipped. | `specs/batch-story-creation`, `changes/archive/2026-09-09-taskativ-74-batch-story-creation` | missing | No counterpart: only the single-line action exists (`web/src/components/RoadmapView.tsx:1917`); a batch route would sit beside `internal/db/macros.go:386`. | - | #634 (open, P9) |

### Epic labels and classification axes

| # | Feature | Taskativ source | Sectile | Evidence | Gap | Ticket |
| --- | --- | --- | --- | --- | --- | --- |
| 25 | The sync keeps an epic's tracker labels, and the row shows them as badges. | `changes/taskativ-38-macro-labels-in-roadmap`, Taskativ `internal/db/epics.go` | missing | No counterpart: the epic record has no labels (`internal/models/models.go:274`); the panel only groups the child tickets' phase and goal labels (`web/src/components/RoadmapView.tsx:1303`). | - | #626 (open, P1) |
| 26 | Filter the roadmap on epic labels, and edit an epic's free labels while the axis prefixes stay protected. | `changes/taskativ-38-macro-labels-in-roadmap`, Taskativ `internal/db/epics.go` | missing | No counterpart; it would belong in the `web/src/components/RoadmapView.tsx` toolbar and panel and in `internal/db/macros.go`. | - | #626 (open, P1) |
| 27 | An epic has its own P0 to P3 priority, set and cleared from its panel and written to the tracker as a label. | `changes/epic-priority-label`, `changes/taskativ-85-condensed-horizon-chips` | missing | No counterpart: the priority shown is the highest among the children (`web/src/lib/roadmap.ts:213`). | - | #627 (open, P2) |
| 28 | An epic has a quarter, read from a prefixed or a bare label and set from its panel. | `specs/epic-quarter-label` | missing | No counterpart: "quarter" appears nowhere in `web/src/components/RoadmapView.tsx` or `internal/db/macros.go`. | - | #627 (open, P2) |
| 29 | Seed the priority and the quarter from the epic titles, with a preview per epic before anything is written; the title is never changed. | `changes/epic-priority-label`, `specs/epic-quarter-label` | missing | No counterpart; it would sit beside the pending horizon pushes (`internal/db/macrohorizons.go:250`). | - | #627 (open, P2) |
| 30 | Filter the roadmap on epic priority, including "no priority". | `web/src/components/RoadmapView.tsx` | missing | No counterpart; it would belong in the `web/src/components/RoadmapView.tsx` toolbar. | - | #627 (open, P2) |
| 31 | Sort the epics by priority, in either order, besides the backlog order. | `changes/taskativ-55-drag-epic-selection` | missing | No counterpart; it would belong in the `web/src/components/RoadmapView.tsx` toolbar. | - | #627 (open, P2) |
| 32 | Group each horizon tab into sections by an axis the user picks (priority, quarter, readiness), with collapsible sections and a "no value" section. | `specs/roadmap-epic-grouping`, `changes/archive/2026-09-04-taskativ-51-roadmap-epic-grouping` | missing | No counterpart; it would belong in the `web/src/components/RoadmapView.tsx` toolbar and `web/src/lib/roadmap.ts`. | - | #628 (open, P3) |
| 33 | Drag an epic onto a section to set that priority or quarter, or onto "none" to clear it. | `changes/taskativ-52-roadmap-condensed-mode`, `changes/taskativ-54-drag-epics-by-quarter` | missing | No counterpart: the only drag in the roadmap is the cut handle (`web/src/components/RoadmapView.tsx:453`). | - | #628 (open, P3) |
| 34 | Select several epics (Ctrl or Cmd click, Shift click) and drag them together; epics already in the target are skipped and failures are named. | `changes/taskativ-55-drag-epic-selection` | missing | No counterpart; it would belong in the `web/src/components/RoadmapView.tsx` list. | - | #628 (open, P3) |
| 35 | An epic carries a readiness level (idea, shaping, ready) decided by a person, suggested until then, usable as a grouping axis. | `changes/taskativ-94-epic-readiness` | partial | `web/src/lib/roadmap.ts:111`; `web/src/components/RoadmapView.tsx:559` | Sectile derives a maturity from the child tickets; nobody can decide a level, and it cannot group the roadmap. | #633 (open, P8) |
| 36 | Each project names its own label prefixes for the priority, quarter and readiness axes. | `changes/taskativ-86-axis-label-prefixes` | missing | No counterpart; it would belong in `web/src/components/ProjectModal.tsx`. | - | #635 (open, P10) |
| 37 | The priority and quarter axes map to tracker custom fields, flat or two-level; unknown options are learned when writing, and an unmapped value is never sent. | `changes/taskativ-87-axis-custom-fields`, `changes/taskativ-91-quarter-option-labels`, `changes/taskativ-92-learn-axis-options`, `changes/taskativ-93-cascading-quarter-field` | missing | No counterpart; the tracker layer already reads the edit metadata for ticket priority (`internal/trackerapi/jira_priority.go:436`). | - | #635 (open, P10) |
| 38 | The mapping between the tracker's priority scheme and Sectile's four levels is stored on the project, visible and editable, and a guessed mapping is refused on write. | `specs/tracker-priority-mapping`, `changes/archive/2026-09-04-taskativ-50-tracker-priority-mapping` | partial | `internal/trackerapi/jira_priority.go:130`, `:181`, `:330` | The mapping is discovered and cached in memory only; it cannot be seen or corrected, and a guessed match is written. | #635 (open, P10) |

### Epics of other tracker projects

| # | Feature | Taskativ source | Sectile | Evidence | Gap | Ticket |
| --- | --- | --- | --- | --- | --- | --- |
| 39 | A project declares other tracker projects, and the roadmap also reads their epics; one unreachable project does not stop the others. | `specs/roadmap-remote-projects`, `changes/archive/2026-09-09-taskativ-68-roadmap-remote-projects` | partial | `web/src/components/ProjectModal.tsx:1193`; `internal/db/roadmapprojects.go:20`; `internal/db/macrohorizons.go:180` | The projects can be declared, but only to attach story keys to slicing lines (`internal/db/sddentries.go:120`); their epics never reach the roadmap. | #632 (open, P7) |
| 40 | Pick which declared projects the roadmap shows, with a count per project, remembered between visits. | `changes/taskativ-80-roadmap-origin-selection` | missing | No counterpart; it would belong in the `web/src/components/RoadmapView.tsx` toolbar. | - | #632 (open, P7) |
| 41 | An epic from another project is marked read only, its write actions are hidden, and its classification stays local. | `specs/roadmap-remote-projects` | partial | `internal/db/macrohorizons.go:60`, `:265`; `internal/db/macros.go:423` | Writes and pending pushes already skip those epics, but since they never appear, the panel has no mark and hides nothing. | #632 (open, P7) |
| 42 | A project can allow writing the priority and the quarter on another project's epics, one epic at a time; otherwise the panel says why the controls are missing. | `changes/taskativ-90-remote-axis-writes` | missing | No counterpart; it would belong in `web/src/components/ProjectModal.tsx` and the roadmap panel. | - | #632 (open, P7) |
| 43 | A slicing line can aim its story at a declared project of the tracker, not only at a Sectile project. | `changes/taskativ-89-remote-target-projects` | missing | No counterpart: the picker lists Sectile projects only (`web/src/components/RoadmapView.tsx:1846`) and a declared key is refused (`internal/db/macros.go:423`). | - | #632 (open, P7) |

### Epic template and tracker mirror

| # | Feature | Taskativ source | Sectile | Evidence | Gap | Ticket |
| --- | --- | --- | --- | --- | --- | --- |
| 44 | A project maps its epic template sections (context, value, description, specifications, decisions) to tracker fields, and the panel shows them. | `specs/epic-template-fields`, `changes/archive/2026-09-09-taskativ-65-todos-from-epic-template-fields` | missing | No counterpart: the epic record has no sections (`internal/models/models.go:274`). | - | #426 (closed, left out on purpose) |
| 45 | Produce the slicing from the epic's Functional Scenarios table. | `specs/epic-template-fields`, `changes/archive/2026-09-09-taskativ-65-todos-from-epic-template-fields` | missing | No counterpart: the `scenarios` source was removed (`web/src/components/RoadmapView.tsx:1984`). | - | #426 (closed, left out on purpose) |
| 46 | Edit a template section from the panel and save it to its tracker field, refusing a concurrent change. | `specs/epic-template-fields`, `changes/archive/2026-09-09-taskativ-76-editable-epic-fields` | missing | No counterpart. | - | #426 (closed, left out on purpose) |
| 47 | The framing and the slicing checklist are published as comments on the epic ticket and kept up to date. | Taskativ `internal/db/epiccomments.go` | missing | No counterpart: the framing is stored locally only (`internal/db/macros.go:320`). | - | #636 (open, P11) |

### Timeline

| # | Feature | Taskativ source | Sectile | Evidence | Gap | Ticket |
| --- | --- | --- | --- | --- | --- | --- |
| 48 | Create a batch of sprints, rename them, change their dates, close and delete them, all written to the tracker. | `web/src/components/SprintTimeline.tsx`, Taskativ `internal/db/sprints.go` | partial | `web/src/components/SprintTimelineView.tsx:390`, `:432`, `:494`; `web/src/lib/sprints.ts:213` | Only Jira sprints are written to the tracker. A GitLab project gets local sprints, although the GitLab tracker already manages iterations (`internal/trackerapi/gitlab_sprints.go`). | #430 (open; its prerequisite shipped in #505) |
| 49 | Reopen a closed sprint of the tracker, and see in the delete confirmation how many tickets will become unplanned. | `web/src/components/SprintTimeline.tsx` | partial | `web/src/components/SprintTimelineView.tsx:407`, `:1241` | Reopening is offered for local sprints only, and the delete confirmation gives no ticket count. | #631 (open, P6) |
| 50 | Drag tickets from the backlog to a sprint and between sprints, several at once (Ctrl or Cmd click, Shift click range), with one write, the real moved count, busy tickets and the selection kept on failure. | `changes/taskativ-20-multi-select-sprint-drag`, `web/src/components/SprintTimeline.tsx` | partial | `web/src/components/SprintTimelineView.tsx:230`, `:604`, `:639`, `:1043` | Sprints accept a drop only while the backlog panel is open; selection is by checkbox only; tickets already in the target are counted; the selection is cleared even when the write fails. Removing tickets from a sprint works (#27). | #631 (open, P6) |
| 51 | A closed sprint, or one without a tracker id, refuses a drop before release and is not highlighted as a target. | `changes/taskativ-20-multi-select-sprint-drag` | missing | No counterpart; the check would sit in `web/src/components/SprintTimelineView.tsx:615`. | - | #631 (open, P6) |
| 52 | A progress bar per sprint with time elapsed against work done, an issue-type badge on each ticket, and an "open in tracker" link on backlog tickets. | `changes/taskativ-35-sprint-progress-bar`, `web/src/components/SprintTimeline.tsx` | partial | `web/src/components/SprintTimelineView.tsx:1003`, `:1222`, `:1323` | The bar has no elapsed-time marker and disappears on an empty sprint; tickets show no issue type; backlog tickets have no tracker link. | #631 (open, P6) |
| 53 | Close a sprint after a confirmation that says how many tickets are still open. | `web/src/components/SprintTimeline.tsx` | covered | `web/src/components/SprintTimelineView.tsx:494`, `:1859` | Sectile also moves the open tickets to the next sprint or the backlog. | #426 (closed, delivered) |
| 54 | Send tickets back to the backlog, one at a time or as a selection. | `web/src/components/SprintTimeline.tsx` | covered | `web/src/components/SprintTimelineView.tsx:655`, `:686`, `:950` | - | #27 (closed, delivered) |
| 55 | Assign the selected backlog tickets to a sprint picked from a list, without dragging. | `web/src/components/SprintTimeline.tsx` | covered | `web/src/components/SprintTimelineView.tsx:661`, `:1675` | - | #273 (closed, delivered) |
| 56 | A deadline badge in plain words on each sprint (ends in N days, ended but not closed, starts in N days). | Taskativ `web/src/lib/sprints.ts` | covered | `web/src/lib/sprints.ts:149`; `web/src/components/SprintTimelineView.tsx:1192` | - | #273 (closed, delivered) |
| 57 | Show or hide closed sprints, with the count of hidden ones. | `web/src/components/SprintTimeline.tsx` | covered | `web/src/components/SprintTimelineView.tsx:150`, `:811` | - | #273 (closed, delivered) |
| 58 | A backlog panel of unplanned tickets, with a search, that folds away and keeps its width. | `web/src/components/SprintTimeline.tsx` | covered | `web/src/components/SprintTimelineView.tsx:168`, `:280`, `:845` | - | #273 (closed, delivered) |

## Excluded entries

| Taskativ entry | Reason |
| --- | --- |
| `changes/taskativ-57-macro-skills` | Macro workflow skills, out of scope and already ported. |
| `changes/taskativ-77-realign-spec-on-slicing` (skill) | The `realign-macro` skill, already ported (`web/src/components/MacroRealignButton.tsx`); only the line origin display is kept, as row 23. |
| `changes/taskativ-79-macro-skill-external-shell` | How a macro skill is launched, not something the roadmap shows. |
| `changes/taskativ-81-name-import-sources` | Wording of the slicing import controls, part of the macro workflow ported by #423 and #426. |
| `changes/taskativ-10-translation` | Interface translation in general; Sectile's roadmap strings were handled by #529 and #530. |
| `changes/taskativ-12-issue-type-in-task-form-header` | The quick-add form, not the Roadmap or the Timeline. |
| `changes/taskativ-16-description-editor-maximize-button` (ticket part) | The ticket detail editor; the roadmap part is row 17. |
| `changes/taskativ-25-command-palette-all-views` | Palette coverage of every view; its roadmap part is in row 1. |
| `changes/taskativ-27-condensed-kanban-cards` | The board's cards; its proposal excludes the roadmap. |
| `changes/taskativ-99-renambe-kanban-board-to-board` | Renames the board view. |
| `specs/epic-template-fields`, "a macro's specification hands the product level to whoever writes it" | Behaviour of a macro workflow skill. |
| Taskativ epic creation, renaming, deletion and ticket moves | Basic epic editing, which Sectile has (`internal/db/macros.go:465` to `:817`). |
| Taskativ sprint read-back and date format adapters | Internal plumbing with nothing a user sees. |

## Proposed tickets

Ranking rule: first the gaps other gaps depend on, then by user value (a
feature used at every roadmap review before an occasional one), then by
smaller size. Titles follow the `Area | Summary` style of the Sectile issues.
The owner approved all eleven on 2026-09-29; they were created as #626 to
#636 for the Roadmap macro.

**P1. Roadmap | Keep and show an epic's labels** (#626) (rows 25, 26). The sync
throws away the labels an epic carries on the tracker. Keep them, show them as
badges on the roadmap row, let the user filter the roadmap on them and edit
the epic's free labels. The labels the roadmap uses for its own axes stay
protected. Every axis below builds on this.

**P2. Roadmap | Give each epic its own priority and quarter** (#627) (rows 27 to 31;
depends on P1). An epic gets a P0 to P3 priority and a quarter of its own,
set from its panel and written to the tracker as labels. The roadmap can
filter and sort on the priority. A one-off action reads both values from the
epic titles, shows what it would set, and writes nothing until the user
confirms.

**P3. Roadmap | Group and reorder epics by priority or quarter** (#628) (rows 32 to
34; depends on P2). Each horizon tab can be split into sections by priority or
quarter, with a "no value" section at the end. Dragging one or several epics
onto a section sets that value; epics already there are skipped and failures
are named.

**P4. Roadmap | Remember the view and give the list more room** (#629) (rows 3, 12 to
17). The roadmap forgets its tab, its selected epic, its expanded panel and
its collapsed sections when the user leaves it. Remember them, let the user
hide the panel to give the list the full width, copy an epic's link or key,
maximize the framing editor, and show how many tickets lag behind an epic's
maturity.

**P5. Roadmap | Go from a ticket to its epic and back** (#630) (row 18). From a ticket
with a parent, open the roadmap on that epic. From an epic, open the ticket
views filtered on it, and return to the view the user came from.

**P6. Timeline | Move tickets between sprints like Taskativ did** (#631) (rows 49 to
52). Let a sprint accept a drop while the backlog is closed, select tickets
with Ctrl, Cmd and Shift clicks, count only the tickets that really move, keep
the selection when the write fails, and refuse a drop on a closed sprint. Show
the elapsed time on the progress bar, the issue type on each ticket, a tracker
link on backlog tickets, the ticket count in the delete confirmation, and allow
reopening a tracker sprint.

**P7. Roadmap | Show the epics of declared tracker projects** (#632) (rows 39 to 43).
A project can already declare other tracker projects, but only to attach story
keys. Read their epics into the roadmap, let the user choose which projects
to show, mark those epics as read only, and let a slicing line create its
story in one of them. Writing the priority and quarter on them is an opt-in
per project. This keeps the read-only rule #426 set for declared projects;
only the opt-in in row 42 would relax it, one epic at a time.

**P8. Roadmap | Let a person decide an epic's readiness** (#633) (row 35; depends on
P3). Today the maturity is computed from the child tickets. Add a readiness
level (idea, shaping, ready) that a person sets, suggested until then, and
usable to group the roadmap.

**P9. Roadmap | Create the stories of a slicing in one gesture** (#634) (rows 23, 24).
Turn the ticked slicing lines into stories at once, with a report per line,
and show on each line where it came from.

**P10. Roadmap | Map the epic axes to tracker fields** (#635) (rows 36 to 38; depends
on P2). Let a project name its axis label prefixes, map the priority and
quarter to tracker custom fields, and see and correct how the tracker's
priorities map to Sectile's four levels, refusing a guessed mapping on write.
This is the part of the list most tied to one tracker's configuration; the
owner may prefer to drop it, as #426 did for the template fields.

**P11. Roadmap and Timeline | Smaller gaps** (#636) (rows 2, 47). A search on the
Timeline no longer narrows the tickets it shows, and an epic's framing and
slicing checklist are published as comments on its tracker ticket.

Not proposed:

- Rows 44 to 46 (epic template fields and the scenarios slicing source): #426
  left them out on purpose, because they are specific to one Jira setup.
  Re-open only if that decision changes.
- Row 48 (GitLab sprints written to the tracker): tracked by #430, whose
  prerequisite, a GitLab tracker with iterations, shipped in #505. What remains
  is the web side, which still treats Jira as the only tracker that manages
  sprints (`web/src/lib/sprints.ts:213`).

## Method

- **Inventory.** The Taskativ openspec specs `roadmap-epic-grouping`,
  `roadmap-remote-projects`, `epic-quarter-label`, `epic-template-fields`,
  `tracker-priority-mapping` and `batch-story-creation`; the archived changes
  50, 51, 65, 68, 69, 74 and 76; the open changes `epic-priority-label`, 10,
  12, 16, 20, 25, 27, 29, 35, 36, 38, 52 to 55, 57, 77 to 82, 85 to 87, 89 to
  94 and 99, found by the list of the specification and by a search of
  `openspec/` for "roadmap", "timeline" and "frise". Then the features of
  Taskativ's `web/src/components/RoadmapView.tsx`, `SprintTimeline.tsx`,
  `internal/db/epics.go`, `epiccomments.go` and `sprints.go` that no openspec
  entry describes. Entries that refine one feature share a row.
- **Evidence.** Read in Sectile's code; each path exists on `origin/main` at
  `acdf4d18`. Line numbers point at the place that proves the status.
- **Tickets.** All 275 Sectile GitHub issues, open and closed, read on
  2026-09-29 with `gh issue list --repo sebastienferry/sectile --state all
  --limit 1000 --json number,title,state,body`, filtered on roadmap, timeline,
  epic, macro, horizon, quarter, priority, sprint, iteration, axis, readiness,
  template, labels, selection, reopen, issue type and remote projects, then
  read one by one.
