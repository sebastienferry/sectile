# PR and MR state indicators

## User stories

1. As a user of either interface, I can identify whether the current PR is open, conflicting, merged or closed without merge before opening the forge.
2. As a user with several PRs on a story, I see the state of the current (last) link on task indicators and the individual state beside each historical link.
3. As an operator, I can synchronize a project without introducing a full story read for each linked PR.

## Acceptance scenarios

- Given GitHub or GitLab reports a merge, when metadata is refreshed, then every surface for that link uses the merged indicator, irrespective of task workflow stage.
- Given an open PR reports actual merge conflicts, then the conflicting indicator is shown; unknown mergeability, missing approvals and pending checks do not mean conflicts.
- Given a PR closes without merge or reopens, the next successful refresh reflects its current state.
- Given a legacy or inaccessible link, its unknown or last observed state is preserved; errors never imply closed or merged.
- Given multiple links, refreshing their states does not change their order, branch metadata, current URL or task status.
- Given a project synchronization, known links are read in bounded forge batches and no single-story synchronization jobs are introduced.
- Given a changed or explicitly resynchronized story, additional reads concern only that story's PR metadata.
- Given a workflow step with linked PRs, those links are refreshed without synchronizing unrelated stories.
- Given a URL on another host, credentials are never sent to that URL; unsupported hosts stay unknown.

No dedicated polling, automatic merge, workflow-derived merge state, or GitLab issue tracker implementation is included.
