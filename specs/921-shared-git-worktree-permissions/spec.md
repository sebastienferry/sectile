# Specification #921 - Shared Git worktree permissions

Task: gh-f8b8807d-84dc-42ba-a43a-0b832629eae0-921

Source: https://github.com/sebastienferry/sectile/issues/921

Baseline: accepted clarification round 2 in `docs/clarifications/921.md`.

## Scope

Cover built-in Codex app-server conversations and autonomous CLI launches separately. Require live macOS qualification. Exclude custom command templates, other providers, general authorization settings (#874), and changes to the permissions of this specification session. Other platforms receive no unverified guarantees.

The original operation created commit `9b84aed0`, exited zero and left a clean tree while reporting a denied shared `packed-refs.lock` creation. The commit did not fail. The requester and responsible enforcement layer are not yet established.

## Prioritized user stories

### US1 (P1) - Perform authorized Git work

As a user working in a linked task worktree, I can stage and commit through either supported Codex launch path with narrowly scoped access to the necessary shared Git metadata.

- Given a disposable linked worktree under the actual supported launch policy, when the reproducing Git workflow executes, then evidence identifies the process requesting the shared lock, its triggering conditions, effective permissions and provider/Git versions.
- Given a supported local correction exists, when staging, committing and the identified lock-requesting operation execute under each launch path, then the intended Git state is reached without the observed lock denial.
- Given a command exits zero and creates its commit but an auxiliary operation is denied, when its outcome is reported, then both the successful commit and denied auxiliary operation are stated separately.

### US2 (P1) - Keep permission boundaries

As a user, I retain protection for hooks, configuration and unrelated paths while allowing authorized task Git operations.

- Given overlapping writable and protected locations, when effective access is qualified, then evidence establishes precedence rather than inferring access from an emitted policy alone.
- Given protected hooks/configuration and unrelated locations, when access is checked in disposable fixtures, then those locations remain denied.
- Given a guard denial, when execution reports it, then that intention stops with the refusal preserved and owner direction requested before another attempt; no silent retry, alternate route or unrestricted bypass occurs.

### US3 (P2) - Understand an upstream limitation

As a user, I receive an actionable supported remedy if Sectile cannot safely correct an upstream restriction.

- Given an evidence-backed upstream restriction with no safe supported local correction, when the ticket is completed, then the responsible layer, affected launch paths, tested versions, remaining limitation and actionable supported recovery/configuration path are explicit.
- Given that outcome, then reports do not claim denial-free Git operations or conceal the restriction. Runtime diagnostics use the existing French surface language; repository documentation uses English.

## Functional requirements

- FR1: Qualify both built-in paths independently on macOS using a real linked worktree and common Git directory.
- FR2: Record sanitized commands, relevant environment/configuration, requested and effective policy, process evidence, stdout/stderr, exit status and resulting Git state.
- FR3: Attribute the defect to Sectile configuration, upstream enforcement or their interaction using evidence.
- FR4: Select the narrowest supported correction when one exists; preserve selective access and protected locations.
- FR5: Exercise the actual reproducing operation in regression coverage; policy serialization tests alone are insufficient.
- FR6: Meet either the corrected local behavior in US1 or the evidenced upstream-only outcome in US3, separately stating the result for each launch path.
- FR7: Update relevant launch/permission and testing documentation. Add an English Unreleased changelog entry if behavior visible to users changes.

## Completion and open points

No open product questions. The lock requester, trigger, effective precedence, correction feasibility and supported remedy are implementation investigations. They block selecting a concrete correction, not specification completion. A non-reproducing run alone does not establish a fix or upstream attribution. Completion requires an evidence-backed disposition for both launch paths and verification of protected boundaries.
