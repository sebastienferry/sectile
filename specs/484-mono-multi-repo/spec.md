# Specification #484 - Attached folders, and one kind of project

- Ticket: https://github.com/sebastienferry/sectile/issues/484
- Branch: `feat/484`
- Clarification: `docs/clarifications/484.md` (rounds 1 to 5, owner confirmed
  on 2026-09-26, no open product question)
- Framework: Spec Kit

## Summary

A workstation can attach any number of folders to any project, by hand, in the
desktop project settings. They are handed to every skill execution of that
project as context. An attached Git checkout with a remote can be changed
through a worktree on the ticket's branch, and then needs its own pull request;
a folder without a remote can be changed in place.

The mono-repo/multi-repo distinction disappears. Every project has a code
repository and may declare other repositories; a ticket runs in the code
repository unless it is pinned to another one. No launch waits for somebody to
choose a repository any more, and the specifications folder of every project
defaults to its code checkout.

## Scope

In scope:

- a per-project list of attached folders on each workstation, edited in the
  desktop project settings;
- handing those folders to every launch, for every provider;
- changing an attached Git folder through a worktree and a pull request, and a
  folder without a remote in place;
- checking at the stage transitions that every repository a ticket changed has
  its pull request, attached folders included;
- removing the ticket's worktrees in attached folders at handoff;
- removing the mono-repo/multi-repo setting and everything that depended on it:
  the "choose a repository" wait, the desktop console picker, the
  "specifications folder required" state;
- the release note and the architecture decision record.

Out of scope:

- the skills of the shared marketplace plugin; their sentence "On a multi-repo
  project, `$SECTILE_REPOSITORIES` lists the task's folders" becomes
  inaccurate, and updating it is left to the plugin's owner;
- sharing attached folders between workstations, or with the server;
- enforcing read-only access to a context folder: it stays read-only by
  instruction only, as in #456;
- the web editor of the project's repositories and the pin of a ticket to one
  of them, which keep their behaviour apart from no longer depending on the
  removed setting;
