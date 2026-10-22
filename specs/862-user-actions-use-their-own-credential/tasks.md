# Tasks #862 - Every action a user performs uses their own tracker credential

## 1. Revert the routing - `revert`

- [ ] T1.1 `git revert --no-commit` a37912f5, the skill-run test of 3ec7b91e, 32de3fd2, 554bb0c6, 062c9459, c52a8576; resolve conflicts.
- [ ] T1.2 Restore `MissingServerCredentialError` and its helpers, and `serverWriter` for unattended writes.
- [ ] T1.3 Keep the personal-check split and the OAuth port intact; build.

## 2. Reads with fallback - `fix(trackerapi)`

- [ ] T2.1 Jira reads fall back to the server token for a person without a credential; a locked credential refuses.
- [ ] T2.2 Tests for GitHub, GitLab and Jira reads: person's token, fallback, locked refusal.

## 3. Scopes - `fix(oauth)`

- [ ] T3.1 GitHub consent `repo read:project`; scope pin test; web scope list and its test.

## 4. Tests - `test`

- [ ] T4.1 Routing tests back to pre-#855 assertions; route and skill-run test files removed.
- [ ] T4.2 A skill-run comment and a web stage move carry the person's token.

## 5. Docs - `docs`

- [ ] T5.1 ADR 0059; status lines on ADR 0058, 0029, 0044.
- [ ] T5.2 README, user guide, API spec, `.env.sample`, CHANGELOG, translations.

## 6. Gates

- [ ] T6.1 `gofmt -l`, `go vet ./...`, `go test ./...`.
- [ ] T6.2 Web `tsc`, `oxlint`, `npm test`.
- [ ] T6.3 End-to-end replay against the fake GitHub.
