# Specification #481 - Git project setup on desktop

- Ticket: https://github.com/sebastienferry/sectile/issues/481
- Branch: `feat/481`
- Clarification: `docs/clarifications/481.md` (rounds 1 and 2, confirmed by
  the owner on 2026-09-25)
- Framework: Spec Kit

## Summary

When a project is set up in the desktop project settings with a local folder
that cannot host worktrees, Sectile offers to make it a Git repository with an
empty first commit. It says, next to the action, that this repository stays on
the workstation, is never pushed, and only exists so Sectile can create
worktrees from it, and that those worktrees start without the files already in
the folder. Declining changes nothing compared with today.

## Scope

In scope: the two folder fields of the desktop project settings, **Local
repository** and **Specifications folder**.

Out of scope:

- The folder of each repository of a multi-repo project (its mapping requires
  a checkout whose `origin` matches the repository, which an initialized
  folder does not have).
- Adding a remote, pushing, creating a hosted repository (GitHub, GitLab).
- The web project settings.
- Committing the folder's current content: nothing is ever staged.
- The pull request stage of a project without a remote, which stays out of
  reach; nothing here claims otherwise.

## Definitions

- **Folder state**, as seen by the desktop for a path typed or chosen in one of
  the two fields:
  - **ready**: inside a Git checkout whose `HEAD` names a commit;
  - **unborn**: inside a Git checkout whose `HEAD` names no commit yet;
  - **not a repository**: an existing directory outside any Git checkout;
  - **missing**: anything else (empty, relative, absent, not a directory).
