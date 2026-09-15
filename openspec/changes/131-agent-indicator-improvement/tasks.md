## Implementation
- [x] Add `web/src/lib/remoteRunIndicator.ts` with the pure state derivation and the cancelled visibility window.
- [x] Add `web/tests/remoteRunIndicator.test.mjs` covering precedence, the cancelled window boundary, cancellable filtering and the empty case.
- [x] Rewrite `web/src/components/RemoteRunBadge.tsx` as a single icon with the hover/focus stop affordance and accessible labelling.
- [x] Run `openspec validate --strict`, the web build, lint, TypeScript check and the Go test suite; review the full branch diff.
- [ ] Update the draft PR with the evidence and mark it ready after the review.
