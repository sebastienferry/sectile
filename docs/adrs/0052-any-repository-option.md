# ADR 0052: A ticket may work in any repository

- Status: Proposed
- Date: 2026-10-05
- Issue: [#737](https://github.com/sebastienferry/sectile/issues/737)
- Extends: [ADR 0036](0036-attached-folders-and-one-kind-of-project.md), its attached
  folders, and [ADR 0051](0051-macro-and-issue-specifications-folders.md), its
  Issue specifications folder

## Context

A ticket could change a repository only when the workstation already had a
folder for it: a mapping of one of the project's declared repositories, the
project's own checkout, or an attached folder whose `origin` matched. On a
project spanning many repositories, every one had to be declared, mapped or
attached before a ticket could touch it; PE-1668 stopped on `akamai-python`,
which was none of these. Two related limits followed: a ticket whose only
change was in another repository still got an empty worktree in the code
repository and was refused at `implemented` without a pull request there, and
a ticket could only be pinned to a declared repository.

## Decision

**A per-project workstation option, off by default.** "Any repository" is a
setting of the project on each workstation (`anyRepository`), next to its
clones folder (`clonesPath`, defaulting to the parent folder of the project
checkout). The risk it takes (disk, credentials, which repositories a session
may touch) is the workstation's, so the server holds no copy.

**The session finds, the agent clones.** `prepare_repository_worktree` takes an
optional `path`: a checkout the session found, accepted only when it is the top
level of a Git checkout whose `origin` is the repository. Without one, the
agent daemon clones the repository into the clones folder with the
workstation's own git setup, outside the Claude Code sandbox, never prompting
for credentials and never overwriting a folder. Sectile never scans the disk.

**What is found is remembered in the workstation mapping.** The mapping from a
repository identity to its folder is already workstation-wide and already read
for any identity. Writing the found or cloned checkout there lets the next
ticket of any project find it, and lets handoff remove its worktree.

**A new secondary branch starts from the remote default branch.** For every
repository but the launch's code checkout, a branch that exists neither
locally nor on `origin` is created after a fetch, from the remote default
branch, so a stale clone gives no stale base. A fetch that fails is a warning.

**The code worktree is made on demand when nothing needs it at launch.** With
the option on and the specifications away from the code checkout (dropped, or
in an Issue specifications folder of their own), an unpinned ticket is launched
without a code worktree: the session starts in the Issue worktree, else in the
project checkout, and the code repository is read-only context until the
session prepares it like any other repository. The code repository then needs
no pull request while the ticket never prepared it and the agent confirms the
branch carries nothing there; the first changed repository's pull request
stands for the ticket.

**Any remote may be pinned.** The server accepts a pin to any remote identity,
since only the launching workstation knows its option. A workstation without
the option refuses to launch such a ticket and names the option, rather than
run it in another repository. Removing a repository from a project clears the
pins to it, so a pin left behind cannot read as a deliberate one.

## Consequences

- A worktree prepared mid-run outside the session's directories is typed into
  the task's live Claude Code sessions as `/add-dir`; a session the agent does
  not own is told to add it.
- A pin to a repository a project stopped declaring before this change now
  reads as a pin to an undeclared repository: such a ticket launches only on a
  workstation with the option on. Unpinning it restores the previous
  behaviour.
- Nothing changes with the option off, except the fetch before a new secondary
  branch.

## Rejected alternatives

- **Creating worktrees by hand.** A `git worktree add` outside
  `prepare_repository_worktree` skips the record that makes the pull request
  accepted, shown and cleaned up.
- **Sectile searching the workstation.** Roots to configure and scan for a
  checkout the session can usually name itself.
- **Cloning from the session's shell.** It needs the sandbox's network hosts
  and writable paths for every clone; the daemon already runs git outside it.
- **Attaching the found folder to the one project.** Attached folders are
  per project; the mapping already serves every project of the workstation.
- **A server-side option.** It would decide for workstations whose disks and
  credentials it does not know.
