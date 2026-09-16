# Sectile Architecture & Engineering Documentation

Welcome to the comprehensive technical documentation for **Sectile** (formerly Sectile, the Autonomous AI Task & Project Orchestrator). This documentation is designed to provide complete architectural clarity, component blueprints, data specifications, and step-by-step instructions so that any software engineer or Large Language Model (LLM) can understand, maintain, extend, or completely re-implement this system from scratch.

---

## 📚 Documentation Index

1. [**Architecture & System Design** (`ARCHITECTURE.md`)](./ARCHITECTURE.md)
   - High-level architecture, design philosophy, and core invariants.
   - Backend Go service layer, SQLite persistence, and concurrency model.
   - Git Worktree isolation engine and automatic dependency/skill propagation.
   - WebSocket streaming and Interactive PTY pseudo-terminal subsystem.

2. [**Core Capabilities & Workflows** (`CAPABILITIES.md`)](./CAPABILITIES.md)
   - Multi-project management and dynamic CWD resolution.
   - Issue tracker abstraction (GitHub Issues, Jira, Local SQLite).
   - Autonomous AI Skill pipeline (`clarify-issue`, `specify-issue`, `code-issue`, `adjust-issue`, `pick-issue`).
   - Interactive live CLI terminal with persistent ZSH sessions.
   - Live Git branch switching, diff inspection, and pull request generation.

3. [**UX Components & Frontend Design** (`UX_COMPONENTS.md`)](./UX_COMPONENTS.md)
   - Layout architecture, navigation bar, and project filter bar.
   - Kanban Board with fluid Drag & Drop (`@dnd-kit`).
   - Tabular List View with column sorting, filtering, and grouping.
   - Task Detail Modal (sliding drawer / modal dialog with Speckit integration).
   - Task Chat Drawer with dual mode: Rich Assistant Chat & Interactive Xterm.js PTY Terminal.
   - Git Diff Inspector modal with side-by-side / inline syntax highlighting.
   - Activity Center & real-time background queue monitor.

4. [**API Reference & Data Specifications** (`API_AND_DATA_SPEC.md`)](./API_AND_DATA_SPEC.md)
   - Complete SQLite Database schema, tables, indexes, and migrations.
   - RESTful API specification (endpoints, request/response models).
   - WebSocket protocol specification for live terminal streaming and window resizing.
   - Task workflow lifecycle and activity status state machines.

5. [**Re-Implementation Guide for LLMs** (`REIMPLEMENTATION_GUIDE.md`)](./REIMPLEMENTATION_GUIDE.md)
   - Step-by-step guide to rebuild Sectile from zero.
   - Critical implementation pitfalls, concurrency rules, and deadlock prevention.
   - Verification checklist and test suite specification.
