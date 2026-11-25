---
name: daily
description: "Morning brief on your Jira tickets from the sources you configured once (what waits on you, what is in flight, which new tracker tickets should enter the sprint or move up in priority, the next tickets to run), then the day's master session that starts and tracks one session per ticket."
---
# Daily Brief

## Ticket access
This skill comes from the Sectile Lite plugin: there is no Sectile server, agent or MCP behind it. Jira is the source of truth for the ticket; the forge (GitLab or GitHub) holds the code and its pull requests.
- Read and write the ticket through the Atlassian connector the session exposes (an Atlassian or Jira MCP server, such as the claude.ai Atlassian connector). Discover its actual tool names from the session; do not invent one. Its writes run with the user's own Jira access.
- The argument names a Jira key such as `PE-123`, or its URL. Read the issue and check its key and URL before any write: a bare number names no ticket.
- Never call a Sectile MCP server's tools, even when one is connected to this session (the full Sectile plugin installed beside this one, for example). They write through the Sectile server, which moves the Jira status along with the stage label: a Lite run touches Jira only through the Atlassian connector, as below.
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

## Goal
Give the user their morning brief on their tickets: what waits on them, what is in flight, what moved since the last brief, which new tickets of the tracker should enter the sprint or move up in priority, and the next tickets to run today with the command that runs each. Then stay open as the day's master session: start the tickets the user picks, one dedicated session each as the dispatcher does, and relay what they reach until the day ends. Which tickets the brief reads is the user's own configuration, asked once and remembered, never a team default.

