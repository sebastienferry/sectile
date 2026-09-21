# #279 — Implementation plan

## Stack

- **Go 1.x Agent Daemon**: `internal/agent/agent_desktop.go` (local agent HTTP API endpoints, daemon capabilities, request forwarding to central server).
- **Go 1.x Server Backend**: `internal/handlers/handlers.go` (existing `POST /api/tasks/{id}/stage` endpoint, stage verification, tracker update queuing).
- **Electron Companion Runtime**: `desktop/electron/preload.cjs` and `desktop/electron/main.cjs` (IPC channels, capability checks, secure IPC exposure).
- **Desktop Frontend Application**:
  - `desktop/src/workflow.mjs` (workflow stage and skill resolution, `skills` table, `nextTaskStep`).
  - `desktop/src/main.js` (ticket row menu items, execution toolbar controls, confirmation dialog, state refreshing).
  - `desktop/src/index.html` (toolbar DOM structure for secondary reviewed action).

---

## Architecture decisions

### D1 — Local Agent Daemon: `POST /desktop/tasks/transition`

The local agent daemon acts as the trusted bridge between the desktop companion and the Sectile server.

1. **Capabilities Announcement**:
   In `internal/agent/agent_desktop.go`, update `/desktop/status` to include `"transition-stage"` in its `capabilities` list:
   ```go
   capabilities := []string{"git-diff", "create-task", "remove-project", "free-console", "transition-stage"}
   ```

2. **Transition Route**:
   Add `POST /desktop/tasks/transition` to `internal/agent/agent_desktop.go`:
   - Validates that `projectId`, `taskId`, and `stage` are non-empty.
   - Fetches configuration using `d.fetchConfig(r.Context(), input.ProjectID, "")` to ensure project validity.
   - Forwards the request via authenticated `agenthttp.Client(d.link.token)` to the central server:
     `POST /api/tasks/<taskID>/stage`
   - Request payload:
     ```json
     {
       "stage": "reviewed",
       "note": "Code declared as reviewed from desktop app"
     }
     ```
   - Proxies the HTTP status code and response body back to the caller.

```
+------------------+         +------------------+         +-----------------+
|  Desktop App UI  |  IPC    |  Electron Main   |  HTTP   |   Local Agent   |  HTTP   +----------------+
|  (main.js)       | ------> |  (main.cjs)      | ------> |   Daemon        | ------> | Sectile Server |
|                  |         |                  |         |  (/desktop/...) |         |  (/api/tasks)  |
+------------------+         +------------------+         +-----------------+         +----------------+
```

---

### D2 — Electron IPC Bridge (`preload.cjs` and `main.cjs`)

1. **Preload Exposure (`desktop/electron/preload.cjs`)**:
   Expose `transitionStage`:
   ```javascript
   transitionStage: (projectId, taskId, stage, note) => ipcRenderer.invoke('transition-stage', { projectId, taskId, stage, note }),
   ```

2. **IPC Handler (`desktop/electron/main.cjs`)**:
   Register `transition-stage`:
   ```javascript
   ipcMain.handle('transition-stage', async (_, { projectId, taskId, stage, note }) => {
     if (!projectId || !taskId || !stage) throw Error('Project, task, and stage required')
     const status = await api('/desktop/status')
     if (!status.capabilities?.includes('transition-stage')) {
       throw Error('The running local agent does not support stage transitions. Update and restart the agent.')
     }
     return api('/desktop/tasks/transition?projectId=' + encodeURIComponent(projectId), 'POST', { taskId, stage, note })
   })
   ```

---

### D3 — Desktop Workflow Engine (`desktop/src/workflow.mjs`)

Currently, `desktop/src/workflow.mjs` defines:
```javascript
const skills = { new: ['clarify', 'Clarify'], clarified: ['specify', 'Specify'], specified: ['implement', 'Implement'], implemented: implementedStep }
```
And in `nextTaskStep`:
```javascript
if (stage === 'reviewed') return { stage, message: 'Awaiting human merge' }
```

This causes two problems:
1. `skills.reviewed` is missing, so `reviewed` has no mapped skill.
2. `nextTaskStep` intercepts `reviewed` before checking `skills`, returning a message without `skillId`, which causes `renderNextStep` in `main.js` to hide the `#next-step` button.

**Changes**:
1. Map `reviewed: ['handoff', 'Handoff']` in `skills`:
   ```javascript
   const skills = {
     new: ['clarify', 'Clarify'],
     clarified: ['specify', 'Specify'],
     specified: ['implement', 'Implement'],
     implemented: implementedStep,
     reviewed: ['handoff', 'Handoff'],
   }
   ```
2. Remove the early `if (stage === 'reviewed')` return from `nextTaskStep`.
3. For a task in `reviewed` stage with `handoff` present in `project.server.skills`, `nextTaskStep` returns:
   ```javascript
   { stage: 'reviewed', skillId: 'handoff', label: 'Handoff', message: 'Ready for the next step' }
   ```
