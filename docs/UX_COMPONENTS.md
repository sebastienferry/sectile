# UX Components & Frontend Design

This document details the user interface architecture, component hierarchy, interaction models, and styling patterns used across **Sectile**.

---

## 1. Application Layout & Hierarchy

The frontend is a React 19 Single Page Application styled with modern Tailwind CSS and custom CSS variables for Graphite light and dark themes:

```
App.tsx
├── Sidebar (Project Selection, View Navigation, Saved Board Views, Activity and Sync Indicators)
├── Header (Project Settings, Open Saved View, Global Search, Active Filter Chips, Quick Add)
├── ProjectFilterBar (Horizontal chip-based project selector)
├── Main View Area (Conditional on activeView: 'board' | 'list')
│   ├── BoardView.tsx (Kanban Board with Drag & Drop)
│   └── ListView.tsx (Sortable Tabular List)
├── Modals & Drawers:
│   ├── TaskDetailModal.tsx (Sliding Drawer / Modal Dialog for ticket management)
│   ├── TaskChatDrawer.tsx (Dual-mode Chat Assistant & Interactive Xterm.js PTY Terminal)
│   ├── QuickAddModal.tsx (Rapid task creation with project binding)
│   ├── (Local Git inspection belongs to Desktop; see section 2.6)
│   ├── ActivityCenter.tsx (Job queue monitor and task output stream)
│   ├── ProjectModal.tsx (Workspace & repository settings)
│   ├── BoardViewModal.tsx (Create, edit or delete a saved board view: name, projects, labels)
│   └── SettingsModal.tsx (AI provider, themes, language, tracker tokens)
```

---

## 2. Component Specifications

### 2.1 Kanban Board (`BoardView.tsx`)
- **Technology**: Built using `@dnd-kit/core`, `@dnd-kit/sortable`, and `@dnd-kit/utilities`.
- **Columns**: Mapped to the 6 core workflow stages:
  1. **To Clarify** (`#new`)
  2. **To Specify** (`#clarified`)
  3. **To Implement** (`#specified`)
  4. **To Test** (`#implemented`)
  5. **In Review / PR** (`#reviewed`)
  6. **Finished** (`#finished`)
- **Behaviors**:
  - Dragging a task card across columns executes an optimistic UI update and triggers `updateTask({ status })`.
  - Automatically updates external tracker status if configured in the background using the runner.

### 2.2 Task Card (`TaskCard.tsx`)
- **Card Header**:
  - Direct clickable tracker badge.
  - Task priority badge (`urgent` flame, `high` amber, `medium` blue, `low` slate).
- **Body**:
  - Title and 2-line truncated description.
  - Color-coded workflow stage tags (`#new`, `#clarified`, `#specified`, `#implemented`, `#reviewed`).
  - Git Branch pill with branch icon (clicking opens the Git Diff viewer).
  - Pull Request / Merge Request pill with direct external link.
- **Latest Activity Badge**:
  - Automatically computes the latest active or completed activity on the task.
  - Displays animated spinner for `running` jobs, green check for `completed`, red alert for `failed`.
  - Clicking the activity badge opens the Task Chat Drawer directly.
- **Hover Action Bar**:
  - `Chat`: Opens the Copilot Chat / PTY Drawer.
  - `Clarify` / `Code`: One-click skill execution.

### 2.3 Tabular List View (`ListView.tsx`)
- Tabular representation of all tasks for high-density management.
- Multi-column sorting on Key, Title, Status, Priority, Due Date, and Created Date.
- "Group by Status" accordion toggle.
- Quick status dropdown selector and live activity indicators on every row.

### 2.4 Task Chat Drawer & Interactive Terminal (`TaskChatDrawer.tsx` & `InteractiveTerminal.tsx`)
The drawer slides in from the right edge of the screen and hosts a single view: the
task's real TTY session. There is no separate non-TTY chat mode, so every exchange
with the agent happens in the shell where the agent CLI actually runs.

#### Interactive ZSH PTY Terminal
- Mounted via `<InteractiveTerminal task={chatTask} />`.
- Connects directly to the Go WebSocket endpoint `/ws/terminal?taskId=<id>`.
- Embeds a full Xterm.js terminal emulator with auto-fit addon and dark obsidian theme.
- Directly controls the shell running in `.tasks/worktrees/<taskKey>`.
- Action toolbar: `Launch agent`, `/clarify`, `/specify`, `/code`, `/adjust-issue`, `Ctrl+C`, `Clear`, `Reset`.

### 2.5 Task Detail Modal (`TaskDetailModal.tsx`)
- Supports two display modes: **Sliding Panel** (default) or **Center Modal Dialog** (switchable via settings).
- Full ticket editing: Title, Markdown Description, Status, Priority, Project binding, Assignee, Due Date, Labels.
- Dedicated tabs for:
  - **Details**: Core ticket information and Speckit QA requirements.
  - **Skills**: Manual skill runner with prompt overrides.
  - **History**: Complete chronological audit log of all activities, commands, and outputs.

### 2.6 Desktop Changes inspector (`desktop/src/gitDiff.js`)

The selected execution has keyboard-operable **Console** and **Changes** controls.
Changes loads a local comparison on opening and offers explicit **Refresh**. A
selectable file list accompanies a unified patch pane; paths, code, and warning
text are inert text nodes. Additions, deletions, and hunk headers have distinct
colors. Narrow windows keep file navigation and patch scrolling accessible.

Context identifies the actual directory, branch, local default reference, ancestor,
and read time. Loading, empty, error, and partial states are explicit; binary,
submodule, oversized, and unsupported contents have markers. File selection survives
refresh when still present. Failed refreshes clear source results. Request generations
discard obsolete responses after selection changes, view closure, or disconnect.
Switching views preserves the PTY; returning to Console restores focus and size.
The web app does not retrieve or display this local source comparison (ADR 0003).


### 2.7 Activity Center (`ActivityCenter.tsx`)
- Drawer monitoring all background agent executions across the entire workspace.
- Real-time counters: Running, Queued, Completed, Failed.
- Real-time stdout/stderr log inspector with auto-scroll.
- Actions to retry failed jobs, cancel running jobs, and clear finished history.

## Graphite styling

The web theme uses neutral Graphite surfaces with project accents reserved for
selection and actions. Theme tokens live in `web/src/index.css`; the header and
sidebar use opaque surfaces, and board lanes share the canvas background.
Expanded task cards use 12px corners and the selected density padding; condensed
rows preserve their compact geometry. Card transitions respect reduced motion.
