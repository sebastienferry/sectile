---
name: setup
description: "Get this workstation ready for Sectile Lite: ask for your team once and remember it for every Lite skill, check the Atlassian connector, the host, git and the GitHub or GitLab CLIs, and say what fixes each failure."
---
# Sectile Lite Setup

## Goal
Get this workstation ready for Sectile Lite and say what works: ask which team the user works in, once, and remember it where every Lite skill reads it; set up the ticket workspace where the skills write each ticket's specification and notes, with its Spec-Driven Design framework; check the tools the skills rely on, the Atlassian connector, the host features, `git` and the forge CLIs, and the local files the plugin keeps; and end with one table that marks each check as passed, failed or not needed, with the command that fixes each failure. Run again at any time, it is the diagnosis when a Lite skill fails.

## Read first
- The team configuration, `~/.config/sectile-lite/config.json`, when it exists. It belongs to this workstation, is written by this skill only, and gives every Lite skill the user's site and team:

  ```json
  {
    "site": "example.atlassian.net",
    "team": {
      "name": "Payments",
      "projects": ["ABC"],
      "jql": "project in (ABC) AND statusCategory != Done ORDER BY Rank ASC"
    },
    "workspace": {
      "path": "/Users/me/sectile-workspace",
      "sync": true,
      "framework": "speckit"
    },
    "checkedAt": "2026-11-25T09:00:00+01:00"
  }
  ```

  `site` is the Jira site. `team.name` is how the user names their team. `team.projects` are the Jira project keys the team works in. `team.jql` is the team's ticket query: the default query of the board and the default source and triage pool of the daily brief. `workspace` is the ticket workspace, where the ticket skills write each ticket's clarification, specification and notes for the owner to review in Claude: `path` its folder, `sync` whether it is a Git clone shared through a remote (one branch and one merge request per ticket) or local only, `framework` its Spec-Driven Design framework (`speckit` or `openspec`). `checkedAt` is when the last full check passed.
- The argument, when there is one: `check` to diagnose without asking anything nor writing any file, `team` to change the team only, `workspace` to change the ticket workspace only.
- The other files of the plugin, when they exist: the daily brief's `~/.config/sectile-lite/daily.json` and the dispatcher's repository map, `~/.config/sectile-lite/repositories.json`.
- The connectors and tools this session exposes, under the names the host gives them.
- The current folder, and whether it is a git clone with an `origin` remote.

## Steps
1. **Check the Atlassian connector.** Find the Atlassian or Jira connector among the session's tools; without one, mark it failed with the fix "add the Atlassian connector in claude.ai (Settings, Connectors), then start a new session" and skip every Jira check. With one: list the Atlassian sites it reaches, read the user's own identity through it, and run one search, `assignee = currentUser() ORDER BY updated DESC`, limited to one result. Mark each call passed or failed with its error. Note whether the connector offers `editJiraIssue` and `addCommentToJiraIssue`, which the ticket skills write with.
2. **Ask for the team, once.** Skip with `check`, or when `config.json` has a `team` and the argument is not `team`. Otherwise ask in one message, offering what is already known as the default:
   1. the Jira site, from the sites the connector reaches (the only one when there is one, or the `site` of `daily.json`);
   2. the team's name;
   3. the Jira project keys the team works in, offering the projects of the tickets the search of step 1 returned;
   4. the team's ticket query: offer `project in (<keys>) AND statusCategory != Done ORDER BY Rank ASC`; the user may give a Jira filter (`filter = <id>`) or the query of their team's board instead.

   Run the query once with the connector, limited to five results, and show what it returns: a query that fails is shown with its error and asked again. Then show `config.json` as it would be written and write it only on a yes, keeping any key already there. Without a yes, write nothing and say the other skills will keep asking.
