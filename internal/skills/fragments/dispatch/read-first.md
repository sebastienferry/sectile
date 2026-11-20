- The hand-off: Jira keys given as arguments, each optionally followed by a skill (`PE-123`, `PE-123 specify-issue`, `PE-123 auto`), or what the Sectile board sent (`ticket`, `url`, `skill`, `command`, and whether the launch is autonomous).
- For a key that comes without a command, the ticket's `#<stage>` label.
- The repository map, `~/.config/sectile-lite/repositories.json`, when it exists. It belongs to this workstation and lists which local folder works which tickets:

  ```json
  {
    "repositories": [
      { "project": "PE", "component": "Wiki", "path": "/Users/me/Sources/pe/pe-wiki", "remote": "gitlab.example.com/pe/pe-wiki" },
      { "project": "PE", "path": "/Users/me/Sources/pe/iac-core" }
    ]
  }
  ```

  `project` is a Jira project key. `component` and `label` narrow a rule to the tickets that carry them. `path` is the folder of a local clone. `remote` is the repository's address without its scheme, used to recognise its pull requests. Compare two addresses as host and path only: drop the scheme, any user, a trailing `.git` and a trailing slash, and read `git@host:group/repo` as `host/group/repo`.
- For each ticket, its project, components, labels and pull request links (the remote links on the issue).
- The sessions this session already started, from the host's list of linked sessions.