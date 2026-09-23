# Re-Implementation Guide for LLMs & Engineers

This document is an actionable, step-by-step blueprint designed to enable another AI model or software engineer to re-implement **Sectile** from scratch with full fidelity.

---

## 1. Stack & Dependencies

### Backend (Go 1.22+)
- `github.com/mattn/go-sqlite3`: Embedded SQLite driver.
- `github.com/creack/pty`: Unix pseudo-terminal (PTY) spawning and window resize.
- `github.com/gorilla/websocket`: WebSocket server for live streaming terminal I/O.

### Frontend (React 19 + TypeScript + Vite)
- `@dnd-kit/core`, `@dnd-kit/sortable`, `@dnd-kit/utilities`: Accessible Kanban drag-and-drop.
- `@xterm/xterm`, `@xterm/addon-fit`, `@xterm/addon-web-links`: Full browser terminal emulator.
- `lucide-react`: Modern iconography.
- `tailwindcss` (v4): Responsive dark theme styling.

---

## 2. Step-by-Step Implementation Blueprint

### Step 1: Database & Concurrency Layer (`internal/db/db.go`)
1. Create SQLite tables with WAL mode enabled (`PRAGMA journal_mode=WAL;`).
2. Implement `initSchema()` creating clean empty tables without hardcoded user data.
3. Protect database operations with a `sync.RWMutex`.
4. **Deadlock Prevention Rule**: Always implement private query helpers (e.g. `getProjectsUnsafe()`, `getSettingsUnsafe()`) that do not acquire locks. Exported public functions acquire `mu.RLock()` or `mu.Lock()` and call these unsafe helpers.

### Step 2: Git Worktree & Skill Engine (`internal/db/db.go`)
1. Implement `EnsureTaskWorktree(mainRepoPath, task)`:
   - Check if `repo_path` has a `.git` folder.
   - Append `.tasks/` to `.gitignore` automatically.
   - Compute task branch name `fmt.Sprintf("%s-%s", task.Key, slugifiedTitle)`.
   - Run `git worktree add .tasks/worktrees/<taskKey> -b <branchName> main`.
   - Install JavaScript dependencies inside the worktree (the local agent does this in `internal/agent/provision.go`, after resolving the worktree and before the session starts): for the worktree root and each direct, non-hidden subdirectory holding a `package.json` and a `package-lock.json`, run `npm ci` and write `node_modules/.install-stamp`. Reinstall only when `package.json` or `package-lock.json` is newer than the stamp. Skip the main checkout, and skip any `node_modules` that is a link rather than a plain directory. Nothing is linked from the main checkout, and `.env` / `.env.local` are neither linked nor copied. A failed or timed-out install is logged and never blocks the launch.
   - Symlink `.gemini`, `.agents`, `.agy`, `.taskflow` folders.
   - Ensure the 5 default skill templates (`clarify-issue`, `specify-issue`, `code-issue`, `adjust-issue`, `pick-issue`) are written into `.gemini/skills/` and `.agents/skills/`.
2. Implement `RemoveTaskWorktree(mainRepoPath, taskKey)`:
   - Run `git worktree remove --force` and `git worktree prune`.

### Step 3: Interactive PTY Manager (`internal/terminal/terminal.go`)
1. Define `Session` with `pty.StartWithSize` running `/bin/zsh -l` (or `$SHELL`).
2. Set session `Dir` to the task worktree path.
3. Inject environment variables: `SECTILE_TASK_ID`, `SECTILE_TASK_KEY`, `SECTILE_TASK_WORKTREE`.
4. Buffer output into a 64KB ring buffer for tab reconnects.
5. Handle WebSocket connection `/ws/terminal`:
   - Stream PTY read buffer to WebSocket client.
   - Write WebSocket client input directly to PTY master.
   - On resize JSON message (`{"type": "resize", "cols": N, "rows": M}`), call `pty.Setsize()`.

### Step 4: Subprocess Execution Runner (`internal/runner/runner.go`)
1. Implement `GetDynamicCustomPath()` to search `~/.local/bin`, `/opt/homebrew/bin`, `/usr/local/bin`, `/usr/bin`, `/bin`.
2. Implement `FindCliTool(tool)` to locate `agy`, `claude`, `vibe`, etc.
3. Execute agent skills with stdout/stderr capture and cancellation context.

### Step 5: Frontend State & Context (`web/src/context/AppContext.tsx`)
1. Manage global state: `tasks`, `projects`, `activities`, `selectedProjectId`, `settings`.
2. Provide CRUD methods: `createTask`, `updateTask`, `deleteTask`, `runSkill`, `checkoutTaskBranch`.
3. Provide real-time polling or WebSocket synchronization for background activities.

### Step 6: Frontend UX Components
1. **Kanban (`BoardView.tsx` & `TaskCard.tsx`)**:
   - Render 6 stage columns with drag-and-drop.
   - Render task cards showing priority, tracker badge, branch pill, PR link, and **latest activity badge** (computing running/completed jobs).
2. **List View (`ListView.tsx`)**:
   - Tabular representation with multi-column sorting and grouping.
3. **Chat & Terminal Drawer (`TaskChatDrawer.tsx` & `InteractiveTerminal.tsx`)**:
   - Header with tab toggle (`💬 Discussion` vs `💻 Terminal Zsh`).
   - Chat tab: Markdown message stream + action buttons.
   - Terminal tab: Mounted Xterm.js instance connected to `/ws/terminal`.
4. **Task Detail (`TaskDetailModal.tsx`)**:
   - Sliding right drawer or modal dialog with Speckit specification viewer.
5. **Diff Inspector (`GitDiffModal.tsx`)**:
   - File tree and colored unified diff.

---

## 3. Invariants & Critical Rules

> [!CAUTION]
> 1. **No Personal References in Code**: Never commit hardcoded user home directories (`/Users/...`), personal emails, or private company names.
> 2. **Project-Bound Tracker**: The task creation modal must **never** ask the user to pick an issue tracker manually. The user chooses the **Project**, and the tracker is inherited 100% from the project configuration.
> 3. **Never Commit Binaries or DBs**: Ensure `tasks.db*`, `bin/`, and `node_modules` are excluded via `.gitignore`.
> 4. **Always Test Worktree Independence**: AI agents executing code in `.tasks/worktrees/<taskKey>` must never corrupt or modify the main working tree repository.

---

## 4. Verification Checklist

- [ ] `go test ./...` passes in under 2 seconds.
- [ ] `go build -o /dev/null ./cmd/server` succeeds with 0 warnings.
- [ ] `cd web && npm run build` compiles TypeScript with 0 errors.
- [ ] Starting server (`go run ./cmd/server`) listens on port 8090.
- [ ] Creating a task generates an isolated worktree under `.tasks/worktrees/<KEY>`.
- [ ] Opening the Terminal tab in the drawer establishes a live WebSocket connection to ZSH with working keystrokes and `agy` triggers.
