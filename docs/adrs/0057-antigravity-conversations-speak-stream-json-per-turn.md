# ADR 0057: Antigravity conversations speak stream-json per turn

- Status: Proposed
- Date: 2026-10-14
- Extends: [ADR 0047](0047-claude-conversations-speak-the-streaming-input-protocol.md), [ADR 0054](0054-codex-conversations-use-app-server.md)

## Context

Desktop conversations already support Claude Code (one process per turn over the
streaming input protocol) and Codex (one `codex app-server` process per
conversation). Antigravity (`agy`), Sectile's default coding engine, still
opened interactive launches and project prompts in a PTY terminal even when
**Settings → General → AI consoles** was set to **Conversation**.

Antigravity's CLI supports structured JSONL output in print mode (`agy
--output-format stream-json --verbose -p`) and session resumption across process
invocations (`--conversation=<uuid>`). Unlike Claude Code, its print mode reads
the prompt from `-p` or stdin until EOF and does not expose a bidirectional
stdio control channel for mid-turn tool approval prompts: unapproved actions are
denied by the CLI and reported in the final `result` frame under
`denied_actions`.

## Decision

- **Run one `agy` process per turn**, resuming the session on subsequent turns
  with `--conversation=<uuid>` captured from the `init` or `result` frame. Model
  (`--model`), reasoning effort (`--effort`), permission mode (`--mode` or
  `--dangerously-skip-permissions`) and project folders (`--add-dir`) are
  resolved afresh on each turn without process-restart machinery.
- **Write the prompt on stdin and close stdin immediately.** `-p` is passed as a
  boolean flag so the prompt is never truncated by argument-length limits and a
  message starting with `/` is not mistaken for `-p=/command` flag syntax,
  except for Antigravity's built-in print-mode slash commands (`/skills`,
  `/mcp`, `/stats`, `/models`), which `agy` only dispatches via `-p=/<cmd>`.
- **Expose Antigravity's permission modes in the composer**: `accept-edits`
  (`--mode=accept-edits`, the default), `default` (no `--mode` flag, following
  the user's configured rules), `auto` (`--dangerously-skip-permissions`) and
  `plan` (`--mode=plan`). When a turn finishes with `denied_actions`, emit a
  notice in the conversation trace naming the denied tool calls and reminding
  the user how to adjust the permission mode.
- **Normalize Antigravity tool steps into the shared conversation event
  contract.** `step_update` frames carrying `thinking_delta` and `text_delta`
  drive the live draft; tool calls (`view_file`, `replace_file_content`,
  `multi_replace_file_content`, `write_to_file`, `run_command`, `grep_search`,
  `find_by_name`, `read_url_content`, `search_web`, `call_mcp_tool`) map to the
  shared `Read`, `Edit`, `MultiEdit`, `Write`, `Bash`, `Grep`, `Glob`,
  `WebFetch`, `WebSearch` and `mcp__<server>__<tool>` card shapes rendered by
  Desktop.
- **Queue follow-up messages sent during an active turn** on the conversation's
  `next` slot so they start the next resumed turn as soon as the current turn
  finishes, because `agy` closes stdin before inference begins.
- **Probe skills and MCP health with hidden one-shot CLI calls** (`agy
  --output-format stream-json -p=/skills` and `agy mcp list`), built through
  `agentexec.Hidden` and `agentexec.StartDetached` so Windows never opens a
  console window.

## Consequences

- Antigravity, Claude Code and Codex all open in the Desktop conversation view
  when **AI consoles** is set to **Conversation**, sharing the same transcript
  store, tool cards, slash-command menu, `!` shell execution and Sectile MCP
  status chip.
- Because `agy` print mode cannot pause mid-turn for interactive approval,
  `accept-edits` is the default mode and any action blocked by policy is
  surfaced explicitly in the trace rather than failing silently.
- Agent-restart recovery remains read-only, as with Claude and Codex.
