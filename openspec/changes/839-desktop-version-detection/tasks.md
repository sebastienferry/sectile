## 1. Main-process release discovery
- [ ] 1.1 Add the injectable update service and state contract described in design.md.
- [ ] 1.2 Implement bounded GitHub metadata retrieval, pagination, strict numeric stable SemVer selection and platform archive matching.
- [ ] 1.3 Test 0.9.0/0.10.0 ordering, equal/older releases, optional v/build metadata, invalid versions, draft/prerelease exclusion, shuffled/paginated releases, missing/zero-size archives and all four published targets plus unsupported targets.
- [ ] 1.4 Test offline, timeout, malformed/oversized metadata, pagination limit, HTTP 403/429 and cooldown without turning errors into up-to-date outcomes.

## 2. Lifecycle, persistence and IPC
- [ ] 2.1 Wire startup, daily deadline, resume and quit; coalesce concurrent manual/automatic requests. Test with an injected clock and pending fetch.
- [ ] 2.2 Persist validated last successful availability/check time and notification-attempt versions without overwriting unrelated settings. Test restart, corrupt/missing settings, changed installed version and persistence failure.
- [ ] 2.3 Add main/preload snapshot, manual check and subscription APIs; validate repository-scoped external links and avoid credentials in network requests.
- [ ] 2.4 Test invalid external URL rejection, safe release/download navigation, stale availability on errors, and clearing availability after successful no-update checks.

## 3. Desktop presentation
- [ ] 3.1 Add a persistent labeled Settings availability marker and Changelog check controls, status/time, available version and browser actions using existing surface language.
- [ ] 3.2 Add one notification attempt per detected version with durable deduplication; test repeated checks/restarts, a newer version, and denied/unavailable notifications.
- [ ] 3.3 Add deterministic UI fixtures/tests for checking, no update, available update, platform/comparison limitation and failed/throttled checks, detached panels and external action activation.
- [ ] 3.4 Verify installed versions/embedded notes offline and agent mismatch/restart remain independent; no automatic archive fetch or installation occurs.

## 4. Delivery validation
- [ ] 4.1 Add the English Unreleased Added changelog entry for the implemented user-visible feature.
- [ ] 4.2 Run `npm --prefix desktop test`, `npm --prefix desktop run build`, and the Desktop UI suite (or document an environmental blocker with focused results). Keep all automated update tests network-independent.
- [ ] 4.3 Run `openspec validate 839-desktop-version-detection --strict` and review each behavior scenario against passing evidence. Reuse the task draft PR; do not advance to reviewed during creation.
