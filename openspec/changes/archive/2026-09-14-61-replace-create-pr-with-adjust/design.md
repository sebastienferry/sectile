# Design: adjustment of an existing PR

## Context and constraints

The assigned branch is `feat/61`. OpenSpec is initialized. The live project policy is `specified`, while the application default remains `implemented`. Preserve both. This is a Go backend with a React/TypeScript frontend and generated skills for multiple native clients. No new service or dependency is required.

Current behavior is spread across `internal/db/skilltemplates.go`, `projectskills.go`, `board.go`, `db.go`, `skillresult.go`, `internal/models/models.go`, `internal/runner/runner.go`, and `web/src/lib/workflow.ts`. Review currently uses canonical ID `create_pr` and alias `review`; result validation checks build/lint/test evidence, clean checkout, branch identity, and forge PR lookup. `PromptCreatePR` and `prompt_create_pr` retain saved prompts. Generated files are products of project overrides and embedded templates; handwritten edits are already detected as divergence.

## Canonical identity and compatibility

Use canonical skill ID `adjust`, directory/command `adjust-issue`, and visible name Adjust. Its stages remain implemented to reviewed. Centralize normalization so `create_pr` and `review` resolve to `adjust` before dispatch, validation, template lookup, managed-run exclusion checks, and success handling. Map old `/create-pr` entry points to forwarding compatibility skills that invoke the same behavior. New menus and generated workflow instructions advertise only Adjust. Do not reinterpret workflow state `reviewed` or tracker aliases as skill identifiers.

Keep stored jobs and activity records unchanged; normalize only execution and presentation lookup. Historical names and reports remain readable. Retain `PromptCreatePR`/`prompt_create_pr` and existing wire fields as compatibility storage in this change, relabeling their editor as adjustment instructions. This avoids an unrelated settings migration.

Resolve saved project overrides in deterministic order: nonempty `adjust`, then `create_pr`, then `review`, then the built-in template. Do not delete losing entries. Show the selected origin and any conflicting legacy entries in the existing skill editor. Treat any inherited legacy override/custom prompt as requiring compatibility review, without attempting unreliable semantic classification: expose its complete content and an actionable warning before execution. The user can save it under Adjust or explicitly reset it using existing editor semantics. Until resolved, block automatic execution of that customization and preserve the task stage. Empty/default prompts use the new built-in behavior immediately. The new adjustment contract must also accompany custom content after reconciliation; custom content cannot restore PR creation or weaken completion evidence.

Generate canonical files for all `SkillAgentDirs` and the Claude command layout from templates. Install forwarding legacy commands only where they do not overwrite divergent files; report divergence using the existing mechanism. Update installation manifests and project workflow context to list Adjust. Do not hand-edit generated copies as the source of truth.

## PR lifecycle and recovery

| Project policy | Specify | Implement | Adjust |
| --- | --- | --- | --- |
| specified | Validate artifacts; push and create/reuse draft; persist URL | Implement/check/push; update same PR | Review/fix/check/update existing PR; mark ready |
| implemented (default) | Validate artifacts; no PR creation | Implement/check/push; create/reuse draft; persist URL | Review/fix/check/update existing PR; mark ready |

Use existing forge discovery by repository and actual task branch. A matching open PR discovered without a saved URL is linked and reused. A saved URL must agree with the forge and branch; mismatched, closed, or merged PRs cannot satisfy adjustment. A forge lookup failure is not proof that a PR is absent: report retryable external failure and preserve the stage. Never create a duplicate on an inconclusive lookup. Preserve a reused ready PR's existing readiness during earlier stages; newly created PRs are drafts.

The missing-PR shortcut launches the project's configured creation-owning skill (`specify` or `implement`) with explicit recovery context: preserve accepted artifacts/code, perform remaining stage checks, create/reuse/link the PR, and preserve an already achieved later stage. Do not add a creation workflow stage or advance to reviewed. Recovery must not downgrade an implemented task to specified, rerun unrelated clarification, or bypass the selected owner's checks. Update standalone transition and managed completion handling to recognize recovery context and cap completion at the already attained stage when later than the owner's normal destination.

