# Tasks

- [ ] Add `GithubApiUrl`, `GithubToken`, `GithubTokenSet`, `GithubTokenFromEnv`, `GitlabUrl`, `GitlabProject`, `GitlabToken`, `GitlabTokenSet`, `GitlabTokenFromEnv` to `models.Settings`, with the tokens `json:",omitempty"` like `JiraAPIToken`.
- [ ] Add the matching optional override fields to `models.Project` and to its create/update request shapes.
- [ ] Add the additive SQLite columns and the `ALTER TABLE` migrations for the settings and projects tables in `internal/db/db.go`, and extend the settings SELECT/UPSERT statements.
- [ ] Rename `db.JiraTokenClearSentinel` to `db.TrackerTokenClearSentinel` (value unchanged) and apply the empty-means-unchanged / `__clear__`-means-delete rule to the GitHub and GitLab tokens.
- [ ] Strip the tokens from every settings and project response and populate the `...Set` / `...FromEnv` flags, including the existing Jira pair, which is declared but never filled.
- [ ] Add a per-project credential resolver in `internal/db` applying project override → settings → environment, and have `trackerapi.Client` take resolved credentials per request instead of the values frozen at `NewDB`.
- [ ] Make `HandleTrackerSetup` tracker-aware: accept the tracker being configured with its own fields, implement the GitHub and GitLab `GET /user` check, and persist only after a successful check. Leave the Jira path's current behaviour untouched.
- [ ] Make `TrackerSetup.tsx` render the fields of the selected tracker, show "provided by the environment" from the `...FromEnv` flags, and hide the out-of-database checkbox for GitHub and GitLab.
- [ ] Expose the per-project overrides in `ProjectModal.tsx` and carry the new fields through `AppContext.tsx`.
- [ ] Assert in `internal/db/agentconfig_test.go` that no GitHub or GitLab token reaches the agent configuration.
- [ ] Unit-test the resolution order (project → settings → environment), the write-only token rules including the clear sentinel, and the check-before-save refusal path.
- [ ] Update the README credential documentation: the interface is the primary way to configure GitHub and GitLab, the environment variables stay as a headless fallback.
- [ ] Run `make test` and record the output.
