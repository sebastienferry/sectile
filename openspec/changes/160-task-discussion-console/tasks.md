## Implementation
- [x] Map the reserved `discuss` id in `dispatchCommand` (`cmd/agent/agent_config.go`) to `runner.InteractiveAgentLaunch`, ignoring any carried prompt, before the configured-skill lookup.
- [x] Admit `discuss` without instructions in the desktop launch validation (`cmd/agent/agent_desktop.go`).
- [x] Add Go tests: discussion launches the bare provider line, a carried prompt is ignored, an unknown id is still rejected, and the desktop launch admits `discuss` with an empty prompt.
- [x] Add the "Discuss" entry to the web task menu (`web/src/components/TaskCard.tsx`) and to the task detail skills area (`web/src/components/TaskDetailModal.tsx`).
- [x] Add the "Discussion" option to the desktop launch and relaunch selectors (`desktop/src/main.js`), with no instructions required.
- [x] Run `openspec validate 160-task-discussion-console --strict`, `go test ./...`, the web test suite, the TypeScript check and the linter; review the full branch diff.
- [ ] Update the draft PR with the evidence and mark it ready after the review.
