## 1. Repository profile storage and local contract

- [ ] 1.1 Add the bounded UTF-8 Markdown profile store at `.sectile/agents`, shared ID/content validation, deterministic discovery and warnings.
- [ ] 1.2 Implement mapped-primary-root containment and no-follow filesystem access, exclusive atomic creation, cleanup and duplicate protection.
- [ ] 1.3 Add local Desktop list/create routes and optional `agentProfileId` console input; resolve and snapshot before run admission.
- [ ] 1.4 Test missing/readonly roots, empty directory, invalid UTF-8/NUL/blank/oversized prompts, invalid IDs, duplicate/concurrent creation, symlink and ancestor escapes, partial failures, and cross-project isolation.

## 2. Session instruction delivery

- [ ] 2.1 Add independent profile metadata and prompt snapshot to controlled-run/conversation state and restoration; leave engine and project defaults unchanged.
- [ ] 2.2 Implement additive Claude PTY/conversation delivery with proper argv/PTY quoting and preserved native instruction discovery.
- [ ] 2.3 Verify supported Codex configuration and app-server schemas, then implement additive developer instructions for PTY, thread start/resume and collaboration-mode turns without replacing existing instructions.
- [ ] 2.4 Reject unsupported provider/template/version combinations before successful profiled admission; keep unprofiled behavior.
- [ ] 2.5 Add fake-provider and protocol tests for both engines/views: first and later turns, reconnect/restore, repository instruction composition, two simultaneous profiles, no-profile session isolation, stale files before launch, snapshot stability after launch, and shell metacharacters/newlines/Unicode in prompts.
- [ ] 2.6 Retain/add Windows CREATE_NO_WINDOW assertions for any changed/new process construction and test existing console failure reporting.

## 3. Desktop interaction

- [ ] 3.1 Extend Electron IPC/preload with narrow list/create methods and the optional launch selection; test request validation and legacy-agent capability behavior.
- [ ] 3.2 Add project-row plus and share the existing context-menu dialog; test pointer/keyboard actions, project identity, focus, touch availability and non-propagation to row collapse.
- [ ] 3.3 Add independent profile selection and creation form, async loading/error states, success-only selection, draft preservation, cancel/focus handling and duplicate-submit prevention.
- [ ] 3.4 Add UI fixtures covering empty/invalid catalogues, save success/failure, late responses after close/project change, engine changes, unsupported delivery, unavailable checkout and no-profile launch regression. Assert no task creation or defaults mutation.

## 4. Documentation and validation

- [ ] 4.1 Document the directory, format, limits, creation workflow, provider/view support and unsupported templates; update relevant README/Desktop guidance and add an ADR for portable repository profiles.
- [ ] 4.2 Add one user-facing Added entry under CHANGELOG.md Unreleased.
- [ ] 4.3 Run focused Go tests for changed agent paths, `go test ./internal/agent/...` as required, Desktop `npm test`, relevant `npm run test:ui` fixtures and `npm run build`; run the project's required lint/build checks for changed packages.
- [ ] 4.4 Smoke-test real Claude and Codex console/conversation launches with distinctive repository instructions and selected profile, then an unprofiled launch. Confirm the session awaits the user and profile creation only writes the intended repository file.
- [ ] 4.5 Run `openspec validate 869-project-agent-profiles --strict`; report evidence and any provider-version limitations before implementation completion. Preserve attained task stage on retries and follow the configured PR creation owner checks without advancing to reviewed.
