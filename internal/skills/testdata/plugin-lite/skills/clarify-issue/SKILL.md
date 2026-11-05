---
name: clarify-issue
description: "Analyse a ticket against the code, iterate in clarification rounds, and resolve open questions until the owner confirms satisfaction."
---
# Clarify Issue

Stage: new -> clarified.

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

## Session title
- As soon as the ticket is identified, and before doing the work, rename the current session to `<ticket ID> - <ticket title>`, for example `#47 - Remove the parallelism setting`. The ID and title stay the same for the whole run: only a leading status emoji is added or removed.
- The status emoji mirrors the run state, at the same moment and at no other. While the skill works the title carries none: the host already shows whether the session is running.
  - `❓` right before a question the skill cannot continue without, and when the skill stops with open questions for the owner. Remove it when the work resumes.
  - `✅` once the skill reached its goal.
  - `❌` when the skill stops on a failure or a blocker it cannot resolve.
- When the skill runs nested in pickup-issue or pickup-issues, do not rename the session: the outer skill owns the title and its status.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, or a rename is refused or left unapproved, keep the current title and continue silently. It never blocks, delays or replaces the work of the skill.

## Session links
- Right after renaming the session, write one line in the conversation that links the ticket: `Ticket: [<ticket ID>](<external URL>)`, with the external URL of the Jira issue. Skip the line when the ticket has no external URL.
- When the skill creates or finds a pull request or merge request, write one line: `PR: [<number>](<URL>)`. On a GitHub pull request, when the host can bind the session to a pull request (the Claude desktop app's PR binding tool, for example) and the session does not show it already, bind it too. Elsewhere, GitLab included, the line is the only link.
- When the skill runs nested in pickup-issue or pickup-issues, write neither line: the outer skill writes the ticket line, and the PR line once a stage returns with a new pull request.
- If the host cannot bind a pull request, or the binding is refused, keep the line and continue. Links never block, delay or replace the work of the skill.

## Session experience
These make the session read like a Sectile Desktop execution. Use whatever the host exposes for each (in the Claude desktop app: its sidebar group, chapter, pane and notification tools); when the host has no such capability, or a call is refused, skip that item silently. None of them ever blocks, delays or replaces the work of the skill.
- **Project group.** Right after the session links, file the current session under the sidebar group named after the Sectile project (`projectName` from get_project_context). Reuse an existing group with that exact name; create it only when none exists. Move only the current session.
- **Chapter.** When the skill starts in a session that already holds earlier work, mark a chapter titled after the skill and the ticket ID, for example `Clarify #47`. Do not mark one when the skill is the first thing the session does.
- **Next step**. When the skill ends with `✅`, finish the reply with the next step, ready to copy, with the full task ID and the command name the skills were invoked under (`/sectile-lite:<skill>`): `/specify-issue <task ID>`. When it ends on `❓` or `❌`, the next step is what the owner has to answer or fix; say that instead.
- **Notification**. When the run reaches `❓`, `✅` or `❌`, send one desktop notification, under 200 characters, leading with what the owner has to do (for example `#47 waits for your answer: 2 product questions`). Send none for routine progress; the host drops it anyway when the owner is watching.
- When the skill runs nested in pickup-issue or pickup-issues, do none of the above: the outer skill owns the group, the chapters, the pane, the next step and the notifications.

## Status update
Wherever the skill runs, end every reply addressed to a person with this block, including the final report and a reply that stops on a question. Keep the three labels as written; write the items in the language of the conversation, and write "None" for an empty line. A headless run, with nobody to read it, writes none.

```markdown
### 📋 Status Update

- **Done**:
  - <what was done in this reply>
- **Remaining (Agent)**:
  - <what is left for the agent, or None>
- **Pending (User)**:
  - <what the user has to do or decide, or None>
```

When the skill runs nested in pickup-issue or pickup-issues, do not write the block: the outer skill writes one for the whole run.

## Specifications workspace
The task's clarification report (`docs/clarifications/<n>.md`) and specification (`specs/<KEY>-<slug>/` or `openspec/changes/<KEY>-<slug>/`) are its issue artefacts. Without a Sectile server there is no separate specifications repository: they live in the task worktree, on the task branch, and are committed with the stage's message like the code. The dropped artefacts check (`git check-ignore -q`) runs there too.

## Goal
Turn an ambiguous ticket into a settled specification baseline. Clarification is an
iterative dialogue with the work item's owner: execute in rounds until the owner
explicitly confirms that the clarification is satisfactory.

## Read first
- The ticket: title, description, comments via get_task, parent epic if present.
- The existing clarification report if one exists: docs/clarifications/<n>.md on the assigned work branch,
  or, when the report is ignored by Git (step 6), in the worktree and in the rounds already
  published on the ticket.
- The code the change would touch. Name the files you actually read.
- Neighbouring features that already solve a similar problem in this codebase.

## Steps
1. Re-read the assigned branch and worktree. If docs/clarifications/<n>.md already exists,
   this run continues an existing clarification into Round N. If not, this is Round 1, unless the
   report is ignored by Git and the ticket already carries published rounds (step 6).
2. In Round 1:
   a. Restate the request in two sentences, including what is out of scope.
   b. List ambiguities, worst first. Only list an ambiguity if two readings lead to different code.
   c. Name critical dependencies: other services, migrations, missing data, third-party limits.
   d. Resolve reversible technical choices using existing code and project conventions.
   e. Formulate essential product questions that alter acceptance criteria, with your recommended option.
   f. Write docs/clarifications/<n>.md, commit with docs(spec): clarify #<n> (round 1), unless the file
      is ignored by Git (step 6).
   g. Publish the round as described below, then ask any open questions interactively
      when the owner is present or in the ticket discussion when unattended.
3. In Round N (follow-up after owner answers):
   a. Read the owner's answers from the interactive prompt or ticket comments via get_task.
   b. Append a dated section: "## Round N - answers from the owner (<date>)" to docs/clarifications/<n>.md.
   c. Explicitly record settled choices and any reversed prior assumptions.
   d. Address newly surfaced ambiguities or dependencies.
   e. Commit updates with docs(spec): clarify #<n> (round N), unless the file is ignored by Git (step 6).
   f. Publish the round as described below. If follow-up product questions remain,
      ask them and stop without transitioning.
4. Exit condition:
   Rounds continue until the owner confirms that the clarification is satisfactory (or zero open
   product questions remain in unattended pickup). Never transition new → clarified while product
   questions remain open.
5. Persist the settled scope, decisions, and assumptions in the report before concluding.
6. Dropped artefacts: `<n>` is the task key without its leading `#` (`487` for `#487`). Before
   committing, run `git check-ignore -q docs/clarifications/<n>.md`. When it succeeds, the project
   drops its specification artefacts on this workstation: write and update the file in the worktree,
   never commit it, never force it with `git add -f`, and include the settled decisions
   in the round section used as the transition note, saying that the report file stays local to the worktree.
   The published rounds are then the only shared record. A fresh worktree does not hold a report
   written elsewhere: when the file is ignored and missing, read the rounds already published on
   the ticket (the Clarification Report comments and each `## Round N` section in them), rebuild the
   file from them in order, and continue with the next round. Never restart at Round 1 while the
   ticket carries a published round.

7. Optional stage publication: read `pushStageCommits` from `get_project_context`
   (or the supplied project configuration); missing or false means off. When true,
   after each stage commit push the actual assigned work branch with a plain push.
   Use `git push -u origin <branch>` on first publication and `git push origin <branch>`
   afterwards. Never force. Report a refused push and continue the stage; retain the
   commit for retry. Ignored artifacts are never committed or force-added, so writing
   them alone triggers no push. This setting does not replace required PR publication.

### Publish every clarification round

- Every round, interactive or unattended, publishes its section in full: Round 1
  uses the whole initial report; Round N uses only its newly appended section.
  Retain the Markdown file as the chronological history, even when it stays local.
  Include the report path, commit/local status, settled decisions, open questions
  and publication failures in that section; a path or summary alone is insufficient.
- Standalone intermediate rounds with open questions use `add_comment`. The final
  round uses its full section as the `transition_stage` note, with no separate
  `add_comment` for the same content. Verify each response before claiming publication.
- Managed runs call no comment or stage tool: put the full round section in the
  supplied result note and let Sectile publish it through its completion contract.
- If a section exceeds the tracker comment limit (GitHub 65,536 characters; Jira
  about 32,767), split at Markdown paragraph boundaries into numbered parts, reserving
  space for the server header and part numbering. Use at most 30,000 characters per
  part for either tracker, and split an oversized paragraph without dropping text.
  Standalone intermediate parts use `add_comment` in order. For a final round, post
  all preceding parts with `add_comment` and use only the last numbered part as the
  transition note, so each part appears once. Managed runs keep the complete section
  in the result note and report any completion-contract size limitation rather than
  bypassing the contract.


## Do not
- Do not transition new → clarified while any product question or decision remains open.
- Do not invent answers to essential product questions in unattended runs; record them and ask.
- Do not write production code or start the technical specification at this stage.
- Do not discard previous round sections when writing Round N; append each round chronologically.
- Do not switch branches or create a new branch: reuse the assigned work branch.

## Report
- The report path: docs/clarifications/<n>.md, and whether it is committed or local to the worktree (ignored by Git).
- Current round number and whether the exit condition was met.
- Settled decisions and reversed assumptions.
- Numbered open questions (if any) and who is expected to answer them.
- Stage transition status (applied or blocked awaiting answers).
- Full content of this round (initial report or new section), publication result and any numbered parts.
- Optional stage push result, including refusals that did not block the stage.


## Execution and ticket state
- **Stage label**: the stage of a ticket is one Jira label, `sectile:clarified`, `sectile:specified`, `sectile:implemented`, `sectile:reviewed` or `sectile:finished`; no such label means `new`. The Sectile board reads these labels. A ticket carries at most one: setting a stage removes the previous `sectile:` label in the same edit and leaves every other label as it is. Never change the Jira status, which belongs to the team's workflow.
- **Recording a stage**: after verifying each completed step, edit the issue's labels to the new stage, then post the stage report as one Jira comment headed `Sectile · <stage>`, carrying the structured report, the actual branch and, when there is one, the pull request URL. Read the issue back to check the label. If the label edit is refused, post the comment anyway, say the label is pending and give it for the owner to apply; never claim a stage the ticket does not show.
Move new → clarified only when the exit condition is met: the owner confirms the clarification is satisfactory (or zero product questions remain open in unattended pickup). Never move new → clarified while any product question or decision remains open.
- **Pull requests**: link each pull request from the Jira issue (a remote link), once per repository, and repeat its URL in every later stage comment. A task whose work changed no repository has no pull request to give: the stage comment says what was done instead.
- Intermediate discussion, such as a clarification round with open questions, is a plain Jira comment, never a stage comment.
- Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.

## Project pull request policy
Read prCreationStage from get_project_context before executing.

### When prCreationStage is "clarified"
In the final clarification round only (the owner confirmed the clarification, or no product question remains open in an unattended run), after committing the report, push the task branch with `git push -u origin <branch>` (never force), discover and reuse its PR/MR or create a draft when absence is confirmed, and include its URL as prUrl in the clarified transition. Intermediate rounds open no PR. Later stages push to the same branch and update the same PR/MR: include it as prUrl in the specified and implemented transitions; when a later stage finds no PR for the branch (a task clarified before this setting), create the draft when absence is confirmed. Keep it draft until adjustment; preserve an existing ready PR. Lookup failure is not absence. When the clarification report or the specification files are ignored by Git (dropped artefacts), open no PR at those stages, say so in the report, and create the draft after implementation.

### When prCreationStage is "specified"
After the specification is written and validated, commit and push the specification on the task branch and open a draft PR/MR for specification review. Reuse an existing PR/MR for that branch. Include its URL as prUrl in the specified transition. Keep newly created PRs draft while implementing; preserve an existing ready PR; update the same PR/MR and mark it ready only after implementation and review. Do not mark the task reviewed merely because a draft exists. When the specification files are ignored by Git (dropped artefacts), open no PR at this stage, say so in the report, and create the draft after implementation.

### Otherwise ("implemented" or empty)
After successful implementation checks, commit and push the branch, discover and reuse its open PR/MR or create a draft when absence is confirmed. Include its URL as prUrl in the implemented transition. Do not create one during specification or adjustment. Lookup failure is not absence. Preserve an existing ready PR.