Check the existing PR prerequisite before adjustment changes the branch. Adjustment fetches the remote default branch, reconciles it according to existing branch policy, reviews the entire final diff against the specification, reads available PR review feedback, fixes actionable findings, and runs project checks on the final code. A feedback retrieval failure is reported, not interpreted as no feedback. Record dispositions for feedback that requires explanation rather than a code change. Commit/push fixes, update the same PR description and validation evidence, and mark it ready only after success. No new PR, merge, approval, closure, or worktree cleanup occurs here.

## Execution and result contracts

Extend `StageSkillByID`, board routing, job lookup, stage postback mapping, autonomous chaining, and runner skill command selection together. `reviewed` retains Handoff as the next workflow action; an explicit repeat Adjust action is allowed for unmerged PRs, keeps reviewed on success, and reports failure without falsely claiming new successful review. Autonomous processing still stops at reviewed.

Keep the existing managed result JSON shape (`runId`, `outcome`, `summary`, `branch`, `prUrl`, `artifacts`, `checks`). Make policy/task/recovery context available to validation and completion rather than inferring it from a skill name. Require matching forge-confirmed PR evidence for completion of the creation-owning stage; persist `prUrl` for specify/implement as well as adjust/pickup. Retain specification artifact checks and implementation build/lint/test checks. For adjustment, retain clean checkout and branch verification, build/lint/test evidence, and verify that the same open PR is ready and targets the task branch. Extend the injectable forge lookup contract to carry open/draft state and branch identity so tests do not require network access. A URL alone cannot prove readiness.

For managed adjustment, capture the resolved existing PR identity before launch and compare it at completion, preventing a newly created replacement from satisfying the gate. For standalone execution, generated instructions enforce the same prerequisite and report evidence through existing stage transitions; keep the current distinction between agent-reported checks and server-verifiable PR identity explicit. Reject reviewed transitions without the required existing, ready PR evidence. Managed runs still own their result-file validation and cannot be advanced by standalone postbacks.

Single pickup runs perform clarify/specify/implement/adjust in order and stop at reviewed. Batch pickup keeps its one branch/one PR model, attaches that URL to each applicable ticket, applies each ticket's creation timing, and performs full combined review. Unfinished tickets must not be marked reviewed because a shared PR exists.

## UI and documentation

Update task card/detail actions, copy-skill menus, workflow helpers, project/profile skill editors, activity rendering, and translation keys referencing creation as the review step. The missing-PR action explains which earlier stage will complete PR setup. Show compatibility warnings next to the affected customization. Keep PR open-link controls and historical activity content intact. All newly authored text is English, per repository instructions.

Update README workflow/configuration guidance, CHANGELOG, `docs/contracts/server-agent-v1.md`, relevant workflow guides, and generated project context. Record the compatibility/ownership decision in an ADR during implementation; existing ADR numbering must be checked then. This design serves as the reviewable decision proposal now.

## Rejected alternatives

- A button-only rename leaves creation in runner fallbacks and default-policy templates.
- Creating a recovery PR inside Adjust contradicts the confirmed earlier-stage ownership.
- A feedback-only action loses the full branch quality gate and requires human feedback to make progress.
- Removing legacy IDs or rewriting history loses queued executions and saved customization.
- Silently executing or discarding old creation prompts either violates the new contract or loses user work.
- Adding an adjusted storage state or migrating settings fields expands scope without behavioral benefit.

## Open questions and rollout

No product requirement remains open. Override precedence, explicit reconciliation, alias forwarding, and recovery stage preservation are implementation decisions within the confirmed compatibility scope. Ship backend, UI, templates, and tests together. Existing missing-PR tasks recover through the owner stage; existing customized projects see a reconciliation notice before automatic adjustment. Preserve unrelated worktree changes throughout implementation.
