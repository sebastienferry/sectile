# Task checklist — #307

Ordered so the build stays green at every step. Requirement ids refer to
`spec.md`; design rationale is in `plan.md`.

## 1. Shared disconnect note

- [ ] T1 — Move the disconnect note text to a constant in `internal/models`,
      importable by both `internal/taskmcp` and `internal/db` without a cycle.
      Keep `taskmcp.disconnectNote` as an alias so existing call sites read
      unchanged. *(FR11)*
- [ ] T2 — Build and run the existing `internal/taskmcp` tests; no behaviour
      change is expected at this point.

## 2. Sectile-owned inactivity clock

- [ ] T3 — Add `lastSeen` and `silentSince` to `liveSession`; set `lastSeen` at
      `open`. Nil receiver and empty session id keep tolerating, as everywhere in
      this file.
- [ ] T4 — Add `Touch(sessionID string)`: refresh `lastSeen`, clear
      `silentSince`. *(FR3, FR5)*
- [ ] T5 — Add the `RunNoter` interface and `db.NoteRemoteRun(runID, note)`,
      a thin exported wrapper over `noteRun` restricted to
      `skill_id='remote_run' AND status='running'`. *(FR4)*
- [ ] T6 — Add the sweeper: a goroutine ticking at a fraction of the bound that,
      for each session past the bound with `silentSince == nil`, sets
      `silentSince` and appends one sentence to each adopted run. It cancels
      nothing and touches no other field. Expose the constructor variant taking
      the bound and the noter, plus a `Stop()`/context so tests and shutdown do
      not leak the goroutine. Inject the clock through the existing `now` field.
      *(FR1, FR4, FR5)*
- [ ] T7 — Register a receiving middleware in `NewServerWithCallers` that calls
      `sessions.Touch(req.GetSession().ID())` before delegating. *(FR3)*
- [ ] T8 — `internal/handlers/agent_api.go`: pass `SessionTimeout: 0` to
      `StreamableHTTPOptions` with a comment saying the bound moved into Sectile;
      rename `defaultMCPSessionTimeout` to its new meaning, set the default to
      `4 * time.Hour`, rewrite the comment that still claims a bridge keepalive,
      and hand the parsed bound to the registry at its construction site rather
      than per request. *(FR2, FR6)*

## 3. Recovering a run canceled by a disconnection

- [ ] T9 — Widen the `UPDATE` predicate in `finishRemoteRun` to also match a
      `canceled` run whose summary equals the shared disconnect note, honouring
      the placeholder convention of `internal/db`. Leave the authorization path,
      the `count == 1` hand-back gate and the `count == 0` mismatch message
      untouched. *(FR7, FR8, FR9, FR10)*
- [ ] T10 — Re-read `handBackRun` to confirm no second guard is needed: the
      recovered row affects exactly one row, so the hand-back replays once on the
      corrected status. Note the finding in the PR description if it differs.

## 4. Tests

- [ ] T11 — `internal/taskmcp/sessions_test.go`: a session past the bound keeps
      its adopted runs `running`, appends exactly one sentence, and closes
      nothing. *(US1, US2)*
- [ ] T12 — Same file: a second sweep during the same silence appends nothing;
      a `Touch` followed by a new silence appends a fresh sentence. *(US2)*
- [ ] T13 — Same file: `Close` (DELETE or broken connection) still cancels with
      the disconnect note, and a run reused via `SECTILE_RUN_ID` is still not
      adopted. *(US4)*
- [ ] T14 — `internal/db` remote-run test: a `canceled` run carrying the
      disconnect note is rewritten to `completed` by its owner, and the chain
      hand-back enqueues the next step exactly once. *(US3)*
- [ ] T15 — Same test: a `canceled` run **without** the note is refused; a run
      owned by another non-admin user is refused with `ErrRunNotYours`. *(US3)*
- [ ] T16 — Exercise T14/T15 against both SQLite and PostgreSQL, following the
      dual-store convention established by #296/#302/#304. *(FR12)*
- [ ] T17 — Run the repository's full gate (`make` targets for build, vet/lint
      and tests) and report the output.

## 5. Documentation

- [ ] T18 — `docs/adrs/0007-mcp-session-ownership.md`: short amendment correcting
      the consequence that treated silence as proof of a dead client. The
      decision itself is unchanged. *(US5)*
- [ ] T19 — `README.md` (l.354-356) and `docs/contracts/server-agent-v1.md`
      (l.335): remove the vanished bridge-keepalive justification; describe
      `SECTILE_MCP_SESSION_TIMEOUT` as a silence *notice* bound, default four
      hours, which never cancels a run. *(US5)*
- [ ] T20 — `.env.sample`: same new meaning and default. *(US5)*
- [ ] T21 — `internal/taskmcp/server.go` tool descriptions for `start_run` and
      `finish_run`: keep "a disconnection closes an unfinished run", drop any
      implication that silence does. *(US5)*
- [ ] T22 — `CHANGELOG.md`: unreleased entry, Keep a Changelog conventions.

## Test plan summary

| Level | What it proves |
| --- | --- |
| `internal/taskmcp/sessions_test.go` | Silence marks and never cancels; once per stretch; `Touch` rearms; real endings still cancel |
| `internal/db` remote-run test | Owner recovery of a disconnect-canceled run; UI cancel stays final; one hand-back; both engines |
| Repository gate (`make`) | Build, vet/lint and the full suite stay green |
| Manual, optional | Against a running server: `start_run`, wait past a short configured bound, observe the run still `running` with the appended sentence, then `finish_run` succeeds |

## Not in this checklist

ADR 0012's waiting signal, the stdio bridge, `ServerOptions.KeepAlive`, adoption
semantics, the restart sweep, any new status or column. See `spec.md` §Out of scope.
