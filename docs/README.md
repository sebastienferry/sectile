# Sectile Architecture & Engineering Documentation

Welcome to the technical and user documentation for **Sectile**. This directory
provides user guides, architectural blueprints, server-agent contracts, data
specifications, and Architecture Decision Records (ADRs) for the Sectile
server, workstation agent, web interface, and desktop companion.

---

## Documentation Index

### User & Workflow Guides

- [**Embedded Web Help Guide** (`../web/public/help/guide.html`)](../web/public/help/guide.html)
  - Standalone interactive HTML guide served by the Sectile server at
    `/help/guide.html` (accessible from the status bar **Guide** link or the
    Command Palette `?` shortcut).
  - Includes UI screenshots, Light/Dark theme toggle, live `/` search, and 7
    step-by-step How-To guides.
- [**User Guide (Markdown)** (`USER_GUIDE.md`)](./USER_GUIDE.md)
  - Sign in, seal personal tracker credentials, configure projects, pair
    workstations, and run SDD workflows from the web board, Sectile Desktop, or
    Claude Code / Codex.
- [**Desktop Guide** (`../desktop/README.md`)](../desktop/README.md)
  - Installing Sectile Desktop via Homebrew (`macOS`), Scoop (`Windows`), or
    release archives; using the interactive Claude Code & Codex conversation
    view, native terminals, desktop board, and worktree diff inspector.

### Architecture & Engineering Reference

1. [**Architecture & System Design** (`ARCHITECTURE.md`)](./ARCHITECTURE.md)
   - High-level topology across Server, Workstation Agent, and Desktop App.
   - Backend Go service layer, SQLite & PostgreSQL persistence, and concurrency model.
   - Git worktree isolation engine and skill/MCP provisioning.
   - WebSocket streaming and interactive PTY terminal subsystem.

2. [**Core Capabilities & Workflows** (`CAPABILITIES.md`)](./CAPABILITIES.md)
   - Multi-project management, multi-workstation routing, and checkout detection.
   - Issue tracker abstraction (GitHub, GitLab, Jira Cloud, Local store) and
     read/write credential separation.
   - Five-stage Spec-Driven Development (SDD) skill pipeline (`clarify-issue`,
     `specify-issue`, `implement-issue`, `create-pr` / `adjust-issue`,
     `handoff-issue`), batch execution (`pickup-issues`), and Macro slicing
     (`refine-macro`, `realign-macro`).

3. [**UX Components & Frontend Design** (`UX_COMPONENTS.md`)](./UX_COMPONENTS.md)
   - Web and Desktop layout architecture, navigation, and keyboard shortcuts.
   - Kanban Board (Workflow & Statuses modes), Backlog, Triage, Roadmap
     (Macros), Timeline (Sprints), and Activities center.
   - Task Detail Modal, Desktop Conversation View, and Git Diff Inspector.

4. [**API Reference, Contracts & Data Specifications** (`API_AND_DATA_SPEC.md`)](./API_AND_DATA_SPEC.md)
   - SQLite & PostgreSQL schema, migrations, and encryption model.
   - REST API endpoints, WebSocket protocol, and MCP tool catalog.
   - [**Server–Agent v1 Contract** (`contracts/server-agent-v1.md`)](./contracts/server-agent-v1.md).

5. [**Testing Guide** (`TESTING.md`)](./TESTING.md)
   - Isolated SQLite test fixtures, migration test rules, and Playwright browser
     verification scripts.

6. [**Interactive Architecture Diagram** (`diagrams/sectile-architecture.html`)](./diagrams/sectile-architecture.html)
   - Explorable server/agent/MCP topology with source links and guided views,
     backed by [`diagrams/sectile-architecture.json`](./diagrams/sectile-architecture.json).

7. [**Architecture Decision Records** (`adrs/`)](./adrs)
   - Numbered ADRs documenting durable architectural and product decisions.

8. [**Re-Implementation Guide** (`REIMPLEMENTATION_GUIDE.md`)](./REIMPLEMENTATION_GUIDE.md)
   - End-to-end blueprint and invariants for rebuilding the core system.
