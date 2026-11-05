## Ticket access
This skill comes from the Sectile Lite plugin: there is no Sectile server, agent or MCP behind it. Jira is the source of truth for the ticket; the forge (GitLab or GitHub) holds the code and its pull requests.
- Read and write the ticket through the Atlassian connector the session exposes (an Atlassian or Jira MCP server, such as the claude.ai Atlassian connector). Discover its actual tool names from the session; do not invent one. Its writes run with the user's own Jira access.
- The argument names a Jira key such as `PE-123`, or its URL. Read the issue and check its key and URL before any write: a bare number names no ticket.
- Where this skill names a Sectile tool or variable, do this instead:
  - `get_task`: read the Jira issue with its description, comments, labels, links and parent.
  - `get_project_context`: read it from the repository. `specFramework` is `speckit` when `.specify/` exists, `openspec` when `openspec/` exists, empty otherwise. `prCreationStage` is the one the repository's `AGENTS.md` or `CLAUDE.md` states, `implemented` otherwise. `projectName` is the Jira project key of the ticket.
  - `add_comment`: a Jira comment on the issue.
  - `transition_stage`: the stage change described under "Execution and ticket state". Its `prUrl` and `prUrls` are the pull request URLs the stage comment carries; `noRepositoryChange` is that comment saying no repository changed.
  - `record_pull_request`: a remote link from the Jira issue to the pull request.
  - `start_run`, `finish_run` and `report_waiting` have no equivalent: skip them, the session itself shows the run.
  - `prepare_task_spec_worktree` and `prepare_repository_worktree` have no equivalent: work in the task worktree.
  - `SECTILE_*` environment variables are never set: read every one of them as absent.
- If the connector is missing, or a call still fails after one retry, keep the work, report which write is pending and give its exact content (comment text, label) for the owner to apply. Do not reach Jira another way (a REST call with a token, another CLI) to get around it.
- This routing does not authorize mutations excluded by the skill or the user's request. Verify each write's response and report partial success explicitly.
