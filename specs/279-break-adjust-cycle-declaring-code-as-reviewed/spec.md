# #279 — Be able to break the adjust cycle by declaring the code as reviewed

## Context

In the Sectile development workflow, once a task reaches the `implemented` stage and has an associated pull request, the desktop application proposes the `adjust` skill (`Next: Adjust` in the execution toolbar and `Run: Adjust` on the ticket row) to perform code review, handle peer feedback, and update the existing pull request.

However, once the developer or reviewer determines that the code is satisfactory, the desktop application provides no gesture or control to break out of this cycle. The desktop UI continuously proposes `adjust`, trapping the task in an endless adjustment loop. Furthermore, the desktop workflow engine specifically intercepts tasks in the `reviewed` stage to display "Awaiting human merge" while suppressing the next action button, preventing developers from triggering the subsequent `handoff` skill from the execution toolbar.

To establish a smooth, end-to-end SDD workflow, the desktop application must allow developers to explicitly declare code as reviewed. Declaring code as reviewed transitions the task to the `reviewed` workflow stage (`#reviewed` label and `to_close` internal status). In this stage, the desktop application must propose `handoff` as the next action, enabling the user to run the handoff skill to produce the handover summary and clean up the local worktree once human merge is complete.

This specification defines the behaviour and acceptance criteria only. Architectural choices, data contracts, and target files are detailed in [`plan.md`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23279/specs/279-break-adjust-cycle-declaring-code-as-reviewed/plan.md), and the ordered implementation checklist is in [`tasks.md`](file:///Users/sferry/Sources/sectile/.tasks/worktrees/%23279/specs/279-break-adjust-cycle-declaring-code-as-reviewed/tasks.md).

---

## Decisions being specified

1. **Dual Access Points for Declaring Code Reviewed**:
   - **Ticket Row Context**: On the project task list, the `…` (more actions) menu includes an item **Declare code as reviewed** for any task currently in the `implemented` workflow stage.
   - **Execution Toolbar Context**: In the active execution console view, when viewing a task in the `implemented` stage, a secondary action button **Mark reviewed** is displayed alongside the primary `Next: Adjust` button.
2. **Interactive Confirmation Dialog**:
   - Declaring code as reviewed triggers a confirmation dialog explaining the consequence of the transition: *"Declare <key> as reviewed? This transitions the task to #reviewed and proposes Handoff after human merge."*
   - The dialog contains a **Confirm** button and allows cancellation via Escape, clicking outside, or a Cancel button.
3. **Workflow Transition**:
   - Confirming the dialog transitions the task stage to `reviewed` with a descriptive note.
   - Upon successful transition, the desktop UI automatically refreshes task details and workflow indicators across the application.
4. **Handoff Proposed for Reviewed Tasks**:
   - For any task in the `reviewed` workflow stage, the desktop workflow engine resolves the next skill to `handoff`.
   - The execution toolbar displays a primary `Next: Handoff` button with the status message *"Ready for the next step"*.
   - The ticket list row displays `Run: Handoff` as its primary launch action.
5. **Stage Guarding & Availability**:
   - The "Declare code as reviewed" action is available only when the task is in the `implemented` stage. It is hidden or absent for tasks in other stages (`new`, `clarified`, `specified`, `reviewed`, `finished`).
   - Active executions and ongoing submissions disable transition gestures to prevent concurrent mutation conflicts.
6. **Graceful Error Handling**:
   - If the local agent daemon is outdated or unable to perform the transition, or if the server rejects the transition, an explicit error notice is presented directly in the dialog without closing it, enabling the user to retry or inspect the failure.

---

## User stories

### US1 — Declare code reviewed from the ticket row menu (P1)

**As a** developer reviewing tasks on the project board in the desktop application,  
**I want** to declare a task's code as reviewed from its row action menu,  
**So that** I can transition an implemented task to reviewed without needing to open or launch an execution console.

- **Given** a task in the `implemented` workflow stage displayed on the project ticket list
- **When** the user opens the `…` (more actions) menu on the ticket row
- **Then** a menu item labeled **Declare code as reviewed** is present and selectable.

- **Given** a task in any workflow stage other than `implemented` (e.g. `clarified`, `specified`, `reviewed`, or `finished`)
- **When** the user opens the `…` (more actions) menu on the ticket row
- **Then** the **Declare code as reviewed** menu item is not shown.

- **Given** a task in the `implemented` stage with an active running execution
- **When** the user opens the `…` (more actions) menu on the ticket row
- **Then** the **Declare code as reviewed** menu item is disabled, indicating that an execution is currently in progress.

---

### US2 — Declare code reviewed from the execution toolbar (P1)

**As a** developer inspecting diffs, test outputs, or skill results in the execution console,  
**I want** a secondary action button to declare the code as reviewed directly in the toolbar,  
**So that** I can immediately approve the code after reviewing the console output without leaving the view.

- **Given** the execution view is displaying a task in the `implemented` stage
- **When** the execution toolbar renders the task's next actions
- **Then** the primary action button displays `Next: Adjust`.
- **And** a secondary action button labeled **Mark reviewed** is displayed in the toolbar.

- **Given** the execution view is displaying a task in any stage other than `implemented` (e.g. `specified`, `reviewed`, or `finished`)
- **When** the execution toolbar renders
- **Then** the **Mark reviewed** secondary action button is hidden.

- **Given** an execution is actively running on the current task
- **When** the execution toolbar renders
- **Then** both the `Next: Adjust` button and the **Mark reviewed** button are disabled while the execution is active.

---

### US3 — Confirmation dialog and stage transition execution (P1)

**As a** developer,  
**I want** a confirmation dialog when declaring code as reviewed,  
**So that** I do not inadvertently advance the task stage by a misclick.

- **Given** the user triggers "Declare code as reviewed" from either the ticket row menu or the execution toolbar
- **When** the gesture is initiated
- **Then** a confirmation dialog opens with the title `Declare <task key> as reviewed?`.
- **And** the dialog text states: *"This transitions the task to #reviewed and proposes Handoff after human merge."*
- **And** the dialog provides a primary **Confirm** button and dismiss options.

- **Given** the confirmation dialog is open
- **When** the user presses Escape or dismisses the dialog
- **Then** the dialog closes without transitioning the task or mutating any remote tracker state.

- **Given** the confirmation dialog is open
- **When** the user clicks **Confirm**
- **Then** the confirm button is disabled and a status notice indicates transition is in progress.
- **And** the task is transitioned to the `reviewed` stage on the server.
- **And** the dialog closes automatically upon success.
- **And** the desktop view refreshes to reflect the new `#reviewed` stage.

---

### US4 — Proposed next step becomes Handoff for reviewed tasks (P1)

**As a** developer with a task in the `reviewed` stage,  
**I want** the desktop application to propose the `handoff` skill as the next step,  
**So that** I can trigger handoff to produce the handover summary and clean up the workspace once the pull request has been merged.

- **Given** a task in the `reviewed` workflow stage (or with label `#reviewed` or status `to_close`)
- **And** the project configuration has the `handoff` skill available
- **When** the execution toolbar renders the task's workflow status
- **Then** the status line indicates that the task is in `reviewed` stage and ready for the next step.
- **And** the primary next-step button displays `Next: Handoff` and is enabled.
- **And** the **Mark reviewed** button is hidden.

- **Given** a task in the `reviewed` workflow stage
- **When** the ticket list row renders for this task
- **Then** the primary action button displays `Run: Handoff`.
- **And** the stage column displays `reviewed`.

- **Given** the user clicks `Next: Handoff` in the execution toolbar or `Run: Handoff` on the ticket row
- **When** the action is clicked
- **Then** the `handoff` skill is launched for the task in the desktop console.

---

### US5 — Outdated local agent and error feedback (P2)

**As a** developer whose local agent daemon is outdated or experiencing network issues,  
**I want** informative error feedback if the stage transition fails,  
**So that** I understand why the action could not be completed and how to recover.

- **Given** the running local agent daemon lacks the capability to transition task stages
- **When** the user clicks **Confirm** in the confirmation dialog
- **Then** the dialog displays an error notice: *"The running local agent does not support stage transitions. Update and restart the agent."*
- **And** the confirm button is re-enabled to allow retry once the agent is restarted.

- **Given** the server rejects the transition (e.g. server offline or network timeout)
- **When** the user clicks **Confirm** in the confirmation dialog
- **Then** the dialog remains open and displays the server error message.
- **And** the confirm button is re-enabled for retry.
