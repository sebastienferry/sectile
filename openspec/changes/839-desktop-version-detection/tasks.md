## 1. Main-process release discovery
- [x] 1.1 Add the injectable update service and state contract described in design.md.
- [x] 1.2 Implement bounded GitHub metadata retrieval, pagination, strict numeric stable SemVer selection and platform archive matching.
- [x] 1.3 Test 0.9.0/0.10.0 ordering, equal/older releases, optional v/build metadata, invalid versions, draft/prerelease exclusion, shuffled/paginated releases, missing/zero-size archives and all four published targets plus unsupported targets.
- [x] 1.4 Test offline, timeout, malformed/oversized metadata, pagination limit, HTTP 403/429 and cooldown without turning errors into up-to-date outcomes.

## 2. Lifecycle, persistence and IPC
- [x] 2.1 Wire startup, daily deadline, resume and quit; coalesce concurrent manual/automatic requests. Test with an injected clock and pending fetch.
- [x] 2.2 Persist validated last successful availability/check time and notification-attempt versions without overwriting unrelated settings. Test restart, corrupt/missing settings, changed installed version and persistence failure.
- [x] 2.3 Add main/preload snapshot, manual check and subscription APIs; validate repository-scoped external links and avoid credentials in network requests.
- [x] 2.4 Test invalid external URL rejection, safe release/download navigation, stale availability on errors, and clearing availability after successful no-update checks.

## 3. Desktop presentation
- [x] 3.1 Add a persistent labeled Settings availability marker and Changelog check controls, status/time, available version and browser actions using existing surface language.
- [x] 3.2 Add one notification attempt per detected version with durable deduplication; test repeated checks/restarts, a newer version, and denied/unavailable notifications.
- [x] 3.3 Add deterministic UI fixtures/tests for checking, no update, available update, platform/comparison limitation and failed/throttled checks, detached panels and external action activation.
- [x] 3.4 Verify installed versions/embedded notes offline and agent mismatch/restart remain independent; no automatic archive fetch or installation occurs.

## 4. Delivery validation
- [x] 4.1 Add the English Unreleased Added changelog entry for the implemented user-visible feature.
- [x] 4.2 Run `npm --prefix desktop test`, `npm --prefix desktop run build`, and the Desktop UI suite (or document an environmental blocker with focused results). Keep all automated update tests network-independent.
- [x] 4.3 Run `openspec validate 839-desktop-version-detection --strict` and review each behavior scenario against passing evidence. Reuse the task draft PR; do not advance to reviewed during creation.

## Implementation evidence (2026-10-14)

- `desktop/electron/desktop-updates.cjs` owns numeric stable version selection, exact archive matching, credential-free bounded metadata retrieval, coalescing, daily/resume scheduling, persisted last-success metadata and notification claims. Nine deterministic service tests cover selection, target mapping, failures, pagination, timeouts, cooldowns, restart state, persistence failures, denied notifications, new versions and scheduling.
- `desktop/electron/main.cjs` wires the service to application readiness, resume/quit, Desktop-owned settings, native notifications, broadcasts and validated browser actions. `desktop/electron/preload.cjs` exposes explicit snapshot/check/subscription/navigation methods. Automatic network checks are disabled in the existing Desktop test mode.
- `desktop/src/desktop-updates.mjs` renders update controls and public status codes as text. `desktop/src/main.js` connects the global availability marker and Changelog panel without retaining panel subscriptions. `desktop/src/style.css` accommodates the marker in normal and collapsed sidebars. `desktop/tests/desktop-updates.ui.cjs` covers status variants, offline notes/versions, marker lifetime and explicit browser actions.
- `CHANGELOG.md` describes detection and release/download navigation under Unreleased → Added. Accepted clarification and design artifacts remain intact; no installation, rolling-channel support or independent agent/server update checking was added.
- `npm --prefix desktop test`: `tests 295`, `pass 295`, `fail 0` with the local socket/process access required by existing tests. The restricted sandbox initially failed six existing environmental tests; these pass with the required access.
- `npm --prefix desktop run build`: `1018 modules transformed`, build successful; existing warning for chunks larger than 500 kB remains. Desktop has no separate lint script; syntax checks for changed Electron files and `git diff --check` pass.
- Full UI command: `node --test --test-timeout=45000 --test-concurrency=1 desktop/tests/*.ui.cjs`: `tests 137`, `pass 136`, `fail 1`. `git-diff.ui.cjs` timed out waiting for the Task a row; its isolated retry passed, and the unchanged HEAD built in a temporary directory also passed that isolated test. This is recorded as a transient full-suite timeout, not a completely green full-suite result. Update UI and both existing settings-version tests pass. The initial restricted UI run could not bind local sockets; an initial unrestricted run was interrupted and replaced by this bounded complete run.
- `openspec validate 839-desktop-version-detection --strict`: `Change '839-desktop-version-detection' is valid`.
- Reuse draft PR #844 on `feat/839` targeting `main`; retain draft status and transition only to implemented, never reviewed.
