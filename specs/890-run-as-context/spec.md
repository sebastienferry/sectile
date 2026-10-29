# Spec #890 - Take an earlier run as the context of a new run

Ticket: https://github.com/sebastienferry/sectile/issues/890
Clarification: `docs/clarifications/890.md` (rounds 1 and 2, settled)
Branch: `feat/890`

## Problem

A run that stops, fails or is canceled leaves its findings behind: what it
read, what it tried, where it got stuck. Relaunching the same skill today
starts from scratch. The new agent re-reads the same files, repeats the same
attempts and has no idea what the previous one learned, unless the person
copies the console into the prompt by hand.

The person wants to start a run with an earlier run as its context, so the new
agent picks up where the earlier one left off.

## Scope

In:

- A "Resume" action on a run that is no longer active, in Sectile Desktop
  (sidebar row and console view). It relaunches the same skill on the same
  ticket and project, directly, with the earlier run as context.
- A `contextRunId` parameter on the MCP `start_run` tool, and a new MCP tool
  `get_run` that returns the context of a run.
- The context handed over: the earlier run's transcript when the workstation
  that starts the new run still holds it, otherwise its server record.
- The new run records which run served as its context, and the run views of
  the desktop and the web show it.

Out (clarification, rounds 1 and 2):

- Resuming the earlier run itself: it stays finished. The provider's native
  session resume (`claude --resume`, Codex `thread/resume`) is not used.
- A "Context run" field in the launch dialog, desktop or web. The dialog does
  not change.
- A "Resume" button in the web app.
- The conversation composer as an entry point.
- Macro runs and project runs, as a source of context or as a resumed run.
- More than one context run per launch.
- Copying transcripts to the server, or sharing them across workstations. How
  runs are recorded and kept does not change.
- The plain retry: it keeps its current behaviour wherever it is offered.

## User stories

### US1 - Resume a run from the desktop (P1)

- **US1.1** Given a task run in the desktop sidebar whose status is completed,
  failed or canceled, its row and its console view offer a "Resume" button.
- **US1.2** Given a run that is queued, pending or running, no "Resume" button
  is shown, in the sidebar or in the console view.
- **US1.3** Given a free console (no ticket) or a macro run, no "Resume" button
  is shown.
- **US1.4** When the person clicks "Resume", a new run of the same skill
  starts on the same ticket, for the same project, on this workstation,
  without any dialog. It uses the earlier run's mode and model, and the new
  run is selected in the sidebar once it shows.
- **US1.5** Given the ticket already carries an active run, "Resume" is
  refused with the same message as any other launch on a busy ticket, and
  nothing starts. Forcing stays the existing "Launch anyway" action.
- **US1.6** Given the ticket belongs to several projects and the earlier run
  names none (a run recorded before #741), the resume uses the project of the
  board the person is on, as the existing relaunch does.
- **US1.7** Given a local agent too old to resume a run, the button says the
  agent must be updated and restarted, as other capability-gated actions do.

### US2 - The new run starts with the earlier run's context (P1)

- **US2.1** Given the workstation running the new run still holds the earlier
  run in its run store, the new agent gets the earlier run's transcript, with
  its server record (prompt, summary, final note, steps, posted output).
- **US2.2** Given the workstation no longer holds the earlier run (pruned,
  cleared, or run elsewhere), the new agent gets the server record only.
- **US2.3** The context states which of the two it is, the earlier run's ID,
  skill, ticket, status and dates.
- **US2.4** The context never travels on the agent's command line. The prompt
  says where to read it before starting.
- **US2.5** The context is bounded. When something was cut, a marker in the
  context says what and how much.
- **US2.6** A context that cannot be written does not fail the run: the run
  starts with a prompt saying the context of run `<id>` was unavailable and
  that `get_run` can read it.

### US3 - Agents name a context run through MCP (P2)

- **US3.1** Given an agent calls `start_run` with `contextRunId`, the run is
  recorded with that context run and the response carries the context.
- **US3.2** Given an agent calls `get_run` with a run ID, it receives the same
  bounded context a resumed run gets: the transcript when the calling
  workstation's run store holds it, otherwise the server record.
- **US3.3** Given an agent outside a Sectile-launched session (no local agent
  reachable), `get_run` returns the server record.

### US4 - Eligibility and refusals (P1)

- **US4.1** A context run is eligible when it is a task run (not a macro or
  project run), of the same project as the new run, on this ticket or another
  ticket, and no longer active.
- **US4.2** An unknown ID, a run of another project, a macro or project run,
  or a run still active is refused with a message naming the reason. Nothing
  is recorded: no run, no activity, as with the busy rule.
- **US4.3** `get_run` follows the same rules, except that it also reads a run
  still active (its context so far).

### US5 - See which run served as context (P2)

- **US5.1** Given a run started with a context run, its console view in the
  desktop shows "Context: run `<short id>` (`<skill>`, `<ticket>`)", which
  selects the earlier run when it is listed.
- **US5.2** Given the same run in the web app's task detail, its activity
  shows the same line, linking to the earlier run's activity.
- **US5.3** A run started without a context run shows nothing new.

## Functional requirements

- **FR1** A launch (`POST /api/tasks/{id}/run-skill`) accepts an optional
  `contextRunId`. It is validated (US4) before anything is recorded.
- **FR2** The retry endpoint (`POST /api/activities/{id}/retry`) accepts an
  optional `withContext`. When true, the relaunch carries the original run as
  its `contextRunId`. Without it, the retry is unchanged.
- **FR3** The new run records its context run ID. Run listings and run details
  (server API, MCP `get_task` activities, desktop run list) expose it.
- **FR4** The workstation receives the context run ID and the earlier run's
  bounded server record with the launch, writes the context to a file private
  to the run, and appends to the prompt a notice naming that file.
- **FR5** The transcript is taken from the workstation's run store: a headless
  run's trace, or an interactive run's console as plain text.
- **FR6** The context is capped at a fixed budget. The server record comes
  first, then the transcript tail. Every cut is marked.
- **FR7** `start_run` accepts `contextRunId`, validated as in FR1, and returns
  the context. `get_run(runId)` returns it on demand.
- **FR8** The desktop "Resume" button shows only on a task run that is no
  longer active, and only when the local agent supports resuming.
- **FR9** The user-visible strings are in English in the desktop, and in
  French and English in the web app, as each surface already is.
- **FR10** `CHANGELOG.md` gains, under `## [Unreleased]`, one `Added` line:
  resume a finished, failed or canceled run from Sectile Desktop, the new run
  starting with what the earlier one did as its context.

## Acceptance criteria

- On a completed, a failed and a canceled run of a ticket, the desktop sidebar
  and console view offer "Resume". On a running run they do not.
- Clicking "Resume" starts a new run of the same skill on the same ticket
  without a dialog, and the new run's prompt names a context file.
- That file holds the earlier run's transcript and server record when the run
  store holds the earlier run, and the server record alone otherwise, and says
  which.
- A long earlier run produces a context within the budget, with a truncation
  marker.
- "Resume" on a busy ticket is refused with the busy message, and no run is
  recorded.
- A launch with the ID of another project's run, of a macro run, of an active
  run, or an unknown ID is refused, and no run is recorded.
- `start_run` with `contextRunId` records it; `get_run` returns the context.
- The new run's console view and its web activity show "Context: run ...".
- The plain retry, without `withContext`, behaves as before.
- `CHANGELOG.md` has the `Added` line.

## Open points

None. The four product questions were answered in round 2 of the
clarification (content, entry points, eligibility, one run per launch), and
the owner confirmed the framing.