## Read first
- The configuration, `~/.config/sectile-lite/daily.json`, when it exists. It belongs to this workstation and remembers where the brief reads tickets:

  ```json
  {
    "site": "example.atlassian.net",
    "sources": [
      { "name": "Mine", "jql": "assignee = currentUser() AND statusCategory != Done ORDER BY Rank ASC" },
      { "name": "Team sprint", "jql": "filter = 12345 AND sprint in openSprints()" }
    ],
    "triage": {
      "jql": "project in (ABC) AND statusCategory != Done AND (sprint is EMPTY OR sprint not in openSprints())",
      "sprintField": "customfield_10020"
    },
    "limit": 3,
    "next": 5,
    "staleDays": 5,
    "lastRunAt": "2026-11-24T08:30:00+01:00"
  }
  ```

  `site` is the Jira site. `sources` are the queries the brief reads, in order: a JQL query, a Jira filter (`filter = <id>`) or the query of a Jira board's filter, each with a short name. `triage.jql` is the triage pool, the tracker query whose new tickets the brief triages: into the sprint, up in priority, or left in the backlog; without it there is no triage. `triage.sprintField` is the id of the site's Sprint field, found on the first triage. `limit` is how many ticket sessions run at once (the dispatcher's limit). `next` is how many tickets the "Next" list holds. `staleDays` is when a ticket in flight with no update counts as stale. `lastRunAt` is when the last brief ended: "since the last brief" means since then.
- The team configuration, `~/.config/sectile-lite/config.json`, written by the `setup` skill, when it exists: its `site` and its `team` (`name`, `projects`, `jql`) are the defaults the configuration offers.
- The argument, when there is one: `configure` to change the configuration, `dry-run` to brief without starting anything nor saving `lastRunAt`, or Jira keys to start right away after the brief.
- The user's Sectile Board, when the host lists one of their pages titled `Sectile Board`: its configuration carries the site and the query the user already chose for it.
- The repository map of the dispatcher, `~/.config/sectile-lite/repositories.json`, and the sessions this session already started, from the host's list of linked sessions.
- Today's date and time, from the system clock. Never guess them.

## Steps
1. **Title the session** `DAILY · <YYYY-MM-DD>` when the host lets a session rename itself, so the day's master session is easy to find.
2. **Load or ask for the configuration.** With `daily.json` present and no `configure` argument, use it without asking. Otherwise ask once, in one message, offering what is already known as the default:
   1. the Jira site: the `site` of `config.json`, else from the sites the Atlassian connector reaches (the only one when there is one);
   2. the sources: the Sectile Board's query when there is a board, else `assignee = currentUser() AND statusCategory != Done ORDER BY Rank ASC`; the user may give other queries, a Jira filter id or a Jira board, whose filter the brief then reads (`filter = <id>`). When `config.json` names a team, offer its `team.jql` as a second source named after the team;
   3. the triage pool: the tracker query whose new tickets may enter the sprint or move up in priority. Offer the team's `projects` from `config.json`, else the projects of the sources, not done and outside the open sprints, `project in (<projects>) AND statusCategory != Done AND (sprint is EMPTY OR sprint not in openSprints())`; the user may give their team's intake query, a Jira filter or a board's backlog instead. No pool means no triage;
   4. the limit (3), the size of the "Next" list (5) and the stale delay in days (5).

   Check each query once with the connector before keeping it: a query that fails is shown with its error and asked again. Then show the file as it would be written and write it only on a yes, keeping any key already there. Without a yes, brief with the answers for this run only. Never write a team's project, board or query in it unless the user chose it.
3. **Read the tickets.** Run every source, merge the results by key and keep each ticket's summary, status, priority, assignee, due date, sprint, labels, components, last update, its remote links (pull requests) and the comments added since `lastRunAt` (or the last 24 hours on a first run). The stage is the ticket's `#<stage>` label, `#new` when it has none.
   Read the triage pool too, narrowed to the tickets created or updated since `lastRunAt` (the last 7 days on a first run), with the same fields plus reporter, issue type, links and creation date. Find the Sprint field as the board skill does (the custom field named `Sprint`, its id kept as `triage.sprintField`) and the open sprint: the one the in-flight tickets of the sources belong to; with several, ask which one, once per run.
4. **Sort them into the brief's sections:**
   - **Needs me**: tickets where someone waits on the user: a comment since the last brief that mentions them or asks them a question, a stage report that asks the owner to decide, a pull request waiting on their review, a ticket session of this dispatcher waiting for their answer.
   - **In flight**: tickets at `#clarified`, `#specified`, `#implemented` or `#reviewed`, with their stage, their pull request and how long since their last update; mark as stale those beyond `staleDays`.
   - **Moved since the last brief**: stage changes, tickets that reached `#finished` or a done status, tickets that entered or left the sources.
5. **Triage the newcomers.** For each ticket of the triage pool, propose one decision with a one-line reason drawn from the ticket itself, never from a guess:
   - **Into the sprint**: a production incident or a bug with user impact, a ticket that blocks a ticket in flight or another team, a due date before the open sprint ends, a requester waiting with a date;
   - **Raise priority** (to a named level): a ticket that matters more than its priority says, by the same signals, but that does not need to start before the next sprint;
   - **Leave in the backlog**: everything else, or a ticket too vague to judge, which is marked "to clarify" instead.

   Weigh each "into the sprint" against what the sprint already holds: say what it would push out when the sprint is full. Propose no more entries than the user can absorb, at most 3 per run unless they ask for more.
6. **Write the brief** in the language of the conversation, every ticket with its key as a link, its summary and its stage: Needs me, Triage (one line per ticket: decision, reason, current priority and sprint), In flight, Moved since the last brief. Add the Sectile Board's link when there is one.
7. **Apply the triage the user approves.** Ask once for the whole batch: the user approves it, edits some decisions or skips it. Then, for each approved ticket, set the open sprint in its Sprint field or its new priority with the connector's `editJiraIssue`, read the ticket back to check the change, and add a short Jira comment giving the reason (`Daily triage: <reason>`). Report each write that failed with its exact content. In `dry-run`, or without a pool, skip this step.
8. **Rank the next tickets.** At most `next` tickets to run today, each with its command, derived from the stage as the dispatcher does (none or `#new` → `clarify-issue`, `#clarified` → `specify-issue`, `#specified` → `implement-issue`, `#implemented` → `adjust-issue`, `#reviewed` → `handoff-issue`) and a one-line reason. Tickets the triage just brought into the sprint count as candidates. Rank by: a ticket that blocks someone, or a date that falls today or tomorrow (due date, sprint end); then finishing what is in flight before starting anything new, the furthest stage first; then priority; then the order of the sources. Never list a finished ticket, or one whose session is still running.
9. **Ask what to start.** One question: which of the "Next" tickets to start now, the top ones up to `limit` offered first, with "none" as an answer. Keys given as the argument are already the answer. In `dry-run`, ask nothing and start nothing.
10. **Become the day's dispatcher.** For the tickets picked, run the `dispatch` skill of this plugin with their keys and commands (in Claude Code, the Skill tool with `sectile-lite:dispatch`; otherwise read the sibling `../dispatch/SKILL.md` and follow it), in this session, so the ticket sessions are linked to it. Keep to `limit` and queue the rest.
11. **Remember the run.** Outside `dry-run`, set `lastRunAt` in `daily.json` to now once the brief is written, when the file exists, and `triage.sprintField` when it was found. Those are the only keys the skill changes without asking.
12. **Stay the master session for the day.** When a ticket session reports back, relay it in one line and start the next queued ticket. When the user asks for the brief again ("brief", "refresh", "what's next"), run steps 3 to 9 again with the configuration already loaded and only the changes since the morning, triage included.

## Do not
- Do not do the ticket work in this session: no code change, no specification, no stage change. The ticket sessions own all of it, as under the dispatcher.
- Do not write to Jira beyond the triage the user approved in this run: the Sprint field, the priority and the comment giving the reason, on the approved tickets only. Never change a status, an assignee, a label or an estimate, and never close or create a ticket.
- Do not write `daily.json` or the repository map without a yes, except `lastRunAt` and `triage.sprintField` as in the steps, and never write anything else outside the session's folder.
- Do not assume a team, project, board or query: every source comes from the user's configuration or answers.
- Do not start a session the user did not pick, for a finished ticket, for one already running, or beyond the limit.
- Do not merge, push or archive anything.
- When the connector cannot reach Jira, brief nothing from memory: say which source failed and stop.

## Report
- The brief: Needs me, Triage, In flight, Moved since the last brief, Next, each line with the ticket's key as a link, and the Sectile Board's link when there is one.
- The triage applied: each ticket moved into the sprint or raised in priority, each write that failed, and what was left in the backlog.
- The sources read, those that failed, and whether the configuration was created, changed or used as it was.
- The ticket sessions started (with their links and folders), queued and skipped, against the limit.
- End every reply addressed to a person with the `### 📋 Status Update` block (Done, Remaining (Agent), Pending (User)), written in the language of the conversation.
