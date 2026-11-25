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
