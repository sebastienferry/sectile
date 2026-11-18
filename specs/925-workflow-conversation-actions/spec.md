# #925 — Conversation workflow actions

Ticket: https://github.com/sebastienferry/sectile/issues/925
Task: gh-f8b8807d-84dc-42ba-a43a-0b832629eae0-925
Branch: `feat/925`
Baseline: `docs/clarifications/925.md`, accepted Round 2 (2026-11-18).

## Scope and definitions

This change concerns the Tauri conversation UI. A **closed conversation** means the selected conversation is read-only history and no execution belonging to its task is active. A completed skill or an idle writable conversation alone does not qualify. Archived history retains its existing behavior. The header next-step action remains available under its existing rules.

Move the existing Stop discussion action beside Workflow for active conversations, including idle active sessions. Retain Archive in the header for inactive conversations. No new close operation, backend behavior, migration, workflow rule, or draft transfer is introduced. The header sizing work in #922 is a visual reference, not a dependency or a separate deliverable here.

## Prioritized user stories and acceptance scenarios

### US1 (P1) — Continue a ticket from closed history

As a ticket owner viewing closed history, I can advance the workflow beside its Workflow chip without searching the header.

1. Given selected read-only task history, an eligible next step, and no active task execution, when the status strip is displayed, then a compact next-step chevron appears immediately beside Workflow and the header next-step remains present.
2. Given those two controls, when either is activated, then it invokes the same next step once using the existing launch lifecycle and presents the corresponding execution or launch error as before.
3. Given an eligible closed history and an unavailable agent, when displayed, then both next-step controls are disabled and the existing agent-unavailable status remains visible.
4. Given an active execution on the selected task (including another selected history's task run), when history is displayed, then no next-step action appears in either location.
5. Given writable history with no active execution, when displayed, then the contextual next-step is absent; header behavior is unchanged.
6. Given a finished or unknown-stage ticket, missing project configuration, missing expected skill, missing task association, or missing launch capability, when displayed, then the contextual next-step follows existing header visibility rules and does not offer an unusable action. Preserve existing workflow explanations rather than add new disabled states.
7. Given archived read-only history satisfying the same eligibility rules, when displayed, then it offers the contextual next-step and retains the existing Start over behavior.
8. Given a workflow offering PR creation/recovery, when either next-step is activated, then the existing recovery selection is used, rather than advancing to a later review step.

### US2 (P1) — Stop an active discussion beside Workflow

As a user in an active conversation, I can stop it from the status strip with the existing stop semantics.

1. Given an active task conversation, busy or idle, when displayed, then Stop discussion appears beside Workflow and is absent from the header.
2. Given an active conversation without a ticket/Workflow chip, when displayed, then Stop discussion remains available in the status strip; moving it must not remove free-conversation stop access.
3. Given an available Stop discussion control, when activated, then it requests the existing stop operation once, refreshes status, and follows the existing stopped callback behavior.
4. Given agent unavailability, stopping in progress, or cancellation already requested, when displayed, then Stop discussion is disabled with existing names, tooltip and stopping feedback preserved.
5. Given a stop failure, when the operation resolves, then existing error feedback is shown and a retry follows the established guards.
6. Given inactive history, when displayed, then no Stop discussion is offered; Archive remains in the header when provided. Read-only history never gains a redundant close action.

### US3 (P2) — Preserve accessibility, layout and conversation state

1. Given a contextual action, when navigating by keyboard, then it is focusable when enabled, has the same accessible name and tooltip as its existing counterpart, and supports standard button activation without toggling Workflow or submitting a message.
2. Given a narrow window or a long workflow stage label, when the status strip is rendered, then actions remain reachable, compact and aligned without obscuring nearby status or composer controls.
3. Given saved draft text, context attachments or attached folders, when controls appear, disappear or launch an action, then no extra clearing, submission or transfer occurs beyond established launch/stop behavior.
4. Given a task run becomes active, stops, or history selection changes, when the UI updates, then visibility and disabled states follow the current lifecycle rather than stale history state.

## Functional requirements

- FR-001: Contextual next-step visibility is the existing header eligibility plus selected read-only history.
- FR-002: Header and contextual next-step share resolution, enabled state, label, tooltip and launch lifecycle.
- FR-003: Active Stop moves to the status strip with its existing operation, feedback and guards; inactive Archive remains in the header.
- FR-004: Workflow expansion, queue access, archive, Start over, draft and context behavior remain intact.
- FR-005: Controls use compact icon-button presentation consistent with adjacent controls and #922; preserve the existing language of each runtime surface.
- FR-006: UI coverage verifies the lifecycle matrix, correct activation, keyboard access and state preservation.
- FR-007: Implementation adds user-facing Unreleased entries to root and Tauri changelogs and updates relevant root/Tauri README guidance.

## Success criteria

All scenarios above are covered by focused UI tests or documented visual verification where layout requires it. The implementation passes the Tauri contributor checks. No duplicate stop request, accidental message submission or workflow advancement is introduced by the new placement.

## Open questions

None. Round 2 accepted all product decisions. No implementation is included in this specification.
