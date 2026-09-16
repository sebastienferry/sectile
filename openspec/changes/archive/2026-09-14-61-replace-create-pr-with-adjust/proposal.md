# Replace Create PR with Adjust

## Why

Ticket #61 requires PR creation to belong to specification or implementation, according to project policy. The current Create PR action conflates creation with the review gate, including a default policy that creates the PR after review. Users need an Adjust action that reviews and corrects the complete branch and updates an existing PR before human merge.

Source: https://github.com/sebastienferry/sectile/issues/61. The confirmed clarification in `docs/clarifications/61.md` selects “Full review, earlier-stage”; no product questions remain open.

## What Changes

- Replace the workflow action between implemented and reviewed with Adjust.
- Create or reuse a draft PR in the configured earlier stage and persist its URL before that stage completes.
- Require an existing PR for adjustment; review the complete branch, incorporate available feedback, fix findings, run checks, and update that PR to ready.
- Preserve legacy invocations, saved customization, and activity history while making the replacement behavior authoritative.
- Align server routing, native and managed execution, generated skills, UI, recovery actions, and single/batch pickup workflows.

## Scope

In scope: both `specified` and default `implemented` PR timing policies; missing-PR recovery through the responsible earlier stage; repeat adjustment; compatibility and verification tests; workflow documentation.

Out of scope: new workflow states, board redesign, automatic merge or approval, automatic ticket closure, new forge integrations, and implementation during this specification stage.

## Impact

The capability is `workflow-adjustment`. Storage states remain `new → clarified → specified → implemented → reviewed → finished`. The visible actions are Clarify, Specify, Code, Adjust, and Handoff. Existing tasks and PRs remain usable. The design file defines identifier compatibility and the executable verification plan is in `tasks.md`.
