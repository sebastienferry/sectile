# #391: Implementation checklist

References: [`spec.md`](./spec.md), [`plan.md`](./plan.md).

## 1. Shared protocol (FR1, FR6)

- [ ] T1.1 Add `Operations`, `ErrUnsupportedOperation`,
      `UnsupportedOperationError` and `IsUnknownOperationReply` to
      `internal/agentprotocol/operations.go`.
- [ ] T1.2 Unit tests: the three build wordings, `errors.Is`, exact reply
      matching (another operation's reply and a longer text do not match).

## 2. Agent (FR1, FR2, FR11)

- [ ] T2.1 `executeOperation` checks `op.Action` against a set built from
      `agentprotocol.Operations`; the macro actions are listed there too.
- [ ] T2.2 The dial URL carries `agentVersion`, `agentCommit` (when known) and
      `operations`.
- [ ] T2.3 Hash `os.Executable()` once at daemon start; `/desktop/version`
      answers the `version.Info` fields plus `binarySha256`.
- [ ] T2.4 Tests: dial URL parameters, `/desktop/version` fingerprint, every
      listed operation passes the allow-list and an unknown one is still
      refused with `unknown local operation`.

## 3. Server relay (FR3-FR7, FR10)

- [ ] T3.1 `AgentBuild` on `AgentConn`; `HandleAgentConnect` parses the three
      parameters (legacy when `operations` is absent) and passes them to
      `Register` / `registerLocal`.
- [ ] T3.2 Capability check before sending and legacy reply conversion in
      `callOperationLocal`; dispatches (`execute_skill`, `open_terminal`) and
      actions the server does not list stay unchecked.
- [ ] T3.3 `codeAgentOutdated` in `internalAnswer`; `outdatedRelay` in
      `forward`.
- [ ] T3.4 `AgentConnInfo` build fields and `outdated`, filled by
      `ConnectedAgents` and, for locally held connections only, by
      `clusterAgents`.
- [ ] T3.5 Tests: connect handshake (announced and legacy), announced refusal
      sends nothing, legacy conversion and pass-through of other replies,
      cross-instance forwarding keeps text and type, `/api/agent/status`
      fields.

## 4. Callers (FR8, FR9)

- [ ] T4.1 `slicingReadError` uses `errors.Is(err,
      agentprotocol.ErrUnsupportedOperation)`; `macroslicing_test` covers the
      typed error, announced and legacy.
- [ ] T4.2 `errAgentTooOld` wraps `ErrUnsupportedOperation` and names the
      desktop fix; update the assertions that quote its text.
- [ ] T4.3 Replace the string-injected case in `adjustment_test.go` (around
      line 394) with an `UnsupportedOperationError`, asserting device, build,
      `"pr_evidence"`, the fix, the absence of `unknown local operation` and
      `no matching`, and the unchanged stage.

## 5. Desktop (FR12, US4)

- [ ] T5.1 `desktop/electron/agent-identity.cjs`: `fileSha256`,
      `agentOutdated`; `desktop/tests/agent-identity.test.cjs` with the
      decision table of `plan.md`.
- [ ] T5.2 `checkAgentIdentity()` in `main.cjs` after each successful
      connection and after a spawn; `outdated` and `promptedFor` state;
      `agent-outdated` event; `version` IPC returns `outdated`.
- [ ] T5.3 `lifecycle('restart', {reason: 'outdated'})`: extra dialog line;
      on confirmation stop, then start the bundled binary with the saved
      settings. Cancel records the prompt and changes nothing else.
- [ ] T5.4 Preload subscription and the settings panel mark in
      `fillChangelogPanel`, refreshed on `agent-outdated`.
- [ ] T5.5 `settings-version.ui.cjs`: mark shown for an agent without
      `binarySha256`, hidden for the bundled hash (run `npx vite build`
      first).

## 6. Changelog (FR13)

- [ ] T6.1 One `Fixed` line under `[Unreleased]`, for example: "**An outdated
      local agent is named instead of failing obscurely.** When the server asks
      the local agent for something its build cannot do, stage transitions and
      the other features that rely on it now say which agent is outdated and
      that restarting or updating the Sectile desktop app fixes it, instead of
      `unknown local operation`. The desktop app notices when the running agent
      is not the one it bundles, including after a rebuild, offers to restart
      it, and marks it as outdated in its settings until then. (#391)"

## 7. Verification

- [ ] T7.1 `go test ./internal/agentprotocol ./internal/agent
      ./internal/handlers ./internal/db`; `go vet` on the same packages.
- [ ] T7.2 `node --test desktop/tests/*.test.cjs`; the desktop UI test.
- [ ] T7.3 Manual, not blocking: start an agent from an older build, connect
      the desktop, check the prompt, cancel, check the mark, restart from the
      settings and check the mark is gone.
- [ ] T7.4 Manual, operational (spec "Manual verification"): replay the SFE-360
      `implemented` transition with MR !97 once that workstation's agent is
      restarted.