3. **Set up the ticket workspace, once.** Skip with `check`, or when `config.json` has a `workspace` and the argument is not `workspace`; then only check it (step 6). Otherwise explain in two lines what it is for (each ticket's clarification, specification and notes, written there rather than in the code repository and opened in Claude for review), then ask in one message:
   1. the folder: an existing one the user names (offer the folders already used for specifications, such as a `workspace` clone next to their repositories), else `~/sectile-workspace`;
   2. whether to sync it with the team: yes means a Git remote (the URL of an existing repository, cloned when the folder does not exist yet, or added as `origin` to the folder), one branch per ticket and a merge request the owner opens; no means a local Git repository, committed but never pushed.

   It is a Git repository by default: a new folder gets `git init` with an initial commit of a short `README.md`, after a yes. Check the result: the folder exists, is a Git repository, and with sync its `origin` answers `git ls-remote`.
4. **Set up the Spec-Driven Design framework.** Detect it in the workspace: `.specify/` is Spec Kit (`speckit`), `openspec/` is OpenSpec (`openspec`). With none, propose one, Spec Kit by default, and install it only after a yes, showing each command before running it:
   - Spec Kit: `uv tool install specify-cli --from git+https://github.com/github/spec-kit.git` when `specify` is missing, then `specify init --here --ai claude` in the workspace;
   - OpenSpec: `npm install -g @fission-ai/openspec` when `openspec` is missing, then `openspec init` in the workspace.

   When a prerequisite is missing (`uv`, `npm`), give its install command and stop there for this check. Commit what the initialisation wrote in the workspace (`chore: initialise <framework>`), and push it with sync after a yes. Then show `config.json` with the `workspace` (path, sync, framework) as it would be written and write it only on a yes, keeping the other keys.
5. **Check the host.** Say whether this host can start a session of its own (in Claude Code, a session-starting tool such as `start_session`), which `/sectile-lite:dispatch` and `/sectile-lite:daily` need to open one session per ticket, and whether it can publish a page (the Artifact tool), which `/sectile-lite:board` needs. A missing feature is "not available in this host", with where it is available (the Claude desktop app's Code tab), never a failure to fix.
6. **Check the command-line tools.** Run each once and read its version or status, never a token:
   - `git --version`, and `git config user.name` and `git config user.email`, which every commit needs. Git 2.5 or later is needed for worktrees.
   - The forges the user's repositories live on: the hosts of the remotes in `repositories.json` and of the current folder's `origin`. For a `github.com` remote, `gh --version` and `gh auth status`. For a GitLab remote, `glab --version` and `glab auth status --hostname <host>`. A forge no remote uses is "not needed".
   - For each forge, one read-only call that needs the login, on one repository the user has: `gh repo view <owner>/<repo> --json name` or `glab repo view <group>/<repo>`.
   - The ticket workspace: its folder exists and is a Git repository, its framework folder is there (`.specify/` or `openspec/`) and its CLI answers (`specify --help` or `openspec --version`), and with sync, `git -C <path> ls-remote origin` answers and the remote's host is a forge checked above.
7. **Check the local files.** For `daily.json`, say whether it exists and whether its sources still run (one search each, limited to one result). For `repositories.json`, check that each `path` exists, is a git clone, and that its `origin` matches the entry's `remote` when one is given (compare as host and path only, as the dispatcher does). An entry that fails is reported with the line to fix; nothing is rewritten.
8. **Remember the check.** Outside `check`, when every check passed and `config.json` exists, set its `checkedAt` to now. That is the only key the skill changes without asking.
9. **Report.** One table, one line per check: what was checked, ✓ passed, ✗ failed or – not needed, and for each ✗ the exact command or action that fixes it (`gh auth login`, `glab auth login --hostname <host>`, `git config --global user.email <address>`, adding the connector). Then which Lite skills are ready to run and which wait on a fix: the ticket skills need the connector, the forge of the ticket's repository and the ticket workspace, `board` needs the connector and pages, `dispatch` and `daily` need the connector and session starts.

## Do not
- Do not install, upgrade or log in to anything, except the ticket workspace and its Spec-Driven Design framework, each only after a yes and with its command shown first. Never log in for the user: give the command, the user runs it.
- Do not print, copy or store a token, a password or the content of a credential file. Read a login's status, never its secret.
- Do not write `config.json` without a yes, except `checkedAt` as in the steps, and do not write `daily.json`, `repositories.json` or anything outside `~/.config/sectile-lite/` and the ticket workspace the user approved.
- Do not write to Jira, and write to a forge only to push the ticket workspace's initialisation after a yes: every other call of this skill only reads.
- Do not assume a team, site, project or query: they come from the user's answers or from their own files.

## Report
- The check table, with the fix of each failure.
- The team configuration: created, changed, used as it was, or not written.
- The Lite skills ready to run, and those that wait on a fix.
- End every reply addressed to a person with the `### 📋 Status Update` block (Done, Remaining (Agent), Pending (User)), written in the language of the conversation.
