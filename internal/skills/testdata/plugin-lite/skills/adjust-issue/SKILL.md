---
name: adjust-issue
description: "Review the branch like a peer would, fix what the review finds, then update the existing merge request and leave the merge to the user."
---
# Adjust Existing Pull Request

Stage: implemented -> reviewed.

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
- **Next step**. When the skill ends with `✅`, finish the reply with the next step, ready to copy, with the full task ID and the command name the skills were invoked under (`/sectile-lite:<skill>`): review and merge the pull request, then `/handoff-issue <task ID>`. When it ends on `❓` or `❌`, the next step is what the owner has to answer or fix; say that instead.
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
Hand a reviewer a branch that is already worth reading: the obvious problems
found and fixed, the risky parts pointed out, the test plan written down.

## Read first
- The full diff of the branch against the default branch. All of it, not the summary.
- The specification, to check that what was asked is what was built. When its files are ignored by
  Git (dropped artefacts), they are not in the diff: read them from the worktree, or from the
  clarification and specification reports on the ticket.
- The current remote default branch: fetch the remote and identify its configured
  default branch before reviewing or publishing.

## Steps
1. Verify a matching PR exists for the task repository and branch before changing files: open, or already merged by the human. Record its URL. If missing, stop and recover through the configured creation owner (specify or implement). Never create a PR during adjustment, and never push onto a merged PR — review the merged state and report it. Read available PR feedback; retrieval failure is a blocker, not absence of feedback.
   A task that changed several repositories (`$SECTILE_REPOSITORIES` role `changed`) has one PR per repository: verify, review, push and update each of them in its own worktree, the same way, and give every secondary repository's PR to `transition_stage` in `prUrls`.
   Fetch the remote (`git fetch origin`) and compare the work branch with the
   remote default branch (normally `origin/main`; use the repository's configured default when different).
   Integrate missing base commits before the final review: prefer rebase when the branch is private, or merge when
   repository policy or shared-branch state requires it. Resolve conflicts and do not continue until the working tree is clean.
2. Review the complete resulting diff against the specification for correctness, side effects, security, and edge cases with no test. Address actionable feedback and record dispositions. No human feedback is required.
3. Update documentation affected by the change. Fix what the review finds, now. A known defect belongs in the code, not in the
   description of the merge request.
4. Re-run build, static analysis and tests after integrating the default branch and on the final state.
5. Commit with a conventional message: type, scope, and why the change exists. Never force-add a
   specification artefact that Git ignores (`git add -f`): the project drops them on this workstation.
6. Push the branch and update the same existing merge request: summary, test plan, and the specific
   places where you want a reviewer's eyes.
   Run `git fetch origin`, then choose the push from the state of `origin/<branch>`:
   - `origin/<branch>` does not exist (first publication): run `git push -u origin <branch>`. Never force a branch the remote does not have.
   - `git merge-base --is-ancestor origin/<branch> HEAD` succeeds (fast-forward): run a plain `git push`.
   - Otherwise an authorized rebase rewrote published history: run `git push --force-with-lease`.
   If the push is refused because commits landed on `origin/<branch>` in between (stale lease or non-fast-forward), run `git fetch origin`,
   replay the local commits with `git rebase origin/<branch>` so the remote commits are kept (merge instead if the conflicts cannot be resolved safely),
   re-run the checks if new commits came in, and retry once with the same rule. If it is refused again, or for another cause
   (branch protection, permissions, authentication), stop, keep the work and report the blocker. Never run an unguarded `git push --force`.
7. Verify the same PR is open and contains the pushed final commit, update its description and check evidence, then mark it ready. If any check, feedback retrieval, push or readiness verification fails, preserve work and report the blocker. If the repository has no remote, stop.

## Do not
- Do not merge, do not approve, do not close the ticket. That is the user's call.
- Do not create a PR. Do not mark a PR ready on a red build. Report the failure instead.
- Do not complete adjustment on a branch known to be behind the remote default branch.

## Report
- What the review found, and which findings you fixed.
- The merge request URL, or why there is none.
- The test plan a reviewer can replay, as a checklist.

## Execution and ticket state
- **Stage label**: the stage of a ticket is one Jira label, Sectile's own: `#new`, `#clarified`, `#specified`, `#implemented`, `#reviewed` or `#finished`, lowercase; no such label means `#new`. Sectile and the Sectile board read the same labels, so a ticket moved by either shows the same stage. A ticket carries one: setting a stage removes every other stage label in the same edit, whatever its case and with or without its `#` (`Clarified`, `#Reviewed`, `#untouched`, `#closed`), and leaves every other label as it is. Never change the Jira status, which belongs to the team's workflow.
- **Recording a stage**: after verifying each completed step, edit the issue's labels to the new stage, then post the stage report as one Jira comment headed `Sectile · <stage>`, carrying the structured report, the actual branch and, when there is one, the pull request URL. Read the issue back to check the label. If the label edit is refused, post the comment anyway, say the label is pending and give it for the owner to apply; never claim a stage the ticket does not show.
Move implemented → reviewed only when this step is complete.
The stage comment carries the verified pull request URL, and the pull request of every other repository the task changed.
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