- the one-time conversion of legacy repository paths (#456).

## Definitions

- **Code repository**: the repository the project's own checkout on the
  workstation belongs to (its code remote).
- **Project repositories**: the code repository plus the other repositories an
  admin declared for the project in the web project settings (#456). A ticket
  can be pinned to one of them.
- **Attached folder**: a folder a person added to a project in the desktop
  project settings of their workstation. It exists only on that workstation.
- **Identity** of a Git folder: the `host/path` of its `origin` remote, as
  Sectile already derives it for project repositories. A folder without
  `origin`, and a plain folder, has none.
- **Primary repository** of a ticket: the repository its main worktree lives
  in.
- **Changed repository**: a repository, other than the primary one, in which
  the ticket has a worktree prepared through `prepare_repository_worktree`.
  The server records its identity on the ticket; it never records a path.
- **Folder map**: the description of a launch's folders the local agent gives
  the agent CLI, in `SECTILE_REPOSITORIES` and in a block of the prompt (#456).

## User stories (prioritised)

### US1 - I attach folders to a project on my workstation (P1)

As a person working on a project from the desktop app, I want to attach other
folders of my workstation to the project, so that the skills see the code,
libraries or notes the ticket depends on.

**Acceptance**

- **Given** any project, **when** I open its desktop project settings,
  **then** I see an *Attached folders* list, empty at first, with a way to add
  a folder through the folder picker.
- **Given** the list, **when** I add a folder and save, **then** the folder is
  listed with what it is: a Git repository with its remote, a Git repository
  without a remote, or a plain folder.
- **Given** an attached folder that no longer exists, **when** I open the
  settings, **then** it is still listed and marked as not found, and I can
  remove it.
- **Given** an attached folder, **when** I remove it and save, **then** it is
  no longer listed and no later launch receives it.
- **Given** I add a folder that is already the project's local repository, its
  specifications folder, one of its mapped repositories or another attached
  folder, **then** the addition is refused with a message saying which one it
  already is.
- **Given** I add a Git folder whose remote is one of the project
  repositories, **then** it becomes that repository's folder on this
  workstation (shown under *Other repositories*), not a second entry.
- **Given** I add a Git folder whose remote is the same as another attached
  folder's, **then** the addition is refused, naming the folder already
  attached for that remote.
- **Given** attached folders on my workstation, **then** no request to the
  server carries their paths, and another workstation of the same project does
  not see them.

### US2 - Every execution receives the attached folders (P1)

As a person launching a skill on a ticket, I want the agent to know every
folder attached to the project, so that it reads them without my repeating
where they are.

**Acceptance**

- **Given** a project with attached folders, **when** a skill is launched on
  one of its tickets from this workstation, **then** `SECTILE_REPOSITORIES` and
  the folder block of the prompt list each attached folder with its path, its
  identity when it has one, what it is (Git repository or plain folder) and
  its role.
- **Given** the provider is Claude or Codex, **then** each attached folder that
  exists is also given to the CLI as an additional directory.
- **Given** another provider, **then** the attached folders are listed in the
  prompt and in `SECTILE_REPOSITORIES` only.
- **Given** an attached folder that no longer exists, **when** a skill is
  launched, **then** the launch goes ahead, the folder map lists the folder as
  not found, and it is not given to the CLI as a directory.
- **Given** an attached Git folder with a remote in which the ticket has no
  worktree, **then** the prompt says it is context: read it, and call
  `prepare_repository_worktree` for it before changing it.
- **Given** an attached folder without a remote, **then** the prompt says it
  may be changed in place, with no worktree and no pull request.
- **Given** a project with no attached folder and a single folder to describe,
  **then** the prompt carries no folder block, as today.

### US3 - A ticket can change an attached Git folder through a pull request (P1)

As a person whose ticket needs a change in an attached repository, I want the
agent to work in a worktree of that repository on the ticket's branch, and
Sectile to require its pull request, so that the change is reviewed like the
rest of the ticket.

**Acceptance**

- **Given** a ticket with a branch and an attached Git folder with a remote on
  the caller's workstation, **when** the agent calls
  `prepare_repository_worktree` with that folder's remote or identity, **then**
  the local agent creates, or reuses, the ticket's worktree on the ticket's
  branch in that folder and returns its path and branch.
- **Given** that worktree was prepared, **then** the ticket records the
  folder's identity among its changed repositories, visible from the web and
  from any workstation, and no path is recorded.
- **Given** the caller's workstation neither maps nor attaches a folder for the
  given remote, **then** `prepare_repository_worktree` fails with a message
  saying the repository is not attached on this workstation and where to
  attach it, and nothing is recorded on the ticket.
- **Given** an attached folder without a remote, **when** the agent calls
  `prepare_repository_worktree` for it, **then** the call fails, saying the
  folder has no remote and may be changed in place.
- **Given** the given remote is the ticket's primary repository, **then** the
  call fails as today, saying the primary worktree already exists.
- **Given** a ticket that recorded changed repositories, **when** it is moved
  to `implemented`, or adjusted, **then** the transition is refused unless each
  changed repository, attached folders included, has a pull request on the
  ticket's branch that meets the existing rules (#392, #456), and the refusal
  names each repository that lacks one.
- **Given** each changed repository has its pull request, **then** the
  transition records them all, the primary repository's pull request staying
  the ticket's current one, as today.
- **Given** the ticket is handed off, **then** its worktrees are removed in the
  primary repository and in every changed repository where it is on this
  workstation, attached folders included, and a repository that could not be
  cleaned is named.

### US4 - Every project works the same way (P1)

As an admin or a member of a project, I want a single kind of project, so that
I no longer have to choose between mono-repo and multi-repo and live with what
that choice changes.

**Acceptance**

- **Given** the web project settings, **then** there is no *Mono-repo*
  setting, and the list of the project's other repositories is available on
  every project.
- **Given** the desktop project settings, **then** there is no *Repository
  layout* row, and *Other repositories* is shown on every project that
  declares repositories beside its code repository.
- **Given** a ticket not pinned to a repository, **when** it is launched on any
  workstation, **then** it runs in the code repository, whatever the other
  repositories mapped on that workstation.
- **Given** a ticket pinned to a project repository, **when** it is launched,
  **then** it runs in that repository's folder on this workstation, and the
  launch fails with the existing message when that repository has no folder
  here.
- **Given** a project whose repositories list more than one repository,
  **then** the ticket detail offers to pin the ticket to one of them before
  launching, whatever the project was before.
- **Given** any launch, **then** it never waits for somebody to choose a
  repository: the board shows no "waiting for a repository" state and the
  desktop console offers no repository picker.
- **Given** an execution that was waiting for a repository when Sectile was
  upgraded, **then** it no longer shows as waiting for one.
- **Given** a project that was mono-repo or multi-repo before the upgrade,
  **then** its repositories, its tickets' pins and their changed repositories
  are kept.

### US5 - The specifications folder defaults to the code checkout (P2)

As a person running macro skills, I want every project to use its code
checkout as specifications folder unless I set another one, so that no project
refuses to run a macro operation for lack of a setting.

**Acceptance**

- **Given** a project with no specifications folder set on this workstation,
  **when** a macro operation runs, **then** it reads and writes the
  specifications in the project's code checkout.
- **Given** a specifications folder set on this workstation, **then** it is
  used, as today.
- **Given** the desktop project settings of any project, **then** the
  *Specifications folder* row offers "Specifications live in the code
  repository" and is never marked as required.

### US6 - The change is documented (P3)

**Acceptance**

- **Given** `CHANGELOG.md`, **then** `[Unreleased]` names the attached
  folders, the removal of the *Mono-repo* setting and of the repository
  choice wait, and the specifications folder default.
- **Given** `docs/adrs`, **then** a new ADR records the attached folders, the
  removal of the setting and the ticket-level record of changed repositories,
  and ADR 0027 and ADR 0028 are marked as superseded in part by it.
- **Given** `README.md` and `docs/`, **then** they no longer describe a
  mono-repo or multi-repo project, and they describe the attached folders.

## Functional requirements

- **FR-001** Each workstation holds, per project, an ordered list of attached
  folders, stored in its local settings only.
- **FR-002** The desktop project settings list, add and remove attached
  folders, for every project, and show for each one its kind (Git repository
  with remote, Git repository without remote, plain folder, not found) and its
  remote when it has one.
- **FR-003** A folder is refused as an attached folder when it is the project's
  local repository, its specifications folder, a folder already mapped to a
  project repository, an already attached folder, or a Git folder whose remote
  is already attached. A folder must be an absolute path to a directory when it
  is added.
- **FR-004** A Git folder whose remote is a project repository is stored as
  that repository's folder on the workstation instead of being attached.
- **FR-005** No request to the server carries the path of an attached folder.
- **FR-006** Every launch's folder map includes each attached folder with its
  path, its identity when it has a remote, its kind and its role. An attached
  Git folder with a remote has the role of a context repository, or of a
  changed repository once the ticket has a worktree in it; a folder without a
  remote has a role of its own that allows changing it in place.
- **FR-007** Claude and Codex receive each existing attached folder, and each
  worktree the ticket has in a changed repository, as an additional directory.
  Other providers receive none.
- **FR-008** A missing attached folder never fails a launch; it is listed as
  not found and not given as a directory.
- **FR-009** The prompt's folder block states the rule for each role: work in
  the primary worktree; call `prepare_repository_worktree` before changing a
  context repository; a folder without a remote may be changed in place with no
  worktree and no pull request; each changed repository needs its own pull
  request, given to `transition_stage` in `prUrls`.
- **FR-010** The description of the `prepare_repository_worktree` MCP tool
  states the same rule for attached folders and no longer mentions a
  multi-repo project.
- **FR-011** `prepare_repository_worktree` accepts a project repository or the
  remote of a Git folder attached on the caller's workstation. The server
  records the identity on the ticket only once the caller's agent returned a
  worktree for it.
- **FR-012** Every changed repository recorded on a ticket, other than its
  primary repository, needs its pull request at the transitions that require
  pull request evidence, whether or not it is a project repository.
- **FR-013** Handoff removes the ticket's worktree in every changed repository
  where this workstation has it, attached folders included.
- **FR-014** The mono-repo/multi-repo setting no longer exists in the data
  model, the database, the API, the web and desktop settings or the agent
  configuration. An API client that still sends it is not refused.
- **FR-015** A ticket's primary repository is the project repository it is
  pinned to, else the code repository.
- **FR-016** No launch waits for a repository choice; the waiting state, its
  board wording and the desktop console picker are removed, and a wait
  recorded before the upgrade is cleared.
- **FR-017** The specifications folder of a project defaults to its code
  checkout on every workstation that sets none.
- **FR-018** A pull request of a repository other than the project's own may
  be recorded on a ticket of any project, under the existing checks of #456.

## Edge cases

- A folder attached to two projects on the same workstation is allowed; each
  project lists it.
- An attached Git folder whose remote changes after it was attached is read
  with its current remote at each launch and at each
  `prepare_repository_worktree` call.
- An attached Git folder whose current remote becomes a project repository
  after it was attached is treated as that repository's folder, unless the
  workstation already maps another folder to it, in which case the mapping
  wins and the attached entry is listed as a duplicate in the settings.
- A ticket that recorded a changed repository which is attached on another
  workstation only: its pull request is still required; the handoff on this
  workstation names that repository as not found here rather than failing the
  rest.
- An agent older than this change answers `prepare_repository_worktree` for a
  repository it does not know with an error, which the caller sees; it never
  records anything.
- A pin to a repository no longer in the project's list is ignored, as today.

## Success criteria

- A mono-repo project of before the upgrade gets a sibling library as context
  by attaching one folder, with no change on the server.
- No launch parks waiting for a repository after the upgrade.
- A ticket that changed an attached repository cannot reach `implemented`
  without that repository's pull request.

## Open questions

None. The follow-up on the plugin's skill wording belongs to the plugin's
owner.
