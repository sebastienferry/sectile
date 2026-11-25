- The team configuration, `~/.config/sectile-lite/config.json`, when it exists. It belongs to this workstation, is written by this skill only, and gives every Lite skill the user's site and team:

  ```json
  {
    "site": "example.atlassian.net",
    "team": {
      "name": "Payments",
      "projects": ["ABC"],
      "jql": "project in (ABC) AND statusCategory != Done ORDER BY Rank ASC"
    },
    "checkedAt": "2026-11-25T09:00:00+01:00"
  }
  ```

  `site` is the Jira site. `team.name` is how the user names their team. `team.projects` are the Jira project keys the team works in. `team.jql` is the team's ticket query: the default query of the board and the default source and triage pool of the daily brief. `checkedAt` is when the last full check passed.
- The argument, when there is one: `check` to diagnose without asking anything nor writing any file, `team` to change the team only.
- The other files of the plugin, when they exist: the daily brief's `~/.config/sectile-lite/daily.json` and the dispatcher's repository map, `~/.config/sectile-lite/repositories.json`.
- The connectors and tools this session exposes, under the names the host gives them.
- The current folder, and whether it is a git clone with an `origin` remote.