- **Initialization offer**: the explanation and the two actions ("Initialize a
  Git repository", "Not now") shown under a field whose folder is *unborn* or
  *not a repository*.
- **Initialization**: for a folder that is *not a repository*, create a
  repository in that folder on branch `main`, then an empty first commit; for
  an *unborn* one, the empty first commit alone, on the branch its `HEAD`
  already names.

## User stories (prioritised)

### US1 - Offer to initialize the Local repository (P1)

As a user setting up a project on a folder that is not yet a Git repository, I
am offered to make it one, so I do not have to leave Sectile to run Git
commands before the project can be used.

**Acceptance**

- **Given** the desktop project settings, **when** the Local repository field
  is set, by *Browse* or by typing, to a folder that is *not a repository*,
  **then** the initialization offer is shown under the field.
- **Given** the offer, **then** it states, in English, that the repository is
  created locally, is never pushed to any remote, only serves to create
  worktrees, and that worktrees start without the files already in the folder
  because nothing is committed.
- **Given** the offer, **when** the user chooses "Initialize a Git
  repository", **then** the folder becomes a repository on branch `main` with
  one empty commit, the folder's files are left untouched and uncommitted, the
  offer disappears, and a notice says the repository was initialized and that
  the settings still have to be saved.
- **Given** an initialized folder, **when** the settings are saved, **then** it
  is accepted as the Local repository, and a task launched afterwards gets its
  worktree created from that first commit.
- **Given** the offer, **when** the user chooses "Not now", **then** the offer
  disappears for that value, nothing is created on disk, and saving is refused
  with today's message, "Select a local Git repository".
- **Given** the field then changes to another value, or the settings are
  reopened, **then** the folder state is examined again and the offer shown
  again if it applies.
- **Given** the field names a folder that is *ready* or *missing*, **then** no
  offer is shown.

### US2 - Offer to initialize the Specifications folder (P1)

As a user whose specifications folder is a plain folder, I am offered the same
initialization, so macro specifications get their own worktree and branch
instead of being written in place.

**Acceptance**

- **Given** the Specifications folder field shown (not following the local
  repository), **when** it is set, by *Browse* or by typing, to a folder that
  is *not a repository*, **then** the initialization offer is shown under it,
  with the same explanation as US1.
- **Given** the settings are opened on a project whose stored specifications
  folder is *not a repository*, **then** the offer is shown without the user
  touching the field.
- **Given** the user initializes it, **then** the folder kind next to the field
  reads "Git repository" once saved, and macro operations then work with a
  worktree per macro, as for any Git specifications folder.
- **Given** the user chooses "Not now", **then** the folder is saved and used
  as a plain folder, written in place, exactly as today.
- **Given** the Specifications folder follows the local repository (mono-repo,
  box ticked), **then** it shows no offer of its own; the Local repository
  field carries it.

### US3 - A Git folder without any commit (P1)

As a user whose folder already went through `git init` but has no commit, I am
offered the missing first commit, because worktrees cannot start from nothing.

**Acceptance**

- **Given** either field names an *unborn* folder, **then** the initialization
  offer is shown, saying that the repository has no commit yet and that an
  empty first commit is what lets Sectile create worktrees.
- **Given** the user initializes it, **then** no repository is created again:
  one empty commit is made on the branch `HEAD` already names, and nothing
  else in the repository changes (no branch renamed, no file staged).
- **Given** the user chooses "Not now", **then** today's behaviour is kept: the
  folder is accepted by both fields as it is today.

### US4 - Failures are shown as they are (P1)

**Acceptance**

- **Given** a workstation with no Git identity configured, **when** the user
  initializes a folder, **then** the error Git returns is shown under the
  field, and the settings are not saved as a side effect.
- **Given** that failure happened after the repository was created, **then**
  the folder is left as an *unborn* repository, and the offer shown again is
  the one for an *unborn* folder, so a retry makes the commit alone.
- **Given** the user asks to initialize the root of the filesystem or their
  home directory, **then** the initialization is refused with a message saying
  why, and nothing is created.
- **Given** the folder was changed on disk between the offer and the action
  (it became *ready*, or *missing*), **then** nothing is committed and the
  offer is replaced by what the folder now is.
- **Given** a local agent too old to initialize repositories, **then** no offer
  is shown, and every field behaves as today.

## Functional requirements

- **FR1** The desktop examines the folder state of the Local repository field
  and, when shown, the Specifications folder field: when the settings open,
  when *Browse* returns a folder, and when a typed value is committed (the
  field's change).
- **FR2** A field whose folder is *not a repository* or *unborn* shows the
  initialization offer; any other state shows none.
- **FR3** The offer's explanation says: the repository stays on this
  workstation; it is never pushed and needs no remote; it only exists so
  Sectile can create worktrees; the first commit is empty, so worktrees start
  without the folder's existing files. For an *unborn* folder, it also says the
  repository has no commit yet.
- **FR4** Initializing a folder that is *not a repository* creates the
  repository in that exact folder, with `main` as its branch, then one empty
  commit. Initializing an *unborn* folder makes the empty commit alone.
- **FR5** Initialization never stages, commits, moves or deletes a file of the
  folder, and never adds a remote or pushes.
- **FR6** After initialization, `.tasks/` is excluded from the repository's
  status through its local exclude file, so Sectile's own worktrees never show
  as untracked files there; the folder's own `.gitignore` is never created or
  edited.
- **FR7** The commit uses the workstation's Git identity. When Git refuses it,
  its message is shown as it is and nothing else is undone or retried.
- **FR8** Initialization does not save the settings; saving is the user's own
  action, as for any other field.
- **FR9** "Not now" keeps today's behaviour: the Local repository refuses a
  folder that is *not a repository* and accepts an *unborn* one; the
  Specifications folder accepts both.
- **FR10** Initialization is refused for the filesystem root and the user's
  home directory, and for any path that is not an existing absolute directory.
- **FR11** An agent that does not offer initialization leaves the desktop
  behaving as today, with no offer shown.
- **FR12** New interface text is in English, as the desktop already is.
- **FR13** `CHANGELOG.md` records the change under `[Unreleased]` / `Added`.

## Edge cases

- A folder inside another Git checkout (for example a subfolder of a cloned
  repository) is *ready* or *unborn* according to that checkout; the repository
  it belongs to is the one offered the first commit, and no nested repository
  is created.
- An *unborn* repository whose `HEAD` names a branch other than `main` or
  `master`: the commit is made on that branch as it is. Task worktrees start
  from `HEAD` and work; macro worktrees look for `main` or `master` and still
  refuse to start there, with today's message. The branch is not renamed.
- A folder chosen for both fields: initializing it from either field makes it
  *ready* for both; the other field's offer disappears when it is examined
  again.
- Two clicks on "Initialize a Git repository": the second finds the folder
  *ready* and commits nothing more.
- A project with no remote reaches every stage up to implemented; the pull
  request stage still needs a remote, which this change does not add.

## Open requirements

None. The clarification settled every product question. The only point it left
to the specification, excluding `.tasks/` through the local exclude file, is
settled by FR6. FR10 (refusing the filesystem root and the home directory) is a
safety guard added at specification; it narrows what can be initialized and
removes nothing the clarification asked for.
