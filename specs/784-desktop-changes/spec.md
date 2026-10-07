# Specification #784 - Desktop Changes

- Ticket: https://github.com/sebastienferry/sectile/issues/784
- Branch: `feat/784`
- Clarification: `docs/clarifications/784.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-07)
- Framework: Spec Kit

## Summary

The chevron after the path of the selected execution in Sectile Desktop
(#762) stops copying paths and becomes a folder selector. The folder it
selects drives the whole toolbar: the path shown and copied, **Open in
<editor>**, and the **Changes** view, which inspects that folder instead of
always inspecting the execution's primary checkout.

## Scope

In scope:

- selecting a folder from the chevron menu, the selection marked in the menu;
- the path, its copy, the editor and the **Changes** view following the
  selection;
- the local agent accepting a folder on `GET /desktop/git-diff` and
  `POST /desktop/open-editor`, limited to the run's own folders;
- inspecting a folder other than the primary checkout against its own branch
  and default branch;
- one new agent capability, the agent contract, the Desktop README and the
  changelog.

Out of scope:

- what the diff computes inside one folder (baseline rules, limits, Markdown
  rendering);
- a view combining several folders;
- the web client, the server, the tracker and the database;
- remembering the selection across Desktop restarts.

## User stories

### US1 - Select the folder the toolbar speaks of (P1)

As a Desktop user, I pick one of the folders of the selected execution, so the
toolbar shows, copies and opens that folder.

- **Given** an execution with more than one folder and an agent that supports
  folder selection, **when** I open the chevron menu, **then** each item is a
  single-choice entry and the selected folder is marked as checked; the
  primary folder is checked by default.
- **Given** the open menu, **when** I choose an item, **then** the menu closes,
  that folder is checked, and the path below the title shows its absolute path.
- **Given** a selected folder, **when** I click the path, **then** its path is
  copied and **Copied** shows beside it.
- **Given** a selected folder and a configured editor, **when** I click **Open
  in <editor>**, **then** the editor opens that folder.

### US2 - Changes inspects the selected folder (P1)

- **Given** the **Changes** view is open, **when** I select another folder,
  **then** the view reloads for that folder, and its context line names that
  folder and its branch.
- **Given** a folder other than the primary checkout, **then** it is compared
  with its own default branch, on the branch it is on: a context checkout on
  its default branch shows its uncommitted and unpushed local changes.
- **Given** a folder that is not a Git repository, is not the root of one, is
  missing, or is not on a branch, **then** it stays selectable, the **Changes**
  view shows the reason, and the path and the editor still work.
- **Given** the primary folder is selected, **then** the view behaves exactly
  as today.

### US3 - The selection belongs to its execution (P2)

- **Given** I selected a folder on one execution, **when** I select another
  execution and come back, **then** the folder I selected is still selected.
- **Given** a selected folder, **when** it leaves the execution's folder list,
  **then** the primary folder is selected again, and an open **Changes** view
  reloads for it.
- **Given** Desktop restarts, **then** every execution starts on its primary
  folder.

### US4 - Older agents and single folders keep today's behavior (P2)

- **Given** an execution with one folder, or an agent that sends no folder
  list, **then** no chevron is shown and the toolbar behaves as today.
- **Given** an agent that lists folders but does not support folder selection,
  **then** the chevron menu keeps copying paths (#762), and the editor and
  **Changes** use the primary folder.

## Functional requirements

- **FR1** `GET /desktop/status` announces the capability `folder-selection`.
- **FR2** `GET /desktop/git-diff?id=<runId>&folder=<path>` and
  `POST /desktop/open-editor` with `{runId, folder}` accept an optional folder.
  No folder, or the run's own directory, means the primary checkout, as today.
- **FR3** Any other folder is accepted only when it is one of the run's listed
  folders (compared as cleaned paths); otherwise the agent refuses it with 404
  and starts nothing. The path used is the one the run lists, never the one
  the request carries.
- **FR4** A non-primary folder is inspected against its own current branch and
  its own Git common directory, with the existing baseline rules. A folder that
  is missing, not a Git repository, not its repository's root, or on a detached
  HEAD is refused with a specific error message.
- **FR5** Desktop sends a folder only for a non-primary selection and only to
  an agent announcing `folder-selection`.
- **FR6** The selection is kept per execution in memory, falls back to the
  primary folder when the folder leaves the list, and drives `#directory`, the
  path copy, **Open in <editor>** and the **Changes** view.
- **FR7** The **Changes** view drops a result that answers a previous folder,
  as it already drops one that answers a previous execution.
- **FR8** New user-facing strings are written in English, like the surrounding
  toolbar and panel.

## Success criteria

- On an execution with a context repository, selecting it shows its path, and
  **Changes** lists its local changes against its default branch.
- An unlisted folder sent to either route is refused by the agent.
- A single-folder execution, and every execution of an older agent, shows
  exactly today's toolbar.
- `go test ./internal/runner/... ./internal/agent/...`, `npm test` and the
  Desktop UI tests covering the menu, the editor and **Changes** pass.

## Open points

None. The clarification settled every product question.