4. If `handoff` is not available in `project.server.skills`, it returns:
   ```javascript
   { stage: 'reviewed', message: 'Next skill is unavailable: Handoff' }
   ```
5. Preserve `closingStep(task, project)` for compatibility with `offerClosure`.

---

### D4 — Desktop UI Integration (`desktop/src/main.js` & `index.html`)

1. **Ticket Row More Menu**:
   In `ticketRow(view, task)`:
   - When `taskStage(task) === 'implemented'`:
     Add an action item before custom instructions:
     ```javascript
     { label: 'Declare code as reviewed…', transition: 'reviewed' }
     ```
   - When clicked: call `confirmDeclareReviewed(view.projectID, task)`.
   - Disabled if `!view.info.configured` or if an execution is active on the task.

2. **Execution Toolbar Secondary Button**:
   In `desktop/src/index.html`:
   - Add `#mark-reviewed` button in `#toolbar` adjacent to `#next-step`:
     ```html
     <button id="mark-reviewed" type="button" class="secondary" hidden>Mark reviewed</button>
     ```
   In `desktop/src/main.js`:
   - In `renderNextStep()`:
     - Check `const stage = taskStage(nextStepData.task)`.
     - If `stage === 'implemented'`: show `#mark-reviewed`.
     - Otherwise: hide `#mark-reviewed`.
     - Disabled if `busy || pending`.
   - Wire click handler:
     ```javascript
     document.querySelector('#mark-reviewed').onclick = () => {
       const run = currentTaskRun()
       if (run && nextStepData?.task && taskStage(nextStepData.task) === 'implemented') {
         confirmDeclareReviewed(run.projectId, nextStepData.task)
       }
     }
     ```

3. **Confirmation Dialog Helper**:
   Implement `confirmDeclareReviewed(projectId, task)`:
   ```javascript
   async function confirmDeclareReviewed(projectId, task) {
     const key = task.key || task.id
     showDialog('Declare ' + key + ' as reviewed?')
     paragraph('This transitions the task to #reviewed and proposes Handoff after human merge.')
     const confirm = document.createElement('button')
     confirm.textContent = 'Confirm'
     const notice = document.createElement('p')
     notice.setAttribute('role', 'status')
     dialogBody.append(confirm, notice)
     confirm.focus()
     confirm.onclick = async () => {
       confirm.disabled = true
       notice.textContent = 'Declaring code as reviewed…'
       try {
         await api.transitionStage(projectId, task.id, 'reviewed', 'Code declared as reviewed from desktop app')
         dialog.close()
         await refresh()
       } catch (err) {
         notice.textContent = err.message
         confirm.disabled = false
       }
     }
   }
   ```

---

## Data contracts

### IPC Contract: `transition-stage`
```typescript
interface TransitionStageIPCInput {
  projectId: string;
  taskId: string;
  stage: 'reviewed';
  note: string;
}
```

### Local Agent Daemon HTTP Contract: `POST /desktop/tasks/transition`
- Query param: `projectId=<id>`
- Body:
  ```json
  {
    "taskId": "task-uuid",
    "stage": "reviewed",
    "note": "Code declared as reviewed from desktop app"
  }
  ```
- Response: 200 OK with JSON representation of updated task, or 4xx/5xx with error message.

### Server API Contract: `POST /api/tasks/{id}/stage`
- Path: `/api/tasks/<taskId>/stage`
- Body:
  ```json
  {
    "stage": "reviewed",
    "note": "Code declared as reviewed from desktop app"
  }
  ```
- Response: 200 OK `{"success": true, "message": "...", "task": { ... }}`.

---

## Target files

1. [`internal/agent/agent_desktop.go`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23279/internal/agent/agent_desktop.go): Add `"transition-stage"` capability and `POST /desktop/tasks/transition` proxy handler.
2. [`internal/agent/agent_desktop_test.go`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23279/internal/agent/agent_desktop_test.go): Unit tests for `/desktop/tasks/transition` route and capability reporting.
3. [`desktop/electron/preload.cjs`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23279/desktop/electron/preload.cjs): Expose `transitionStage` method to renderer via `contextBridge`.
4. [`desktop/electron/main.cjs`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23279/desktop/electron/main.cjs): Register `transition-stage` IPC handler and capability guard.
5. [`desktop/src/workflow.mjs`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23279/desktop/src/workflow.mjs): Add `reviewed: ['handoff', 'Handoff']` to `skills` and remove blocking `reviewed` branch in `nextTaskStep`.
6. [`desktop/src/index.html`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23279/desktop/src/index.html): Add `#mark-reviewed` button in execution `#toolbar`.
7. [`desktop/src/main.js`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23279/desktop/src/main.js): Add row menu item, toolbar button behavior, and `confirmDeclareReviewed` dialog.
8. [`desktop/tests/workflow.ui.cjs`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23279/desktop/tests/workflow.ui.cjs): Test `reviewed` stage mapping to `handoff` skill.
9. [`desktop/tests/next-step.ui.cjs`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23279/desktop/tests/next-step.ui.cjs): Update next-step integration tests to verify `Next: Handoff` and `#mark-reviewed`.
