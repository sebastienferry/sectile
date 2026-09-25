# ADR 0027: The specifications folder is a workstation setting

Status: Accepted

Supersedes in part: [ADR 0026](0026-macro-runs-and-macro-worktrees.md), its
paragraph "The specifications repository has two declarations".

## Context

ADR 0026 left the macro workflow with two declarations of where a project's
specifications live. The server kept `projects.spec_repo_path`, edited in the
web project options as "Dépôt des spécifications", and read it for the slicing
import, which opened files on the server's own disk. The workstation kept
`specRepos` in its local settings, edited in the desktop project dialog, and
read it for the macro worktree and the macro skill launches.

Once the server runs remotely, the first declaration names a directory nothing
on a workstation reads, and the slicing import reads a disk that holds no
specification. Users had to fill two fields with two different meanings, and
the web one could only be wrong. #443 also asked for specifications kept in a
plain folder, outside any Git checkout, which the workstation refused.

## Decision

**One declaration, on the workstation.** The desktop project setting
"Specifications folder" is the only place a project's specifications folder is
set. It stores an override only. Without one, a mono-repo project uses its local
code checkout, and keeps following it if the local repository changes; a
multi-repo project has none, and every macro operation refuses to run with a
message naming the setting rather than falling back on the code checkout. The
repository layout reaches the agent as `monoRepo` in its configuration; an
older server that does not send it reads as mono-repo, the server's own
default.

**The server stores no specifications path.** Migration 14 drops
`projects.spec_repo_path` with its values, which named server directories and
cannot be carried to any workstation. The web option disappears, and API
clients that still send `specRepoPath` are ignored.

**The web slicing import reads through the requesting user's local agent.** A
new agent operation, `macro_spec_file`, reads `tasks.md` or `spec.md` in the
workstation's specifications folder and returns the content and where it was
read. The server keeps the parsing and the merge; only the file read moves, so
the extractors are not duplicated on two sides of a versioned protocol. The
lookup itself lives in `internal/sddfiles`, a leaf package both sides may
import. A missing agent and an agent too old to know the operation are each
reported as what the user must do with the desktop app.

**A plain folder is accepted and detected, not declared.** A folder inside a
Git checkout is normalised to that checkout's top level, as before; a folder
outside any checkout is stored as typed. The desktop shows which one is in
effect. On a plain folder, macro operations create no worktree and no branch,
answer an empty branch with a warning, and `realign-macro` writes in place
without committing or pushing. The slicing import reads the working tree only:
there is no macro branch to fall back on.

## Consequences

- The web slicing import now needs the desktop app connected. The error says
  so, and the "stories" source, which reads the tracker, is unaffected.
- Task stages are unchanged: clarification and specification still land in the
  code repository, on the task branch, reviewed with the pull request.
- Two workstation settings defects surfaced and were fixed with this change:
  the agent never merged `specRepos` into the overrides its macro operations
  read, and clearing the last folder left it in the settings file.

## Rejected alternatives

- **Sending the parsed slicing from the agent.** It would duplicate the
  extractors on both sides and couple their versions for no gain: the file is
  small, and the server already owns the merge.
- **Keeping a server-side read as a fallback.** It is the declaration this ADR
  removes; a fallback that reads the wrong disk hides the real cause.
- **A toggle to declare whether the folder is a Git repository.** Detection is
  cheap and cannot disagree with the disk; a declaration can.
