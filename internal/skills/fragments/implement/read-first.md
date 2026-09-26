- The specification and its task checklist. It is the contract, follow its order.
  Its files may be ignored by Git (`git check-ignore -q` succeeds on them): the project drops its
  specification artefacts on this workstation. Read them from the worktree, never commit them and
  never force them with `git add -f`. When the project drops its artefacts (the launch prompt says so,
  or the paths are ignored) and the specification is missing from the worktree, stop and report that
  it is not available on this workstation: never rewrite it.
- The surrounding code: naming, error handling, comment density, test style. Match it.
- How this project builds and tests. Find the real commands, do not assume them.