---
name: dispatch
description: "Open one dedicated Claude Code session per ticket handed over by the Sectile board or named as arguments, each in its own worktree, and relay what they reach."
---
# Dispatch Tickets

## Ticket access
This skill comes from the Sectile Lite plugin: there is no Sectile server, agent or MCP behind it. Jira is the source of truth for the ticket; the forge (GitLab or GitHub) holds the code and its pull requests.
- Read and write the ticket through the Atlassian connector the session exposes (an Atlassian or Jira MCP server, such as the claude.ai Atlassian connector). Discover its actual tool names from the session; do not invent one. Its writes run with the user's own Jira access.
- The argument names a Jira key such as `PE-123`, or its URL. Read the issue and check its key and URL before any write: a bare number names no ticket.
- Never call a Sectile MCP server's tools, even when one is connected to this session (the full Sectile plugin installed beside this one, for example). They write through the Sectile server, which moves the Jira status along with the stage label: a Lite run touches Jira only through the Atlassian connector, as below.
- Where this skill names a Sectile tool or variable, do this instead:
  - `get_task`: read the Jira issue with its description, comments, labels, links and parent.
  - `get_project_context`: read it from the repository. `specFramework` is the `framework` of the ticket workspace (see Specifications workspace) when it is set; otherwise `speckit` when `.specify/` exists, `openspec` when `openspec/` exists, in the workspace, empty otherwise. `prCreationStage` is the one the repository's `AGENTS.md` or `CLAUDE.md` states, `implemented` otherwise. `projectName` is the Jira project key of the ticket.
  - `add_comment`: a Jira comment on the issue.
  - `transition_stage`: the stage change described under "Execution and ticket state". Its `prUrl` and `prUrls` are the pull request URLs the stage comment carries; `noRepositoryChange` is that comment saying no repository changed.
  - `record_pull_request`: a remote link from the Jira issue to the pull request.
  - `start_run`, `finish_run` and `report_waiting` have no equivalent: skip them, the session itself shows the run.
  - `prepare_task_spec_worktree`: prepare the ticket workspace as the Specifications workspace section says, when the skill has one. `prepare_repository_worktree` has no equivalent: work in the task worktree.
  - `SECTILE_*` environment variables are never set: read every one of them as absent.
- If the connector is missing, or a call still fails after one retry, keep the work, report which write is pending and give its exact content (comment text, label) for the owner to apply. Do not reach Jira another way (a REST call with a token, another CLI) to get around it.
- This routing does not authorize mutations excluded by the skill or the user's request. Verify each write's response and report partial success explicitly.

## Goal
Open one dedicated Claude Code session per ticket handed over, from the Sectile board or as arguments, each in its own worktree and starting with the ticket's Sectile Lite command. The dispatcher only starts, tracks and relays these sessions: it does none of the ticket work itself.

## Read first
- The hand-off: Jira keys given as arguments, each optionally followed by a skill (`PE-123`, `PE-123 specify-issue`, `PE-123 auto`), or what the Sectile board sent (`ticket`, `url`, `skill`, `command`, and whether the launch is autonomous).
- For a key that comes without a command, the ticket's `#<stage>` label.
- The repository map, `~/.config/sectile-lite/repositories.json`, when it exists. It belongs to this workstation and lists which local folder works which tickets:

  ```json
  {
    "repositories": [
      { "project": "PE", "component": "Wiki", "path": "/Users/me/Sources/pe/pe-wiki", "remote": "gitlab.example.com/pe/pe-wiki" },
      { "project": "PE", "path": "/Users/me/Sources/pe/iac-core" }
    ]
  }
  ```

  `project` is a Jira project key. `component` and `label` narrow a rule to the tickets that carry them. `path` is the folder of a local clone. `remote` is the repository's address without its scheme, used to recognise its pull requests. Compare two addresses as host and path only: drop the scheme, any user, a trailing `.git` and a trailing slash, and read `git@host:group/repo` as `host/group/repo`.
- For each ticket, its project, components, labels and pull request links (the remote links on the issue).
- The sessions this session already started, from the host's list of linked sessions.

## Steps
1. **List the tickets.** For each one, keep its key, its URL and its command. A board hand-off carries the command: use it as sent. Otherwise derive it from the stage label: none or `#new` → `clarify-issue`, `#clarified` → `specify-issue`, `#specified` → `implement-issue`, `#implemented` → `adjust-issue`, `#reviewed` → `handoff-issue`; `auto` → `pickup-issue`. A `#finished` ticket gets no session: report it.
2. **Find each ticket's repository.** Take the first answer in this order, and never guess a path:
   1. the folder the user named for that ticket;
   2. a rule of the repository map that matches the ticket. A rule with a `component` or a `label` matches only the tickets that carry it and wins over a rule with the project alone; two rules that match equally are no answer;
   3. a pull request linked to the ticket whose repository is a rule's `remote`, or the `origin` remote of the folder this session runs in;
   4. the folder this session runs in, only when it is a Git repository and the map has no rule for the ticket's project.

   Check that the folder exists and is a Git repository before using it. Ask once, for all the tickets still without a folder, listing for each one the candidates found (the map's paths and the host's start targets). After the answers, offer to save each of them as a rule of the map, `project` and, when the user says so, `component` or `label`; write the file only on a yes, creating it when it is missing and keeping its other rules.
3. **Skip what is already running.** A ticket whose session, started earlier by this dispatcher, is still working gets no second one: say so, and send that session the new step instead when the user asked for one.
4. **Start one session per ticket** with the host's session start capability (in the Claude desktop app, its start session tool, started now since the user asked for it): a fresh context, started in the ticket's folder with its own worktree there, titled `<KEY> · <skill>`, and as its whole prompt the command (`/sectile-lite:<skill> <KEY>`) followed by the line `Started by the Sectile dispatcher.` Write a bold title line and one sentence before each start.
5. **Keep to the limit.** Run at most 3 ticket sessions at once, unless the user gives another number. Queue the others in hand-off order, and start the next one when a session reports that it ended its turn.
6. **Relay, do not take over.** When a ticket session reports back, say in one line what it reached and link it. A question it asks is the owner's to answer in that session. Follow-ups go to that session, never to a new one.

## Do not
- Do not do the ticket work in this session: no code change, no specification, no Jira write. The ticket sessions own all of it.
- Do not start a session for a finished ticket, for one that is already running, or beyond the limit.
- Do not merge, push or archive anything.
- Do not write the repository map without a yes, and never write anything else outside the session's folder.
- When the host cannot start sessions (a terminal CLI without that capability, for example), start nothing: give the commands to paste, one per ticket, each in its own session.

## Report
- One line per ticket: started (with a link to its session and the folder it works in), queued, skipped (and why), or finished.
- The rules added to the repository map, if any.
- The number of ticket sessions running against the limit.
- End every reply addressed to a person with the `### 📋 Status Update` block (Done, Remaining (Agent), Pending (User)), written in the language of the conversation.
