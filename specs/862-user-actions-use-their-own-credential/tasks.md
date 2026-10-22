# Tasks #862 - Every action a user performs uses their own tracker credential

## 1. Revert the routing - `revert`

- [x] T1.1 `git revert --no-commit` a37912f5, the skill-run test of 3ec7b91e, 32de3fd2, 554bb0c6, 062c9459, c52a8576; resolve conflicts.
- [x] T1.2 Restore `MissingServerCredentialError` and its helpers, and `serverWriter` for unattended writes.
- [x] T1.3 Keep the personal-check split and the OAuth port intact; build.

## 2. Reads with fallback - `fix(trackerapi)`

- [x] T2.1 Jira reads fall back to the server token for a person without a credential; a locked credential refuses.
- [x] T2.2 Tests for GitHub, GitLab and Jira reads: person's token, fallback, locked refusal.

## 3. Scopes - `fix(oauth)`

- [x] T3.1 GitHub consent `repo read:project`; scope pin test; web scope list and its test.

## 4. Tests - `test`

- [x] T4.1 Routing tests back to pre-#855 assertions; route and skill-run test files removed.
- [x] T4.2 A skill-run comment and a web stage move carry the person's token.

## 5. Docs - `docs`

- [x] T5.1 ADR 0059; status lines on ADR 0058, 0029, 0044.
- [x] T5.2 README, user guide, API spec, `.env.sample`, CHANGELOG, translations.

## Notes (implementation)

- The revert applied without conflict. `MissingServerCredentialError` and `serverWriter` were restored for unattended writes only.
- T4.2: the skill-run comment test is `TestACommentFromASkillRunIsThePersons`; the web stage move needs no new test since the route table and `stageMoveOnly` are gone and the pre-#855 isolation tests (stage report as the person) are back.

## 6. Gates

- [x] T6.1 `gofmt -l`, `go vet ./...`, `go test ./...`.
- [x] T6.2 Web `tsc`, `oxlint`, `npm test`.
- [x] T6.3 End-to-end replay against the fake GitHub.

Gate results (2026-10-22): gofmt clean, go vet clean, go test ./... green except
TestTryPullLocalAgentTasks, a TempDir cleanup race that passes 5/5 alone and
already flaked on #855. Web tsc clean, oxlint 0 errors, npm test 764/764.
End-to-end against the fake GitHub: create without a personal token refused
403 tracker_credential_missing; create, comment, reads, stage move and macro
rename all as ada; sync as service-bot.
