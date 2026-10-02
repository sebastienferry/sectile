# Specification #634 - Roadmap: create the stories of a slicing in one gesture

- Ticket: https://github.com/sebastienferry/sectile/issues/634
- Branch: `feat/634`
- Parent: macro `M-11`
- Clarification: `docs/clarifications/634.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit
- Reference: Taskativ
  `openspec/changes/archive/2026-09-09-taskativ-74-batch-story-creation`. The
  Taskativ "write once at the end" and "record one activity" rules are not
  carried over (see Definitions and FR6, FR11).

## Summary

A macro's slicing lists the lines that should each become a story. Today each
line is turned into a story on its own, with its **Créer story** button. With
this change the owner selects several lines in the macro's panel and creates
all their stories in one gesture. Each story lands in the target project its
line names. A selected line that already carries a story is skipped. A line
that fails does not stop the others. The panel then reports the outcome line
by line and as a summary. Each slicing line also shows where it came from:
tasks.md, spec.md, an existing story, or typed by hand.

## Scope

In scope: the selection boxes, "select all" and the batch button in the
macro's panel of the roadmap, the batch route on the server, the per-line
report and its summary, the origin badge on every slicing line, the French and
English strings, and a changelog line. It applies to every tracker the
single-line action already supports.

Out of scope:

- Creating stories from the lines of several macros at once.
- One target project imposed on the whole batch: each line keeps its own.
- Any change to the single-line **Créer story** action, its route and its
  answer.
- A batch tool on the MCP interface.
- Editing a line's origin.
- Recording the batch as an activity, or keeping its report across a reload.
- Guaranteeing that two server replicas never create the same line twice (see
  FR8).

## Definitions

- **Slicing line**: one entry of the macro's slicing checklist (`MacroTodo`).
- **Attached line**: a line that carries a story key, whether the story was
  created from it or it was imported from a story already under the macro.
  An **unattached line** carries none.
- **Selection**: the set of lines the owner ticked for the batch. It is
  interface state only: never stored, and distinct from the existing `done`
  checkbox, which keeps its meaning.
- **Outcome** of a line in a batch: `created` (a story was created and its key
  recorded on the line), `skipped` (the line was already attached when its turn
  came) or `failed` (no story was created; the reason is given).
- **Origin** of a line: the stored `sourceKind`, one of `tasks`, `spec`,
  `stories`, or empty for a line typed by hand (and every line saved before the
  field existed). `sourceEntry` names the entry it came from, when known.

## User stories

### US1 (P1) - Create several stories in one gesture

As the owner of a macro, I want to tick several slicing lines and create their
stories at once, so that turning a slicing into tickets takes one click instead
of one per line.

1. Given a macro whose slicing has three unattached lines, when the panel
   shows it, then each unattached line has a selection box, unticked, beside
   its existing `done` checkbox.
2. Given a slicing with attached and unattached lines, when the owner uses
   **select all**, then every unattached line is selected and no attached line
   is.
3. Given two selected lines, when the owner looks at the batch button, then it
   reads **Créer les stories (2)** and is enabled; given no selected line, it
   is disabled.
4. Given two selected lines targeting the macro's project and one targeting
   another compatible project, when the owner starts the batch, then three
   stories are created, each in its line's target project and under the macro,
   and each line then shows its story key as a line created one by one does.
5. Given a batch that created stories, when it ends, then the board lists the
   new stories without a manual refresh.
6. Given a line ticked `done`, when the owner starts a batch, then its `done`
   state is neither read nor changed: `done` and selection are independent.
7. Given the panel switches to another macro or the page reloads, when the
   owner looks at the slicing, then no line is selected.

### US2 (P1) - One failure does not stop the others

As the owner, I want a failing line to be reported without cancelling the rest
of the batch, so that one bad line does not cost me the others.

1. Given three selected lines whose second targets a project that no longer
   exists, when the batch runs, then the first and third stories are created,
   the second line is reported `failed` with the reason the single-line action
   would give, and it carries no story key.
2. Given a line that failed, when the owner selects it again and relaunches,
   then it is attempted again; the lines created earlier are not.
3. Given a batch in which the acting user has no tracker token for a target
   project, when it runs, then the lines targeting that project fail with the
   missing-token reason, the other lines are still attempted, and the panel
   offers to add the token as the single-line action does.
4. Given a story the tracker created but whose Jira parent it refused, when
   the batch reports that line, then it is `created` and its notice is shown on
   the line.

### US3 (P1) - Already attached lines are skipped, never duplicated

As the owner, I want a batch never to create a second story for a line, so
that relaunching or racing another edit is safe.

1. Given a selected line that became attached after it was selected (for
   example by a single-line creation in another tab), when its turn comes, then
   it is reported `skipped` with the key it carries, and no story is created.
2. Given a batch interrupted after its first line was created (server stopped,
   connection lost), when the owner reopens the panel, then that line carries
   its story key and cannot be selected again.
3. Given a line attached to a story of a roadmap project, when the panel shows
   it, then it has no selection box; if the server is asked for it anyway, it
   is reported `skipped`.
4. Given a batch running on a macro, when another edit of the same macro's
   slicing is saved meanwhile (a line's text, another line's target, a new
   line), then that edit is kept and the batch's story keys are kept too.
5. Given a second tab that loaded the slicing before the batch, when it saves
   the slicing afterwards with those lines still unattached, then the lines
   keep the keys the batch recorded.

### US4 (P1) - A report per line and a summary

As the owner, I want to see what happened to each line, so that I know which
lines still need my attention.

1. Given a finished batch, when the owner looks at the panel, then each
   processed line shows its outcome: its new key for `created`, "déjà
   rattachée à KEY" for `skipped`, the reason in the error colour for
   `failed`.
2. Given a finished batch of 3 created, 1 skipped and 1 failed lines, when the
   owner looks at the panel, then a summary above the checklist reads
   "3 créées, 1 passée, 1 en échec".
3. Given a finished batch, when it ends, then one notification carries the
   same summary: a success when nothing failed, a warning otherwise.
4. Given the report is shown, when the owner starts another batch, switches
   macro or reloads, then the previous report disappears.
5. Given a batch is running, when the owner looks at the panel, then the batch
   button shows it is in progress, and every edit of that macro's slicing
   (selection, "select all", `done`, target project, removal, addition, import,
   the per-line **Créer story** buttons and the batch button) is disabled until
   it ends.

### US5 (P2) - See where each slicing line came from

As the owner, I want each slicing line to say where it came from, so that I
can tell an imported line from one typed by hand.

1. Given lines of each origin, when the panel shows the slicing, then each
   line carries a small badge: "tâches" (`tasks`), "spécification" (`spec`),
   "story existante" (`stories`), "saisie à la main" (empty).
2. Given a line whose `sourceEntry` is known, when the owner hovers its badge,
   then the tooltip shows that entry; given none, the tooltip names the origin
   only.
3. Given a line whose stored kind is one this version does not know, when the
   panel shows it, then the badge shows the stored kind as written rather than
   hiding it or calling it hand-typed.
4. Given the interface in English, when the panel shows the badges, then they
   read "tasks", "specification", "existing story", "typed by hand".

## Functional requirements

### Selection and gesture (web)

- **FR1** Each unattached line shows a selection box. An attached line shows
  none. The selection is never persisted and is cleared when the panel shows
  another macro and on reload. A line that becomes attached, or is removed,
  leaves the selection.
- **FR2** A **select all** control selects every unattached line; when they
  are all selected it reads **deselect all** and clears the selection. It is
  hidden when the slicing has no unattached line.
- **FR3** The batch button shows the number of selected lines, is disabled at
  zero and while a batch runs, and sends the selected line ids in one request.
- **FR4** While a batch runs, the panel allows no edit of that macro's
  slicing (US4.5), so that it never saves a list older than the keys the batch
  is recording. Once it ends, the panel takes the slicing the server returns.

### Batch on the server

- **FR5** `POST /api/projects/{id}/macros/{key}/stories` takes
  `{ "todoIds": [...] }` and processes each distinct known id once, in the
  order the lines appear in the saved slicing, one after the other. An empty
  list, an unknown project, a macro with no saved shaping, or a request that
  names nobody to write as (a key tied to no user) is refused as a whole, with
  nothing created, as the single-line action refuses it.
- **FR6** Each line goes through the same checks and the same creation as the
  single-line action, so a line is refused in a batch exactly when it would be
  refused alone, and for the same reason. The only difference: a line that is
  already attached, including to a roadmap project, is `skipped` rather than
  refused.
- **FR7** After each created story, its key is recorded on that line at once,
  before the next line is attempted. The record changes that line's key only:
  every other field and every other line is taken from the slicing as saved at
  that moment, so a concurrent edit is kept.
- **FR7b** A save of the slicing never clears a story key: a line present in
  both the stored and the saved slicing keeps its stored key when the save
  carries none. A line the save omits is still removed. Sectile offers no way
  to detach a line from its story, so this takes nothing away.
- **FR8** Within one server, a batch and a single-line creation on the same
  macro never run at the same time, so the "already attached" check always sees
  the key the other just recorded. Across server replicas this is not
  guaranteed, as it is not for the single-line action today.
- **FR9** One line's failure never stops the batch. A requested id that no
  longer matches a line is reported `failed` ("ligne de TODO introuvable").
  A story created on a line removed in the meantime is reported `created`
  with a notice saying its key could not be recorded; the same applies when
  the record itself fails.
- **FR10** The answer is `200` whenever the batch ran, even if every line
  failed. It lists one outcome per processed line, in processing order, with
  the line id and its outcome; the key and the created task for `created`, the
  existing key for `skipped`, the reason for `failed`, the notice when there is
  one; a failure caused by a missing tracker token also names the tracker, as
  the single-line refusal does. It also carries the counts of each outcome and
  the macro as saved after the batch.
- **FR11** The batch writes on the trackers as the acting user, like the
  single-line action. It records no activity and writes nothing on the macro's
  own tracker ticket.
- **FR12** The single-line route `POST /api/projects/{id}/macros/{key}/story`
  keeps its request, its answers and its refusals.

### Origin badge (web)

- **FR13** Every slicing line, attached or not, shows its origin badge (US5).
  The tooltip is the `sourceEntry` when present, the origin's name otherwise.

### Text and documentation

- **FR14** New strings are added in French and English in the planning locale.
  Server reasons stay in French, as the existing ones.
- **FR15** `CHANGELOG.md` gains one line under `## [Unreleased]` → `Added`.

## Success criteria

- A macro with ten unattached lines gets its ten stories with one selection
  and one click.
- No sequence of relaunches, interruptions or concurrent edits on one server
  produces two stories for the same line.
- Every line of a batch ends with exactly one visible outcome.

## Assumptions

- The batch is synchronous: the panel waits for the whole answer. The server
  sets no write timeout, and a slicing holds tens of lines at most.
- Progress line by line during the batch is not shown; the report comes at
  the end.

## Open points

None. The clarification settled every product question.
