# ADR 0003: Local desktop console host

Status: Accepted. Supersedes the UI scope of ADR 0002.

Sectile web owns projects, tasks, workflow, reports, and PR/MR links. It no
longer embeds terminals or exposes local branch, diff, worktree or editor controls.
Skill launches require a connected local agent. Legacy server execution APIs
remain for compatibility; the web does not use the terminal APIs.

An Electron application hosts native interactive consoles through the local
agent's PTY manager. It does not implement a chatbot or provider-specific
conversation protocol. The standalone agent always hosts consoles, supervises one process
group per execution, and serves authenticated loopback console/control endpoints.
Electron's main process holds the credential; its sandboxed renderer uses a
narrow preload IPC bridge. Navigation is blocked. Local control APIs reject
browser Origin headers and require authentication, including WebSocket upgrades.
Repository mappings stay local.

The agent is detached from the window lifecycle. Closing or quitting the app
leaves executions alive; reopening reconnects. A daemon restart loses sessions
and the in-memory run index. Console replay is bounded by the existing PTY
history buffer; users can export visible scrollback.

One binary still provides server, agent, MCP bridge and supervisor commands.
macOS/Linux supervision is supported; Windows remains unsupported. Packages
are unsigned and not notarized, development and release packages alike: the
archives published on every release tag carry an ad-hoc signature on macOS and
none on Windows, and the install guide explains how to open them
([ADR 0034](0034-a-release-is-published-on-both-forges.md)).
