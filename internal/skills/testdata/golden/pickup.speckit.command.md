---
description: "Pick a ticket and autonomously execute all development steps up to Pull Request creation."
argument-hint: <TICKET-KEY> [contexte]
---
# Pickup Issue (Auto-Pilot to PR)

Stage: new -> reviewed.

## Sectile task access
- Use the local Sectile agent's exposed task-management interface first for ticket reads, updates, comments, creation, and workflow results. Discover its actual tools or documented commands from the session/project context; do not invent an endpoint or launch another agent daemon as a substitute.
- Resolve the project against its repository, then verify the task's full ID and external URL. A bare key such as #47 can match another project's ticket. Use the full task ID for mutations and an explicit project ID for creation.
- If the local agent interface is unavailable or fails after a bounded attempt, use http://localhost:8090 as a temporary fallback. Record the missing capability or error, check for an existing bug in the same project, and register or update that bug when authorized. If reporting is unavailable or not authorized, preserve the report locally and state what remains pending. Do not bypass Sectile by writing directly to its database or remote tracker.
- For a managed run, submit only through its supplied result contract and let Sectile validate and synchronize the result. An active run without a usable completion contract is a reportable integration failure. Preserve the artifacts and report the blocked transition; do not cancel the activity, forge launch/completion status, or use another endpoint to evade result validation.
- This routing does not authorize mutations excluded by the skill or user request. Verify each mutation's response and report partial success explicitly.

## Session title
- As soon as the ticket is identified, and before doing the work, rename the current session to `<ticket ID> - <ticket title>`, for example `#47 - Remove the parallelism setting`. The ID and title stay the same for the whole run: only a leading status emoji is added or removed.
- The status emoji mirrors the run state the skill reports to Sectile, at the same moment and at no other. While the skill works the title carries none: the host already shows whether the session is running.
  - `❓` with report_waiting true, right before a question the skill cannot continue without, and when the skill stops with open questions for the owner. Remove it with report_waiting false, when the work resumes.
  - `✅` with finish_run completed, once the skill reached its goal.
  - `❌` with finish_run failed, when the skill stops on a failure or a blocker it cannot resolve.
- When the skill runs nested in pickup-issue or pickup-issues, do not rename the session: the outer skill owns the title and its status.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, or a rename is refused or left unapproved, keep the current title and continue silently. It never blocks, delays or replaces the work of the skill. A host that cannot rename, such as a Sectile Desktop console, shows the same state from the Sectile calls themselves.

## Launched by Sectile
A run is launched by Sectile when the invocation supplies a launch runId or a result-file contract, or when the environment carries `SECTILE_RUN_ID` (read it with a plain shell command such as `printenv SECTILE_RUN_ID`). Sectile Desktop then already shows the ticket, the pull request, the next step and its own notifications, so the items marked "not when launched by Sectile" below are skipped: they would only repeat what Desktop shows.

