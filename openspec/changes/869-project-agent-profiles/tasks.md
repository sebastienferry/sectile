## 1. Repository profile storage and local contract

- [x] 1.1 Add the bounded UTF-8 Markdown profile store at `.sectile/agents`, shared ID/content validation, deterministic discovery and warnings.
- [x] 1.2 Implement mapped-primary-root containment and no-follow filesystem access, exclusive atomic creation, cleanup and duplicate protection.
- [x] 1.3 Add local Desktop list/create routes and optional `agentProfileId` console input; resolve and snapshot before run admission.
- [x] 1.4 Test missing/readonly roots, empty directory, invalid UTF-8/NUL/blank/oversized prompts, invalid IDs, duplicate/concurrent creation, symlink and ancestor escapes, partial failures, and cross-project isolation.

## 2. Session instruction delivery

- [x] 2.1 Add independent profile metadata and prompt snapshot to controlled-run/conversation state and restoration; leave engine and project defaults unchanged.
- [x] 2.2 Implement additive Claude PTY/conversation delivery with proper argv/PTY quoting and preserved native instruction discovery.
- [x] 2.3 Verify supported Codex configuration and app-server schemas, then implement additive developer instructions for PTY, thread start/resume and collaboration-mode turns without replacing existing instructions.
- [x] 2.4 Reject unsupported provider/template/version combinations before successful profiled admission; keep unprofiled behavior.
- [x] 2.5 Add fake-provider and protocol tests for both engines/views: first and later turns, reconnect/restore, repository instruction composition, two simultaneous profiles, no-profile session isolation, stale files before launch, snapshot stability after launch, and shell metacharacters/newlines/Unicode in prompts.
- [x] 2.6 Retain/add Windows CREATE_NO_WINDOW assertions for any changed/new process construction and test existing console failure reporting.

## 3. Desktop interaction

- [x] 3.1 Extend Electron IPC/preload with narrow list/create methods and the optional launch selection; test request validation and legacy-agent capability behavior.
- [x] 3.2 Add project-row plus and share the existing context-menu dialog; test pointer/keyboard actions, project identity, focus, touch availability and non-propagation to row collapse.
- [x] 3.3 Add independent profile selection and creation form, async loading/error states, success-only selection, draft preservation, cancel/focus handling and duplicate-submit prevention.
- [x] 3.4 Add UI fixtures covering empty/invalid catalogues, save success/failure, late responses after close/project change, engine changes, unsupported delivery, unavailable checkout and no-profile launch regression. Assert no task creation or defaults mutation.

## 4. Documentation and validation

- [x] 4.1 Document the directory, format, limits, creation workflow, provider/view support and unsupported templates; update relevant README/Desktop guidance and add an ADR for portable repository profiles.
- [x] 4.2 Add one user-facing Added entry under CHANGELOG.md Unreleased.
- [x] 4.3 Run focused Go tests for changed agent paths, `go test ./internal/agent/...` as required, Desktop `npm test`, relevant `npm run test:ui` fixtures and `npm run build`; run the project's required lint/build checks for changed packages.
- [ ] 4.4 Authenticated Claude console/conversation smoke verification is delegated by the owner to [#876](https://github.com/sebastienferry/sectile/issues/876) on the professional workstation. Codex conversation and native CLI instruction/no-profile checks pass locally; the interactive console reached its waiting composer, but automated TUI inference verification was abandoned after an unintended CLI update. Profile creation and absence of an automatic initial request are covered by API/UI tests.
- [x] 4.5 Run `openspec validate 869-project-agent-profiles --strict`; report evidence and any provider-version limitations before implementation completion. Preserve attained task stage on retries and follow the configured PR creation owner checks without advancing to reviewed.

## Implementation evidence and owner-approved follow-up

- Implemented profile storage/API, Desktop creation/selection, and session-only additive Claude/Codex delivery. No infrastructure repository changes.
- The run store restores read-only history only, not resumable provider sessions. Profile identity is persisted with desktop metadata; prompts stay in memory and are deliberately absent from history/server payloads. Active provider process reconnects retain their snapshot. See ADR 0061.
- Codex CLI 0.162.1 app-server schema was inspected for config/read, thread start/resume, and collaboration instructions; 0.164.0 compatibility and native instruction/no-profile checks also pass. Older Codex versions are refused conservatively.
- A terminal smoke harness accidentally selected Codex's update prompt and updated the workstation CLI from 0.162.1 to 0.164.0. The owner was informed, and that interactive harness was removed. Retained opt-in smoke tests never drive provider menus.
- Claude capability and fake-provider tests pass. The real CLI returned `Not logged in · Please run /login`; authenticated verification was explicitly delegated by the owner to #876, rather than recorded as passed.
- Validation details are recorded in the implementation report and draft PR #872. Task progression stops at implemented; no reviewed transition is authorized by this invocation.
