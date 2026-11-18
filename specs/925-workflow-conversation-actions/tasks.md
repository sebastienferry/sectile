# Implementation tasks — #925

Implementation and authorized validation are complete. Publication evidence is recorded in `validation.md`.

- [x] T001 Review accepted `docs/clarifications/925.md` Round 2 and `spec.md`; preserve branch work and confirm existing lifecycle contracts in `plan.md`.
- [x] T002 Add optional adjacent action content to `ConversationStatus` without changing Workflow toggle or diagnostics behavior; render actions for free conversations too.
- [x] T003 Reuse one next-step eligibility/rendering contract in `Conversation`; keep header action and add status-strip action only for eligible read-only history. Preserve PR recovery callback and agent disabling.
- [x] T004 Move selected active-run Stop to the strip for both layouts; preserve idle-active access, cancel/stopping guards, request/refresh/callback/error flow. Keep inactive Archive and archived Start over in the header.
- [x] T005 Apply compact icon-button spacing and responsive strip layout consistent with #922. Preserve existing runtime labels and keyboard focus behavior.
- [x] T006 Extend scoped UI assertions for closed history with both next-step controls, writable history with header only, active selected/other task runs with neither, and transitions between these states. Verify archived eligible history and free-conversation Stop.
- [x] T007 Cover unavailable agent, absent task/callback/project configuration/expected skill, finished/unknown stages and PR recovery. Verify exact skill argument and one callback per activation; no Workflow toggle or form submission side effect.
- [x] T008 Cover busy and idle-active Stop, disabled cancellation/stopping/unavailable states, one request, refresh/stopped callback and failure feedback. Verify inactive history has no Stop and retains Archive/Start over as applicable.
- [x] T009 Cover Tab and Enter/Space activation and accessible labels/tooltips. Check draft, attachments and folders survive placement/visibility changes; compare launch/stop effects with existing behavior rather than require new draft transfer.
- [x] T010 Update root/Tauri Unreleased changelog entries and relevant README guidance for the new action locations.
- [x] T011 Run `npm run check`, `npm test`, `npm run test:ui`, `npm run build`, and `npm run test:rust` from `desktop-tauri`. Record results; visually inspect long labels and narrow/wide windows for spacing, reachability, disabled and focus states.
- [x] T012 Review diff against all spec acceptance scenarios and confirm no backend/lifecycle rule change. Complete configured implementation and PR creation checks at the implemented stage; do not mark reviewed merely for creating/recovering a PR.

## Dependencies and completion

T002 precedes placement tasks T003–T004. T005 follows placement; T006–T009 verify it. Documentation and required checks finish before the implemented report. #922 guides visual consistency but does not block this work. No open requirement blocks implementation.
