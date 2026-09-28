# ADR 0036: Folders are attached on the workstation, and every project is the same kind

Status: Accepted (#484)

Supersedes in part:
- [ADR 0027](0027-specifications-folder-is-a-workstation-setting.md), its rule
  that a multi-repo project has no specifications folder by default;
- [ADR 0028](0028-repositories-are-keyed-by-remote.md), the `monoRepo`
  setting, the resolution order of the primary repository and the launch that
  waits for a repository choice.

## Context

#456 (ADR 0028) gave a project a list of repositories keyed by remote, mapped to
folders on each workstation, behind a *Mono-repo* setting. A project had to be
switched to multi-repo before any of it applied, and that switch changed other
things: the specifications folder lost its default, and a launch whose ticket
was not pinned waited for somebody to choose a repository.

#484 asks for something simpler and more common: give a skill run the other
code, libraries or notes a ticket depends on, from the desktop, on any project.
Those folders are often not repositories the project declares, sometimes not
Git checkouts at all, and they differ from one workstation to the next.

Two constraints shape the answer. The server never holds a workstation path
(ADR 0003, ADR 0026). The skills belong to a shared marketplace plugin this
repository does not own, so the instructions a run needs cannot be written
into them.

The clarification is in `docs/clarifications/484.md`, the specification in
`specs/484-mono-multi-repo/`.

## Decision

**A workstation attaches folders to a project.** The desktop project settings
edit `projectSettings.<projectId>.folders` in `~/.config/sectile/settings.json`,
through the local agent's `/desktop/folders`. Only the paths are stored. What a
folder is (a Git checkout with its `origin`, a checkout without one, a plain
folder, or a folder gone since) is read from the disk at each use, so a
remote changed afterwards is followed. No request to the server carries a
path. A folder that is already the project's local repository, its
specifications folder, a mapped repository or an attached folder is refused,
and so is a second checkout of an attached remote. A checkout of one of the
project's repositories becomes that repository's folder instead.

**Every launch is told about them.** The folder map (ADR 0028) lists each
attached folder after the project's repositories, with two new keys,
`kind` (`git`, `folder`, `missing`) and `attached`, and a new role, `local`,
for a folder without a remote. An attached Git folder is `context`, or
`changed` once the ticket has a worktree in it. A missing folder is listed and
never given to the CLI. Claude and Codex receive the existing folders as
`--add-dir`; Codex declares the flag in its shared options for `codex` and
`codex exec` alike.

**The instructions live in Sectile, not in the skills.** The folder block of
the prompt and the `prepare_repository_worktree` description state the rule
for each role: work in the primary worktree; a context folder is read-only
until `prepare_repository_worktree` returns a worktree in it; a local folder is
changed in place, with no worktree and no pull request; every changed
repository needs its pull request in `prUrls`.

**An attached repository is changed like a project repository.** The server
relays `prepare_repository_worktree` for any remote identity to the caller's
agent, which creates the worktree in the mapped or attached folder, the mapping
winning over an attached checkout of the same repository. The server records
the identity on the ticket once a worktree came back, and never a path. The
ticket's changed repositories are no longer filtered on the project's list,
so an attached repository needs its pull request at the transitions and loses
its worktree at handoff, from any workstation that has it.

**One kind of project.** The `monoRepo` setting is removed from the model, the
database (migration 31), the API, both interfaces and the agent configuration;
a client that still sends it is not refused. A ticket runs in the repository
it is pinned to, else in the code repository. No launch waits for a
repository choice any more, and a wait recorded before the upgrade is cleared.
The specifications folder defaults to the code checkout on every project.
Any project may record a pull request of another repository.

## Consequences

- A project that was mono-repo gets a sibling library as context by attaching
  one folder, with no change on the server.
- A ticket not pinned on a project with several mapped repositories used to
  wait; it now runs in the code repository. Pinning it is how it runs
  elsewhere.
- Context folders stay read-only by instruction only, and Codex makes its
  `--add-dir` folders writable in its sandbox.
- An attached folder exists on one workstation. A ticket that changed one is
  still refused at `implemented` without its pull request when validated from
  another workstation, and the handoff there names the repository as not found
  rather than failing the rest.
- An agent older than this change reads every project as mono-repo: launches
  keep working in the code repository, but it refuses
  `prepare_repository_worktree` for an attached repository. The server and the
  agent are upgraded together.
- A pull request link that names a former path of the project's repository is
  now read in the repository it names, where the forge redirects it.
- The plugin's skills still say "On a multi-repo project, `$SECTILE_REPOSITORIES`
  lists the task's folders"; updating that sentence is left to the plugin's
  owner.

## Rejected alternatives

- **Adding an attached remote to the project's repositories** (clarification
  round 3). It would publish one workstation's folders to every member, and a
  plain folder has no remote to add.
- **Keeping the setting** and attaching folders to multi-repo projects only.
  The setting was the cost the ticket asked to remove.
- **A server-side list of folders.** A path means nothing on another
  workstation (ADR 0003).
- **Writing the rules in the skills.** They belong to a plugin this repository
  does not own; the prompt block and the MCP description reach every provider
  and every skill.
