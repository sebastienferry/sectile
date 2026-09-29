## Session title
- As soon as the {{.Item}} is identified, and before doing the work, rename the current session to `<{{.Item}} ID> - <{{.Item}} title>`, for example `#47 - Remove the parallelism setting`. The ID and title stay the same for the whole run: only a leading status emoji is added or removed.
{{if eq .ID "pickup_issues"}}- For a batch, name the session after the first ticket followed by the remaining count, for example `#47 (+2) - Remove the parallelism setting`.
{{end}}- While the skill works, the title carries no status emoji: the host already shows whether the session is running. Add one at these moments, and at no others:
  - `❓` waiting for the user: right before asking a question the skill cannot continue without, or when the skill stops with open questions for the owner. Remove it when the work resumes.
  - `✅` done: as the very last action, once the skill reached its goal.
  - `❌` blocked: when the skill stops on a failure or a blocker it cannot resolve.
- When the skill runs nested in pickup-issue or pickup-issues, do not rename the session: the outer skill owns the title and its status.
- This applies to every agent, not only Claude Code: use whatever session renaming capability the running agent exposes, be it a session title tool, a rename command or the host session API. Discover it from the session context instead of assuming a name.
- If no renaming capability is available, or a rename is refused or left unapproved, keep the current title and continue silently. It never blocks, delays or replaces the work of the skill.

## Session links
- Right after renaming the session, write one line in the conversation that links the {{.Item}}: `{{if eq .Item "macro"}}Macro{{else}}Ticket{{end}}: [<{{.Item}} ID>](<external URL>)`, with the external URL Sectile returns for it. Skip the line when the {{.Item}} has no external URL.
- When the skill creates or finds a pull request or merge request, write one line: `PR: [<number>](<URL>)`. On a GitHub pull request, when the host can bind the session to a pull request (the Claude desktop app's PR binding tool, for example) and the session does not show it already, bind it too. Elsewhere, GitLab included, the line is the only link.
- When the skill runs nested in pickup-issue or pickup-issues, write neither line: the outer skill writes the ticket line, and the PR line once a stage returns with a new pull request.
- If the host cannot bind a pull request, or the binding is refused, keep the line and continue. Links never block, delay or replace the work of the skill.