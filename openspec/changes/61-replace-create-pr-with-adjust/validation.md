# Implementation validation — issue #61

## Result

Implemented on `feat/61`, reusing draft PR #68. The application offers Adjust between
implemented and reviewed; PR creation belongs to the configured specification or
implementation stage. Human merge and handoff remain separate.

## Changes

- Models, skill templates, board routing, worker dispatch, native dispatch and runner
  prompts normalize adjustment aliases without rewriting stored activity history.
- Project skill editing resolves canonical and legacy content, exposes preserved
  conflicts and blocks unreconciled customization. Reset selects the current default
  while retaining historical entries. Local override and scaffold logic preserve
  divergent legacy files and report them.
- Database and runner PR evidence verify branch, open state, readiness and pushed
  commit. Managed adjustment pins the original PR before execution; a replacement,
  draft, dirty checkout, failed checks or blocked receipt cannot complete review.
- Earlier PR-owning completion records the URL, supports discovery and reuse, and
  owner recovery preserves an already implemented task. Standalone transitions and
  stage postbacks share the PR gate; managed-run exclusion remains enforced.
- Cards, detail actions, copy commands, workflow helpers and editors use Adjust.
  Reviewed tasks offer Handoff; explicit repeat adjustment is available. Missing PR
  actions select the configured earlier owner.
- Generated skills for specification, implementation, adjustment and composite pickup
  were regenerated from the templates for all supported layouts. Existing unrelated
  skill edits and divergent legacy command copies were preserved.
- README, workflow guides, server-agent contract and ADR 0004 document the behavior.

## Commands and actual results

- `make test`: passed. All `go test ./internal/...` packages passed; frontend reported
  `tests 22`, `pass 22`, `fail 0`; TypeScript completed without errors; oxlint exited
  successfully with 53 warnings.
- `go test ./cmd/server`: `ok tasks/cmd/server 2.335s`.
- `go vet ./...`: exit 0, no output.
- `make binary-build`: frontend build passed and embedded Go build completed with
  `Done: bin/sectile`. Vite reports the existing bundle-size warning.
- `openspec validate 61-replace-create-pr-with-adjust --strict`: passed:
  `Change '61-replace-create-pr-with-adjust' is valid`.
- `git diff --check`: exit 0, no output.

Lint baseline was reproduced from `git archive HEAD web` before the implementation
commit and checked using the same installed oxlint. All 53 warning messages match
by source file and rule after ignoring shifted line numbers: none added or removed.

## Regression coverage and verification boundary

Tests cover legacy/canonical dispatch, human stop routing, both PR timing policies,
recovery without downgrade, missing/inconsistent/open/draft/replacement PR evidence,
forge failure, canonical override precedence, reconciliation/reset with retained
legacy content, scaffold divergence and idempotence, native command contracts, and
single/batch templates under OpenSpec and Spec Kit.

Managed adjustment is exercised through the real worker with an injected terminal
and forge evidence: no-feedback success stops at reviewed; failed checks, draft PR,
replacement PR and feedback-retrieval failure retain implemented. Standalone tests
verify URL persistence and readiness rejection, and MCP tests reject unverified URLs.

The checklist's live manual workflow exercise was replaced by these deterministic
integration fixtures to avoid launching competing agents or creating throwaway remote
PRs in this implementation run. Forge creation/push/readiness mutations are performed
by the instructed agent, not by a new application mutation client; their required
failure behavior is covered by generated contracts and failed completion receipts.
No new binary was installed into the running Sectile service, no live agent workflow
was launched, and no PR was merged. The existing real PR was checked as open/draft on
`feat/61`; it remains draft for the subsequent adjustment/review stage.


## Web skills UX follow-up

The five workflow skill names are Clarify, Specify, Implement, Adjust and Handoff.
The web editor displays their transitions with `#` prefixes and separates additional
skills from those five steps. Framework-specific document titles remain inside the
skill documents, rather than replacing the user-facing skill name. The user confirmed
that the stored state remains `implemented`.

The subsequent user clarification restores Create PR as a standalone utility, outside the five workflow stages. Its command no longer redirects to Adjust. Managed creation verifies checks and the matching branch PR while preserving status and labels, with no automatic chaining. Desktop deployment installs separate Create PR and Adjust documents.

The Repository tab now uses the shared project-form field styling. The Codex preset is `codex --approve-for-me '{prompt}'`. Desktop skill regeneration is documented through Project configuration → Deployment → Deploy server skills.

Follow-up verification on current main (2026-09-13): `go test ./...`, `go vet ./...`, server binary build, frontend production build, all 22 frontend tests, and frontend lint passed. Lint reports 53 existing warnings; Vite reports the existing bundle-size warning. The worker regression confirms standalone creation completes, persists the verified PR URL, preserves `implemented`, and does not enqueue the next workflow stage.
