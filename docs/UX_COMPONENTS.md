# UX Components & Frontend Design

This document details the user interface architecture, component hierarchy, interaction models, and styling patterns used across **TaskFlow**.

---

## 1. Application Layout & Hierarchy

The frontend is a React 19 Single Page Application styled with modern Tailwind CSS and custom CSS variables for dark-mode obsidian themes:

```
App.tsx
├── Navbar (Logo, Project Switcher, View Mode Toggle, Quick Add, Activity Center, Settings)
├── ProjectFilterBar (Horizontal chip-based project selector)
├── Main View Area (Conditional on activeView: 'board' | 'list')
│   ├── BoardView.tsx (Kanban Board with Drag & Drop)
│   └── ListView.tsx (Sortable Tabular List)
├── Modals & Drawers:
│   ├── TaskDetailModal.tsx (Sliding Drawer / Modal Dialog for ticket management)
│   ├── TaskChatDrawer.tsx (Dual-mode Chat Assistant & Interactive Xterm.js PTY Terminal)
│   ├── QuickAddModal.tsx (Rapid task creation with project binding)
│   ├── GitDiffModal.tsx (File-tree and side-by-side Git Diff viewer)
│   ├── ActivityCenter.tsx (Job queue monitor and task output stream)
│   ├── ProjectModal.tsx (Workspace & repository settings)
│   └── SettingsModal.tsx (AI provider, themes, language, Linear/GitHub tokens)
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

### 2.6 Git Diff Inspector (`GitDiffModal.tsx`)
- Displays real-time diffs between the task worktree branch and the base `main` branch.
- Left column: Changed files tree with addition/deletion line counters.
- Right pane: Syntax-highlighted unified diff with chunk navigators and raw diff toggle.

### 2.7 Activity Center (`ActivityCenter.tsx`)
- Drawer monitoring all background agent executions across the entire workspace.
- Real-time counters: Running, Queued, Completed, Failed.
- Real-time stdout/stderr log inspector with auto-scroll.
- Actions to retry failed jobs, cancel running jobs, and clear finished history.
