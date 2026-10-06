# Specification #762 - Desktop: Folders

- Ticket: https://github.com/sebastienferry/sectile/issues/762
- Branch: `feat/762`
- Clarification: `docs/clarifications/762.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-06)
- Framework: Spec Kit

## Summary

The checkout path below the title of the selected execution in Sectile Desktop
gains a chevron when the execution has more than one folder. The chevron opens
a menu listing every folder of the execution: the primary worktree, the other
repositories' worktrees, the read-only context repositories, the attached
folders and the specifications worktree. Choosing an item copies that folder's
absolute path. The path itself keeps copying on click, as today.

## Scope

In scope:

- the list of folders of each run, kept by the local agent, exposed on
  `GET /desktop/runs` and kept in the run store;
- folders added while the run is going: a worktree prepared through
  `prepare_repository_worktree`, a folder attached through **Add folder…**;
- the chevron and its menu in Desktop, for ticket runs, conversations and free
  consoles alike;
- the agent contract, the Desktop README and the changelog.

Out of scope:

- opening a folder in an editor or a terminal from the menu;
- attaching, detaching or preparing folders from the menu;
- a "Copy all paths" item (dropped by the owner in round 2);
- the web client, the server, the tracker and the database.

## User stories

### US1 - Copy the path of any folder of the execution (P1)

As a Desktop user, I copy the path of any folder the selected execution works
with, so I can reach it from a shell or an editor without looking for it.

- **Given** a selected execution whose folder list has more than one folder,
  **when** I look at the path below the title, **then** a chevron follows it,
  labelled **Folders of this execution**.
- **Given** that chevron, **when** I activate it, **then** a menu opens listing
  one item per folder, the primary worktree first, each showing the folder's
  name, its role and its absolute path.
- **Given** the open menu, **when** I choose an item, **then** that folder's
  absolute path is on the clipboard, the menu closes and the **Copied** notice
  shows beside the path.
- **Given** the path itself, **when** I click it, **then** it is copied as
  today, whether or not the chevron is shown.

### US2 - The list says which folder is which (P1)

- **Given** a ticket run, **then** the list holds the primary worktree, the
  worktree of every other changed repository that has one, the checkout of
  every context repository mapped on this workstation, every attached folder
  that exists, and the specifications worktree when it is a folder of its own.
- **Given** a folder map entry that is not mapped or not found on this
  workstation, **then** it is not listed.
- **Given** each item, **then** its role reads **primary**, **changed**,
  **context**, **attached** or **specifications**, and its name is the
  repository name (the last segment of its identity), **specifications**, or
  the folder's base name for an attached folder with no repository.
- **Given** two entries naming the same folder, **then** it is listed once.

### US3 - Every execution with several folders offers it (P1)

- **Given** a conversation or a free console on a project with attached
  folders, context repositories or a specifications folder, **then** its
  folders are listed as a ticket run's are.
- **Given** an execution with a single folder, or none, **then** no chevron is
  shown and the path behaves as today.
- **Given** an agent that sends no folder list, **then** no chevron is shown
  and the path behaves as today.

### US4 - The list follows folders added during the run (P2)

- **Given** a running ticket run, **when** the session prepares a worktree in
  another repository with `prepare_repository_worktree`, **then** that worktree
  appears in the menu of every run of the ticket on this agent, as a
  **changed** folder.
- **Given** a running execution, **when** I attach a folder with **Add
  folder…**, **then** that folder appears in its menu.
- **Given** a conversation, **when** a turn starts, **then** its list is the
  project's folder list at that turn.
- **Given** the menu is open, **when** the list changes, **then** the menu
  shows the new list at its next opening, never under the pointer.

### US5 - The list survives an agent restart (P2)

- **Given** a run with several folders, **when** the agent restarts and
  restores it from its run store, **then** the restored run offers the same
  list.
- **Given** a run stored by an older agent, **then** it restores with no list
  and the path behaves as today.

### US6 - The menu behaves as Desktop's other menus (P2)

- **Given** the open menu, **then** arrow keys, Home and End move between
  items, Enter or Space copies, Escape closes it and returns focus to the
  chevron, Tab or a pointer press outside closes it.
- **Given** another execution is selected while the menu is open, **then** the
  menu closes.

## Functional requirements

- **FR1** The agent keeps, on every run it launches, the list of that run's
  folders, derived from the folder map it gives the engine.
- **FR2** `GET /desktop/runs` exposes the list as `folders`, an array of
  `{path, name, role, attached}`, omitted when empty.
- **FR3** Each folder's `path` is its worktree when the entry has one, else its
  folder; entries with no path and missing attached folders are left out;
  duplicates (same cleaned path) are dropped; the primary folder comes first.
- **FR4** `role` is one of `primary`, `changed`, `context`, `spec`, `local`;
  `attached` is true for a folder attached to the project on this workstation.
- **FR5** A worktree prepared for a task through `prepare_repository_worktree`
  is added, as `changed`, to every run of that task this agent holds that has
  not ended.
- **FR6** A folder attached through `/desktop/run-folder` is added to that run.
- **FR7** A conversation's list is refreshed from the project folder map at
  each turn.
- **FR8** The list is written to the run store with the run, and a change of
  the list alone triggers a write.
- **FR9** Desktop shows the chevron only when `folders` has more than one
  entry; the menu items copy through the existing clipboard bridge and reuse
  the **Copied** notice.
- **FR10** New user-facing strings are written in English, like the
  surrounding toolbar.

## Success criteria

- A ticket run with a context repository shows a chevron; its menu copies the
  context repository's path.
- A free console on a project with one attached folder shows a chevron with two
  items.
- A run with a single folder, and every run reported by an agent without the
  field, shows exactly today's control.
- `go test ./internal/agent/...`, `npm test` and the Desktop UI test covering
  the menu pass.

## Open points

None. The clarification settled every product question.
