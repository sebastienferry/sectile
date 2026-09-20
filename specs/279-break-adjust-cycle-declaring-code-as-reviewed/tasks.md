# #279 — Implementation checklist

Ordered implementation checklist ensuring each layer is verified before building on top of it: backend/daemon API first, Electron IPC bridge second, workflow engine third, desktop UI fourth, and end-to-end integration tests fifth.

---

## 1. Local Agent Daemon Layer (`internal/agent`)

- [ ] **T1** In `internal/agent/agent_desktop.go`:
  - Add `"transition-stage"` to the `capabilities` string slice returned by `GET /desktop/status`.
  - Add route handler `d.desktopTaskTransition(w, r)` listening on `POST /desktop/tasks/transition`.
  - Validate query parameter `projectId` and body payload containing `taskId`, `stage`, and `note`.
  - Forward the transition request to the Sectile server `POST /api/tasks/{taskId}/stage` using `agenthttp.Client(d.link.token)`.
  - Stream the response code and body back to the caller.
- [ ] **T2** In `internal/agent/agent_desktop_test.go`:
  - Add unit test verifying that `GET /desktop/status` includes `"transition-stage"` in `capabilities`.
  - Add unit test verifying that `POST /desktop/tasks/transition` validates inputs, checks project configuration, forwards request to server, and returns 200 OK.
  - Add unit test verifying that invalid requests (missing task ID or stage) return HTTP 400.

---

## 2. Electron IPC Bridge (`desktop/electron`)

- [ ] **T3** In `desktop/electron/preload.cjs`:
  - Expose `transitionStage(projectId, taskId, stage, note)` on `window.localAgent` using `ipcRenderer.invoke('transition-stage', { projectId, taskId, stage, note })`.
- [ ] **T4** In `desktop/electron/main.cjs`:
  - Register IPC handler `ipcMain.handle('transition-stage', async (_, { projectId, taskId, stage, note }) => ...)`:
    - Validate parameters (`projectId`, `taskId`, `stage`).
    - Verify that `status.capabilities` includes `'transition-stage'`; throw descriptive error if outdated.
    - Dispatch `api('/desktop/tasks/transition?projectId=' + encodeURIComponent(projectId), 'POST', { taskId, stage, note })`.

---

## 3. Workflow Engine (`desktop/src/workflow.mjs`)

- [ ] **T5** In `desktop/src/workflow.mjs`:
  - Update `skills` map to include `reviewed: ['handoff', 'Handoff']`.
  - In `nextTaskStep(task, project)`: remove early return for `stage === 'reviewed'`.
  - Ensure that for a task with `taskStage(task) === 'reviewed'`, if `handoff` is in `project.server.skills`, `nextTaskStep` returns `{ stage: 'reviewed', skillId: 'handoff', label: 'Handoff', message: 'Ready for the next step' }`.
  - Ensure that if `handoff` is not available in `project.server.skills`, `nextTaskStep` returns `{ stage: 'reviewed', message: 'Next skill is unavailable: Handoff' }`.
- [ ] **T6** In `desktop/tests/workflow.ui.cjs`:
  - Update tests asserting `nextTaskStep({ status: 'reviewed' }, project)` with `handoff` skill to expect `{ skillId: 'handoff', label: 'Handoff' }`.
  - Add test asserting that when `handoff` is omitted from `project.server.skills`, `nextTaskStep` reports `'Next skill is unavailable: Handoff'`.

---

## 4. Desktop UI Integration (`desktop/src/`)

- [ ] **T7** In `desktop/src/index.html`:
  - Add `<button id="mark-reviewed" type="button" class="secondary" hidden>Mark reviewed</button>` inside the `#toolbar` action area, alongside `#next-step`.
- [ ] **T8** In `desktop/src/main.js`:
  - Implement `confirmDeclareReviewed(projectId, task)` displaying the confirmation dialog:
    - Title: `Declare ${task.key || task.id} as reviewed?`
    - Description: `This transitions the task to #reviewed and proposes Handoff after human merge.`
    - Primary button: `Confirm`
    - Calls `api.transitionStage(projectId, task.id, 'reviewed', 'Code declared as reviewed from desktop app')`, closes dialog, and invokes `refresh()`.
    - Handles and displays errors in the dialog notice element.
- [ ] **T9** In `desktop/src/main.js` (`ticketRow`):
  - When `taskStage(task) === 'implemented'`:
    - Add a menuitem to the row's `…` more menu: `Declare code as reviewed…`.
    - Wire click handler to trigger `confirmDeclareReviewed(view.projectID, task)`.
    - Disable item if `!view.info.configured` or if an execution is active on the task.
- [ ] **T10** In `desktop/src/main.js` (`renderNextStep` & execution toolbar):
  - In `renderNextStep()`:
    - If `nextStepData?.task` has `taskStage(nextStepData.task) === 'implemented'`: display `#mark-reviewed` button; otherwise hide it.
    - Disable `#mark-reviewed` if `busy || pending`.
  - Wire `#mark-reviewed.onclick` to trigger `confirmDeclareReviewed(run.projectId, nextStepData.task)`.

---

## 5. Verification & Quality Gates

- [ ] **T11** In `desktop/tests/`:
  - Add or update Playwright UI tests in `desktop/tests/next-step.ui.cjs` or a dedicated test file to verify:
    - A task in `implemented` stage displays `Next: Adjust` and `Mark reviewed` button in the execution toolbar.
    - Clicking `Mark reviewed` opens the confirmation dialog.
    - Confirming the dialog calls the transition API, updates task stage to `reviewed`, and refreshes the view.
    - Once in `reviewed` stage, the toolbar displays `Next: Handoff` and the ticket row displays `Run: Handoff`.
- [ ] **T12** Run full Go test suite:
  ```bash
  go test -v -race ./internal/agent/...
  ```
- [ ] **T13** Run full desktop test suite:
  ```bash
  cd desktop && npm test
  ```
- [ ] **T14** Verify all acceptance criteria in `spec.md` (US1 through US5).
