# Requirements

- A configured project's desktop actions include Open agent console, with Codex and Claude choices.
- Launch starts the selected CLI with no arguments in the mapped repository and preserves its normal interactive permission defaults.
- Each launch has a unique local identity. It has no task, skill, prompt, workflow stage, or PR association.
- Free consoles use authenticated local IPC/HTTP, the existing execution queue, shared-checkout serialization, supervision, stop, replay, and history cleanup.
- Closing the desktop keeps the console alive. Reopening discovers it from the daemon's run list.
- Invalid providers, disconnected projects, missing mappings, and daemon shutdown reject admission. Queue cancellation and launch failure release capacity.
- Task-based execution behavior remains intact. Free consoles display process status without requesting task metadata or posting workflow completion.
