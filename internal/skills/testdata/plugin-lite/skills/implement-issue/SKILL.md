---
name: implement-issue
description: "Implement the ticket from its specification and prove it works with the project's own build, linters and tests."
---
# Implement Code

Stage: specified -> implemented.

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
- **Changes.** When the skill ends after changing code, show the session's diff pane, provided it covers the worktree the skill worked in; otherwise name the worktree path in the reply instead.
- **Next step**. When the skill ends with `✅`, finish the reply with the next step, ready to copy, with the full task ID and the command name the skills were invoked under (`/sectile-lite:<skill>`): `/adjust-issue <task ID>`. When it ends on `❓` or `❌`, the next step is what the owner has to answer or fix; say that instead.
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
The ticket's issue artefacts are its clarification report (`docs/clarifications/<n>.md`, where `<n>` is the ticket key), its specification (`specs/<KEY>-<slug>/` or `openspec/changes/<KEY>-<slug>/`) and its working notes (`notes/<KEY>.md`). They live in the workstation's ticket workspace, not in the code repository: the `workspace` of `~/.config/sectile-lite/config.json`, set by `/sectile-lite:setup`, `{"path": "<folder>", "sync": true, "framework": "speckit"}`.
- **Where**: with `sync` true, the workspace is a Git clone shared with the team. Work in a worktree of it per ticket, `<path>/.tasks/worktrees/<KEY>`, on a branch named like the ticket's code branch: create it from the up-to-date default branch (`git -C "<path>" fetch origin`, then `git -C "<path>" worktree add -b <branch> .tasks/worktrees/<KEY> origin/<default>`), or from `origin/<branch>` when that branch exists, and reuse it as it is afterwards. Add `.tasks/` to the workspace's `.git/info/exclude` when it is missing. With `sync` false, write in `<path>` itself, on its current branch. Without a workspace configured, or when its path is missing, the task worktree is the workspace, as before: say so in the report and suggest `/sectile-lite:setup`.
- **Framework**: the project-configured SDD framework is the workspace's `framework` (`speckit` or `openspec`); the repository detection runs in the workspace. Read and write the artefacts under the workspace path, never relative to the session directory.
- **Notes**: read `notes/<KEY>.md` first when it exists: the owner may have written context, decisions or constraints there. Every stage appends one dated section, `## <stage> - <YYYY-MM-DD>`, with the decisions made, the open points and the links (pull request, Jira comment). Never rewrite or remove what is already there.
- **Commit and publish**: in a Git workspace, stage only the ticket's files (`git -C "<workspace>" add <files>`, never `git add -A`) and commit with the stage's message. With `sync` true, push the branch with `git -C "<workspace>" push -u origin <branch>` the first time and a plain push afterwards; never force and never push the default branch. A refused push is reported and does not block the stage. Opening the merge request of that branch is the owner's gesture: say the branch is pushed and give the link to open it. With `sync` false, commit locally and push nothing. In a folder that is not a Git repository, write in place and run no `git` command there. The dropped artefacts check (`git check-ignore -q`) runs in the workspace.
- With a workspace, the code branch carries no artefact: open no pull request at the clarified or specified stage, even when the project creates it there, say so in the report, and create the draft after implementation. The code pull request description names the workspace's repository and branch when it is synced.
- **Review in Claude**: the owner reviews the artefacts as Markdown in Claude. Each time a stage writes or changes one, show it: give its absolute path as a link and, in the Claude desktop app, open it for the owner (when the workspace is outside the session's folders, ask the host to add the workspace folder to the session first, so its file pane can open it, then send or show the file with the host's file tools). When the stage waits for the owner's confirmation, ask for it once the file is open.

## Goal
Ship the change described by the specification, in code that reads like the code
already there, with the project's checks green.

## Read first
- The specification and its task checklist. It is the contract, follow its order.
  Its files may be ignored by Git (`git check-ignore -q` succeeds on them): the project drops its
  specification artefacts on this workstation. Read them from the worktree, never commit them and
  never force them with `git add -f`. When the project drops its artefacts (the launch prompt says so,
  or the paths are ignored) and the specification is missing from the worktree, stop and report that
  it is not available on this workstation: never rewrite it.
- The surrounding code: naming, error handling, comment density, test style. Match it.
- How this project builds and tests. Find the real commands, do not assume them.

## Steps
1. Reuse the assigned worktree and branch, including a shared batch branch. Never implement on the default branch.
2. When the task has more than one folder, `$SECTILE_REPOSITORIES` (and the prompt's folder
   block) lists them with their role: the project's repositories, the folders attached to the
   project on this workstation, and the specifications folder. Work in the primary worktree.
   A `context` folder is read-only: to change it, call `prepare_repository_worktree` for its
   repository first and work in the worktree it returns. That repository then has role
   `changed` and needs its own pull request, given to `transition_stage` in `prUrls`. A
   `local` folder (attached, no remote) is changed in place, with no worktree and no pull request.
   A repository the task needs but no folder holds is refused unless the workstation has the
   project's Any repository option on: then pass the top level of a local checkout of it as
   `path`, or omit `path` to let the agent clone it. When the launch says no worktree was
   created in the code repository, prepare it the same way before changing it; left
   unchanged, it needs no pull request. When the answer says the worktree was not added to
   the session, ask the user to run `/add-dir` with its path before writing there.
3. Work through the checklist in small steps, each one leaving the tree buildable. When the
   specification artefacts are ignored by Git, commit the code only and never force-add them.
4. Add the tests that cover the new behaviour and its edge cases, not just the
   happy path. A change with no test needs a stated reason.
5. Run build, static analysis and tests. Fix until green, and quote the real output.
6. Re-read your own diff before finishing, as a reviewer would.

## Recovery and blockers
- Repair routine technical issues and update design/tasks when the implementation
  needs to change while preserving acceptance criteria. Continue after documenting why.
- Establish whether a failing test predates the change. Fix failures in scope; report
  unrelated failures with baseline evidence. Never hide them or mark checks green.
- Stop only for an essential product decision, an unavailable dependency after
  bounded recovery attempts, or work that materially expands the requested scope.
- Preserve the work branch, completed checklist items and remaining next action so
  a retry can resume instead of starting over.

## Report
- What changed, file by file, and why.
- The real output of build, linters and tests, remaining failures included.
- What you deliberately left out, and what it would take to finish it.

## Execution and ticket state
- **Stage label**: the stage of a ticket is one Jira label, Sectile's own: `#new`, `#clarified`, `#specified`, `#implemented`, `#reviewed` or `#finished`, lowercase; no such label means `#new`. Sectile and the Sectile board read the same labels, so a ticket moved by either shows the same stage. A ticket carries one: setting a stage removes every other stage label in the same edit, whatever its case and with or without its `#` (`Clarified`, `#Reviewed`, `#untouched`, `#closed`), and leaves every other label as it is. Never change the Jira status, which belongs to the team's workflow.
- **Recording a stage**: after verifying each completed step, edit the issue's labels to the new stage, then post the stage report as one Jira comment headed `Sectile · <stage>`, carrying the structured report, the actual branch and, when there is one, the pull request URL. Read the issue back to check the label. If the label edit is refused, post the comment anyway, say the label is pending and give it for the owner to apply; never claim a stage the ticket does not show.
Move specified → implemented only when this step is complete.
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
