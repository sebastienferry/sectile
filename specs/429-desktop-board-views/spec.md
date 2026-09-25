# #429: Open a saved board view from the desktop task list

Ticket: https://github.com/sebastienferry/sectile/issues/429
Branch: `feat/429`.
Clarification: [`docs/clarifications/429.md`](../../docs/clarifications/429.md)
(confirmed by the owner on 2026-09-24, rebuilt on 2026-09-25).

## Context

Saved board views (#387) select tasks across projects on the web board. The
desktop's task list only opens one project at a time, so a user who organises
their work by view has to go back to the web to see it. A view also says nothing
about where its work happens: a view that gathers the tickets of one codebase
from several projects cannot send its launches to that codebase, and the
evidence and discovery rules only ever look at the project's own repository.

This file states behaviour and acceptance criteria only. Implementation choices
are in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

Out of scope: creating, renaming or deleting a view from the desktop, quick add
from a view, and an agent console on a view's repository.

## Terms

- **View**: a saved board view of the signed-in user (#387): a name, a list of
  projects and labels.
- **View repository**: an optional Git remote URL a view declares, set on the web.
- **View directory**: an optional local folder a view is given on one
  workstation. It never leaves the workstation.
- **Launch from a view**: a launch started from a row of a view's Tickets pane,
  or a relaunch or next step of a run started that way.
- **Recorded view repository**: the view repository a task was last launched
  with, kept on the task.

## User stories

### US1 (P1): The desktop chooser offers views

As a desktop user, I want to open one of my saved views from the task-list
chooser, so that I browse the same selection as on the web board.

**Acceptance**

- **Given** a user with saved views, **when** they open "Tasks list", **then**
  the chooser lists their projects and, in a separate group, their views by name.
- **Given** a user with saved views and a single project, **then** the chooser
  still opens, so that a view can be chosen.
- **Given** a view is chosen, **then** the Tickets pane opens titled with the
  view's name and lists the tasks the view selects, unfinished ones only.
- **Given** a view's Tickets pane, **then** each row shows the project it belongs
  to, and its skills, next step and launch availability are those of its own
  project.
- **Given** a task of a project that is not mapped on this workstation, **when**
  the view has no directory, **then** its row cannot be launched, as today.
- **Given** a launch from a view's row, **then** the run appears in the sidebar
  under its own project; the sidebar stays grouped by project.

### US2 (P1): A view has a local directory on the workstation

As a desktop user, I want to give a view a local folder, so that the work I
launch from the view runs in that checkout.

**Acceptance**

- **Given** a view's Tickets pane, **when** the user sets its local directory to
  a Git checkout, **then** it is saved on this workstation only and shown in the
  pane.
- **Given** a view with a repository, **when** the chosen directory's `origin`
  is another repository, **then** the directory is saved and the user is warned,
  naming both repositories.
- **Given** a directory that is not a Git checkout, **then** it is refused.
- **Given** the user clears the directory, **then** launches from the view use
  the project's resolution again.
- **Given** a launch from a view, **then** the working directory is chosen in
  this order: the view's directory; else the workstation's mapping of the view's
  repository (#456 mappings); else the project's usual resolution.
- **Given** a task launched from a view's directory, **when** the run is
  relaunched, or its next step is launched, or the server chains its next stage,
  **then** it runs in the same directory.

### US3 (P1): A view declares its repository on the web

As a web user, I want to say which Git repository a view's work lives in, so
that runs launched from it are verified and discovered there.

**Acceptance**

- **Given** the view form on the web, **then** it has an optional "Git
  repository" field; an empty value means none.
- **Given** a value that is not a Git remote (no host and path), **then** saving
  is refused with a message.
- **Given** a saved repository, **then** the view lists it and the form shows it
  when editing.
- **Given** a view edited without that field, **then** its repository is kept.

### US4 (P1): A launch from a view records its repository

**Acceptance**

- **Given** a view with a repository, **when** one of its tasks is launched from
  it and the launch runs in a folder the view chose (its directory, or the
  workstation's checkout of its repository), **then** the task records that
  repository and the user who launched it.
- **Given** a launch from a view without a repository, or from a view none of
  whose folders is on this workstation, **then** the task's recorded view
  repository is cleared: its work is back in its project's repository.
- **Given** a launch that is not from a view, **then** the record is left as it
  was.
- **Given** a launch that names a view the user does not own, or a view that does
  not select the task's project, **then** the launch is refused and nothing is
  recorded.

### US5 (P1): The recorded repository carries stage evidence

As the owner, I want a pull request in the recorded view repository to count as
the task's pull request, so that work done there can pass its stages.

**Acceptance**

- **Given** a task with a recorded view repository, **when** a stage requiring
  pull request evidence names a pull request in that repository, **then** it is
  checked with the rules of #392 for a foreign pull request, including on a
  mono-repo project.
- **Given** such a task, **when** the stage names no pull request, **then** the
  pull request is looked up by branch in the recorded view repository.
- **Given** a mono-repo project, **when** a pull request names a repository that
  is neither the project's nor the recorded view repository, **then** it is still
  refused, as today.
- **Given** the adjust prerequisite of such a task, **then** its pull request is
  read in the recorded view repository.

### US6 (P1): Sync discovers the pull request in the view repository

**Acceptance**

- **Given** a task with a recorded view repository, a branch and no pull request
  link, past the pull request creation stage, **when** a full synchronisation
  runs, **then** the pull request of that branch in that repository is attached:
  GitHub is read by the server, GitLab by the local agent of the user who
  launched.
- **Given** a tracker that cannot discover pull requests, **then** the view
  repository is still searched.
- **Given** the lookup fails, **then** the synchronisation reports a warning and
  goes on; nothing is attached.

### US7 (P1): The desktop degrades to projects

**Acceptance**

- **Given** an agent without the view capability, a server without views, the
  shared token, or a user with no views, **then** the chooser shows projects only,
  exactly as today, with no error.

## Functional requirements

- **FR1** A view stores an optional repository as a remote URL; it is validated as
  a Git remote naming a host and a path, and compared by #456 identity (US3).
- **FR2** A view directory is a workstation setting keyed by view ID, validated as
  a Git checkout, never uploaded. An `origin` mismatch warns and is accepted (US2).
- **FR3** The launch root order of US2 applies to every launch from a view, and to
  every later dispatch of the same task on the same agent (US2).
- **FR4** A launch from a view carries the view ID to the server, and whether
  the agent runs it in a folder of the view. The server checks that the view
  belongs to the launching user and selects the task's project, then records
  the view repository identity and the user on the task, or clears them (US4).
- **FR5** The recorded view repository is accepted as a foreign repository for
  pull request evidence on any project, and is where a task without a named pull
  request is looked up (US5).
- **FR6** A full synchronisation searches the recorded view repository by branch,
  under the discovery gate of today, whatever the tracker (US6).
- **FR7** The desktop lists views only when the agent declares the capability and
  the server answers the list; any failure means projects only (US7).
- **FR8** A user-visible `Added` entry is written under `[Unreleased]` in
  `CHANGELOG.md`.

## Success criteria

- In automated tests, a view's Tickets pane lists tasks of two projects, each row
  launched with its own project and the view ID.
- A mono-repo project accepts, in automated tests, the pull request of a task in
  its recorded view repository and still refuses any other foreign one.
- A full synchronisation attaches a pull request found in the recorded view
  repository, in automated tests.
- The existing chooser, launch, evidence and discovery tests pass unchanged.

## Open requirements

None. Every product question was settled in the clarification.
