## Implementation Checklist
- [ ] 1. Add pure batch selection/order/name/prompt helpers and tests for distinct full IDs, project membership, ordering, trimming, length boundaries, invalid paths and existing-work preservation.
- [ ] 2. Extend Electron and local-agent launch serialization with optional ordered batchTaskIds, preserving single-ticket behavior, explicit project, local target device and autonomous pickup mode. Add relay tests and advertise batch-launch capability.
- [ ] 3. Add accessible checkboxes, selected count, clear and prepare actions to Board and Backlog; preserve selection across refresh/filter/view changes within the active project and clear on workspace close/project change. Test both surfaces and navigation/drag/menu regressions.
- [ ] 4. Add preparation dialog, reorder controls, suggested/editable name, validation and cancellation. Test keyboard use, exact launch order and first-member lead.
- [ ] 5. Submit once through existing launch path; gate unsupported agents, suppress duplicate pending submissions, preserve all preparation state on refusal and clear on acceptance. Test busy/stale/mixed-project rejection and no force or per-member launches.
- [ ] 6. Verify existing execution console behavior and server batch membership indicators; confirm the generated prompt requests sequential processing in one shared worktree and one combined PR without overwriting work.
- [ ] 7. Add the user-facing CHANGELOG.md Unreleased Added entry. Run Desktop tests/build, relevant Desktop UI tests, Go agent and handler tests, and repository formatting checks. Record results and preserve artifacts for retry.

## Specification Validation
Run `openspec validate 826-desktop-batch --strict`. Verify every requirement below maps to the checklist, and retain `docs/clarifications/826.md` unchanged.