## Session links
Not when launched by Sectile.
- Right after renaming the session, write one line in the conversation that links the ticket: `Ticket: [<ticket ID>](<external URL>)`, with the external URL Sectile returns for it. Skip the line when the ticket has no external URL.
- When the skill creates or finds a pull request or merge request, write one line: `PR: [<number>](<URL>)`. On a GitHub pull request, when the host can bind the session to a pull request (the Claude desktop app's PR binding tool, for example) and the session does not show it already, bind it too. Elsewhere, GitLab included, the line is the only link.
- When the skill runs nested in pickup-issue or pickup-issues, write neither line: the outer skill writes the ticket line, and the PR line once a stage returns with a new pull request.
- If the host cannot bind a pull request, or the binding is refused, keep the line and continue. Links never block, delay or replace the work of the skill.

## Session experience
These make the session read like a Sectile Desktop execution. Use whatever the host exposes for each (in the Claude desktop app: its sidebar group, chapter, pane and notification tools); when the host has no such capability, or a call is refused, skip that item silently. None of them ever blocks, delays or replaces the work of the skill.
- **Project group.** Right after the session links, file the current session under the sidebar group named after the Sectile project (`projectName` from get_project_context). Reuse an existing group with that exact name; create it only when none exists. Move only the current session.
- **One chapter per stage.** Before invoking each stage skill, mark a chapter titled `<Stage> <ticket ID>`, for example `Specify #47`.
- **Changes.** When the skill ends after changing code, show the session's diff pane, provided it covers the worktree the skill worked in; otherwise name the worktree path in the reply instead.
- **Next step** (not when launched by Sectile). When the skill ends with `✅`, finish the reply with the next step, ready to copy, with the full task ID and the command name the skills were invoked under (`/sectile:<skill>` when installed as a plugin): review and merge the pull request, then `/handoff-issue <task ID>`. When it ends on `❓` or `❌`, the next step is what the owner has to answer or fix; say that instead.
- **Notification** (not when launched by Sectile). When the run reaches `❓`, `✅` or `❌`, send one desktop notification, under 200 characters, leading with what the owner has to do (for example `#47 waits for your answer: 2 product questions`). Send none for routine progress; the host drops it anyway when the owner is watching.
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
The task's clarification report (`docs/clarifications/<n>.md`) and specification (`specs/<KEY>-<slug>/` or `openspec/changes/<KEY>-<slug>/`) are its issue artefacts. They live in the specifications workspace, which may be another repository than the code: the project's Issue specifications folder on this workstation.
- **Where**: `SECTILE_SPEC_REPO` is its path, `SECTILE_SPEC_BRANCH` its branch and `SECTILE_SPEC_WORKTREE` whether it is a dedicated worktree, when Sectile launched the session. Otherwise call the `prepare_task_spec_worktree` MCP tool with the task key: it answers `path`, `branch`, `worktree`, `distinct` and `warning`, and reuses what a launch prepared. When neither is available, the task worktree is the specifications workspace.
- Read and write the issue artefacts under that path, never relative to the session directory, and repeat any warning in your report. Do not create, switch or check out a branch there: when it is not a dedicated worktree, check that the path is on its branch, and stop and say so if it is not.
- **Distinct workspace** (`distinct` true, or `SECTILE_SPEC_REPO` is not the task worktree): the issue artefacts are committed there, never in the code branch. Stage only the task's files (`git -C "<path>" add <files>`, never `git add -A`), commit with the stage's message, and push with `git -C "<path>" push -u origin <branch>` the first time and a plain push afterwards. Never force, never push the default branch; a refused push is reported and does not block the stage. Opening a pull request in that repository is the human's gesture: say the branch is pushed. The dropped artefacts check (`git check-ignore -q`) runs in that path.
- With a distinct workspace the code branch carries no artefact: open no pull request at the clarified or specified stage, even when the project creates it there, say so in the report, and create the draft after implementation. The code pull request description names the specifications repository and branch.
- **Plain folder** (empty branch): write in place and run no `git` command there; nothing is committed or pushed. Say so in the report.

## Goal
Autonomously take a ticket from its current stage through clarification, specification,
implementation, and testing, all the way to opening a clean Pull Request, updating each stage via Sectile.

## Read first
- The ticket: key, title, description, parent macro, and tracker comments.
- The project's code and existing patterns.
- The project SDD framework (OpenSpec or Spec Kit).

## Steps
1. Inspect the current ticket state AND existing artifacts. Reuse assigned branches, specifications, checklist progress and PRs. Verify completed work before skipping it.
2. Reuse or create a dedicated worktree and work branch. Continue through the stages below from the first incomplete stage to a verified PR.
Stop before merge. Stage-local boundaries apply while that stage is active; after its requirements are met, continue to the next stage without asking for routine confirmation.

### Clarify Issue
- The ticket: title, description, comments via get_task, parent epic if present.
- The existing clarification report if one exists: docs/clarifications/<n>.md on the assigned work branch,
  or, when the report is ignored by Git (step 6), in the worktree and in the rounds already
  published on the ticket.
- The code the change would touch. Name the files you actually read.
- Neighbouring features that already solve a similar problem in this codebase.

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


- Do not transition new → clarified while any product question or decision remains open.
- Do not invent answers to essential product questions in unattended runs; record them and ask.
- Do not write production code or start the technical specification at this stage.
- Do not discard previous round sections when writing Round N; append each round chronologically.
- Do not switch branches or create a new branch: reuse the assigned work branch.

Report and persist before continuing:
- The report path: docs/clarifications/<n>.md, and whether it is committed or local to the worktree (ignored by Git).
- Current round number and whether the exit condition was met.
- Settled decisions and reversed assumptions.
- Numbered open questions (if any) and who is expected to answer them.
- Stage transition status (applied or blocked awaiting answers).
- Full content of this round (initial report or new section), publication result and any numbered parts.
- Optional stage push result, including refusals that did not block the stage.


### Specify Issue
- Project-configured SDD framework: speckit. Use it unless the invocation explicitly overrides it.
- The clarification outcome on the ticket: the decisions are already made, apply them. When the
  clarification report is ignored by Git, docs/clarifications/<n>.md may be missing from this
  worktree; the rounds published on the ticket carry the same content.
- Select the SDD framework in order: explicit {sdd_framework} or --framework=<name>,
  then the project-configured framework, then repository detection:
  - If `openspec/` exists -> use OpenSpec SDD.
  - If `.specify/` or `specs/` exists -> use Spec Kit SDD.
- Ensure the project SDD directory is initialized before writing specifications.

1. Reuse the assigned worktree and branch (including a shared batch branch). Only create <KEY>-<title-slug> when no work branch is assigned. Preserve existing work; never write on the default branch.
2. Select the SDD framework from {sdd_framework} argument, flag, or project detection:

   **If using OpenSpec SDD:**
   - Create change directory `openspec/changes/<KEY>-<title-slug>/`
   - Write `proposal.md` (problem, value, in/out scope)
   - Write `design.md` (technical decisions, rejected alternatives)
   - Write `tasks.md` (ordered implementation checklist)
   - Write `specs/<capability>/spec.md` (requirements with Given/When/Then)
   - Validate with `openspec validate <change-id> --strict`

   **If using Spec Kit SDD:**
   - Write `specs/<KEY>-<title-slug>/spec.md` (prioritised user stories, functional requirements, Given/When/Then)
   - Write `plan.md` (stack, architecture, data contracts, target files)
   - Write `tasks.md` (ordered implementation checklist with test plan)
   - Use `/speckit.specify`, `/speckit.plan`, `/speckit.tasks` if available.
3. Dropped artefacts: before committing the specification, run `git check-ignore -q` on one of its
   files (`specs/<KEY>-<title-slug>/spec.md` or `openspec/changes/<KEY>-<title-slug>/proposal.md`).
   When it succeeds, the project drops its specification artefacts on this workstation: write the
   files in the worktree, never commit them, never force them with `git add -f`, and put the
   requirements and the open points in the transition note, saying that the files stay local to the
   worktree. Open no pull request at this stage then, even when the project creates it after
   specification: say in the report that it is deferred to the implemented stage.

4. Optional stage publication: read `pushStageCommits` from `get_project_context`
   (or the supplied project configuration); missing or false means off. When true,
   after each stage commit push the actual assigned work branch with a plain push.
   Use `git push -u origin <branch>` on first publication and `git push origin <branch>`
   afterwards. Never force. Report a refused push and continue the stage; retain the
   commit for retry. Ignored artifacts are never committed or force-added, so writing
   them alone triggers no push. This setting does not replace required PR publication.


- Do not decide what the clarification left open. Mark it as open and say so.
- Do not describe implementation inside the behaviour file.
- Do not start implementing, even the easy part.

Report and persist before continuing:
- The files written, with their paths, and whether they are committed or local to the worktree (ignored by Git).
- The work branch.
- Requirements that are still open, and what they block.

### Implement Code
- The specification and its task checklist. It is the contract, follow its order.
  Its files may be ignored by Git (`git check-ignore -q` succeeds on them): the project drops its
  specification artefacts on this workstation. Read them from the worktree, never commit them and
  never force them with `git add -f`. When the project drops its artefacts (the launch prompt says so,
  or the paths are ignored) and the specification is missing from the worktree, stop and report that
  it is not available on this workstation: never rewrite it.
- The surrounding code: naming, error handling, comment density, test style. Match it.
- How this project builds and tests. Find the real commands, do not assume them.

1. Reuse the assigned worktree and branch, including a shared batch branch. Never implement on the default branch.
2. On a multi-repo project, `$SECTILE_REPOSITORIES` lists the task's folders. Work in the
   primary worktree; the other repositories are read-only context. To change one, call
   `prepare_repository_worktree` for it first and work in the worktree it returns: each
   changed repository then needs its own pull request, given to `transition_stage` in `prUrls`.
3. Work through the checklist in small steps, each one leaving the tree buildable. When the
   specification artefacts are ignored by Git, commit the code only and never force-add them.
4. Add the tests that cover the new behaviour and its edge cases, not just the
   happy path. A change with no test needs a stated reason.
5. Run build, static analysis and tests. Fix until green, and quote the real output.
6. Re-read your own diff before finishing, as a reviewer would.

- Repair routine technical issues and update design/tasks when the implementation
  needs to change while preserving acceptance criteria. Continue after documenting why.
- Establish whether a failing test predates the change. Fix failures in scope; report
  unrelated failures with baseline evidence. Never hide them or mark checks green.
- Stop only for an essential product decision, an unavailable dependency after
  bounded recovery attempts, or work that materially expands the requested scope.
- Preserve the work branch, completed checklist items and remaining next action so
  a retry can resume instead of starting over.

Report and persist before continuing:
- What changed, file by file, and why.
- The real output of build, linters and tests, remaining failures included.
- What you deliberately left out, and what it would take to finish it.

### Adjust Existing Pull Request
- The full diff of the branch against the default branch. All of it, not the summary.
- The specification, to check that what was asked is what was built. When its files are ignored by
  Git (dropped artefacts), they are not in the diff: read them from the worktree, or from the
  clarification and specification reports on the ticket.
- The current remote default branch: fetch the remote and identify its configured
  default branch before reviewing or publishing.

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

- Do not merge, do not approve, do not close the ticket. That is the user's call.
- Do not create a PR. Do not mark a PR ready on a red build. Report the failure instead.
- Do not complete adjustment on a branch known to be behind the remote default branch.

Report and persist before continuing:
- What the review found, and which findings you fixed.
- The merge request URL, or why there is none.
- The test plan a reviewer can replay, as a checklist.


## Do not
- Do not merge into the default branch (merging is reserved for the human user).
- Do not push or open a PR if the test suite is failing.
- Follow the managed or standalone transition contract for the invocation.

## Report
- The created Pull Request URL.
- The work branch and files modified.
- The test results demonstrating that build, lint, and tests pass.
- Summary of settled scope and key architectural decisions.

## Execution and ticket state
- **Managed Sectile run**: When the invocation supplies a result-file contract, follow it. Sectile validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.
- **Remote execution indicator (standalone only)**: Before doing work, call start_run with the full task primary key and skill name. If SECTILE_RUN_ID or a launch runId is supplied, reuse it. Keep the returned activity ID as runId. Nested skills reuse the outer run; only the owner finishes it. Call finish_run with taskKey, runId, status (completed, failed or canceled), and a note when the entire invocation ends, including errors or stopping for user input. Intermediate stage transitions do not finish an enclosing pickup run. Never start a run merely to read a task.
- **Waiting for the user (standalone only)**: Right before asking the user a question you cannot continue without, call report_waiting with taskKey, runId and waiting true, so the board and the owner's desktop show the run as waiting. Your next Sectile call ends the wait; call report_waiting with waiting false if you resume without one. A headless run is left unmarked, which the result says.
- **Standalone invocation**: Read live context with `get_task` and `get_project_context`. After verifying each completed step, invoke `transition_stage` with the task key, completed stage, structured report note and actual branch. Check the tool result for errors before continuing.
Record clarified, specified and implemented after each corresponding step. After PR verification, record reviewed with the PR URL. At implemented and reviewed, give the PR of every other repository the task changed in `prUrls`. For a batch, use the same actual branch and combined PR URL for every completed ticket; never mark unfinished work reviewed.
A task holds an ordered set of pull requests, one or more per repository it changed, `prUrl` being its primary repository's current one. A pull request on a branch the task already used in that repository is a legitimate follow-up and is appended, even when the recorded one is merged; a pull request on an unrelated branch is refused, and its links are corrected from the task detail view rather than by forging evidence. A pull request opened outside a transition is recorded with `record_pull_request`, once per repository.
A task whose work changed no repository (a configuration made through an API, a review, a follow-up) has no pull request to give: pass `noRepositoryChange: true` to `transition_stage` instead of `prUrl`, and say in the note what was done instead. The server refuses the statement when the task records a pull request on its branch or a repository prepared with `prepare_repository_worktree`; then give those pull requests.
Use `add_comment` for an authorized ticket discussion update. Managed runs must not also invoke transition/comment tools for reports owned by Sectile. If MCP is unavailable, preserve work and report the pending transition; do not silently write to a different server or database.
Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.

## Ticket
$ARGUMENTS
