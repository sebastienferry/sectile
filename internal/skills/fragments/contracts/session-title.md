## Session title
- As soon as the {{.Item}} is identified, and before doing the work, rename the current session to `<{{.Item}} ID> - <{{.Item}} title>`, for example `#47 - Remove the parallelism setting`. The ID and title stay the same for the whole run: only a leading status emoji is added or removed.
{{if eq .ID "pickup_issues"}}- For a batch, name the session after the first ticket followed by the remaining count, for example `#47 (+2) - Remove the parallelism setting`.
{{end}}- The status emoji mirrors the run state the skill reports to Sectile, at the same moment and at no other. While the skill works the title carries none: the host already shows whether the session is running.
  - `❓` with report_waiting true, right before a question the skill cannot continue without, and when the skill stops with open questions for the owner. Remove it with report_waiting false, when the work resumes.
  - `✅` with finish_run completed, once the skill reached its goal.
  - `❌` with finish_run failed, when the skill stops on a failure or a blocker it cannot resolve.
- When the skill runs nested in pickup-issue or pickup-issues, do not rename the session: the outer skill owns the title and its status.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, or a rename is refused or left unapproved, keep the current title and continue silently. It never blocks, delays or replaces the work of the skill. A host that cannot rename, such as a Sectile Desktop console, shows the same state from the Sectile calls themselves.

## Launched by Sectile
A run is launched by Sectile when the invocation supplies a launch runId, or when the environment carries `SECTILE_RUN_ID` (read it with a plain shell command such as `printenv SECTILE_RUN_ID`). Sectile Desktop then already shows the ticket, the pull request, the next step and its own notifications, so the items marked "not when launched by Sectile" below are skipped: they would only repeat what Desktop shows.

## Session links
Not when launched by Sectile.
- Right after renaming the session, write one line in the conversation that links the {{.Item}}: `{{if eq .Item "macro"}}Macro{{else}}Ticket{{end}}: [<{{.Item}} ID>](<external URL>)`, with the external URL Sectile returns for it. Skip the line when the {{.Item}} has no external URL.
- When the skill creates or finds a pull request or merge request, write one line: `PR: [<number>](<URL>)`. On a GitHub pull request, when the host can bind the session to a pull request (the Claude desktop app's PR binding tool, for example) and the session does not show it already, bind it too. Elsewhere, GitLab included, the line is the only link.
- When the skill runs nested in pickup-issue or pickup-issues, write neither line: the outer skill writes the ticket line, and the PR line once a stage returns with a new pull request.
- If the host cannot bind a pull request, or the binding is refused, keep the line and continue. Links never block, delay or replace the work of the skill.

## Session experience
These make the session read like a Sectile Desktop execution. Use whatever the host exposes for each (in the Claude desktop app: its sidebar group, chapter, pane and notification tools); when the host has no such capability, or a call is refused, skip that item silently. None of them ever blocks, delays or replaces the work of the skill.
- **Project group.** Right after the session links, file the current session under the sidebar group named after the Sectile project (`projectName` from get_project_context). Reuse an existing group with that exact name; create it only when none exists. Move only the current session.
{{if or (eq .ID "pickup") (eq .ID "pickup_issues")}}- **One chapter per stage.** Before invoking each stage skill, mark a chapter titled `<Stage> <ticket ID>`, for example `Specify #47`.
{{else}}- **Chapter.** When the skill starts in a session that already holds earlier work, mark a chapter titled after the skill and the {{.Item}} ID, for example `Clarify #47`. Do not mark one when the skill is the first thing the session does.
{{end}}{{if or (eq .ID "implement") (eq .ID "adjust") (eq .ID "pickup") (eq .ID "pickup_issues")}}- **Changes.** When the skill ends after changing code, show the session's diff pane, provided it covers the worktree the skill worked in; otherwise name the worktree path in the reply instead.
{{end}}{{if ne .ID "handoff"}}- **Next step** (not when launched by Sectile). When the skill ends with `✅`, finish the reply with the next step, ready to copy, with the full task ID and the command name the skills were invoked under (`/sectile:<skill>` when installed as a plugin): {{if eq .ID "clarify"}}`/specify-issue <task ID>`{{else if eq .ID "specify"}}`/implement-issue <task ID>`{{else if or (eq .ID "implement") (eq .ID "create_pr")}}`/adjust-issue <task ID>`{{else if eq .ID "pickup_issues"}}review and merge the pull request, then `/handoff-issue <task ID>` for each ticket{{else if or (eq .ID "adjust") (eq .ID "pickup")}}review and merge the pull request, then `/handoff-issue <task ID>`{{else if eq .ID "refine_macro"}}`/pickup-issue <task ID>` on the first card of the breakdown{{else if eq .ID "realign_macro"}}`/pickup-issue <task ID>` on the next card to deliver{{else if eq .ID "rewrite_story"}}`/clarify-issue <task ID>`{{end}}. When it ends on `❓` or `❌`, the next step is what the owner has to answer or fix; say that instead.
{{end}}- **Notification** (not when launched by Sectile). When the run reaches `❓`, `✅` or `❌`, send one desktop notification, under 200 characters, leading with what the owner has to do (for example `#47 waits for your answer: 2 product questions`). Send none for routine progress; the host drops it anyway when the owner is watching.
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