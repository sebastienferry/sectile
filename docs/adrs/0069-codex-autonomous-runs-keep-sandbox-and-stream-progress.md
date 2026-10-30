# ADR 0069: Codex autonomous runs keep the sandbox and stream progress

Status: Accepted
Date: 2026-10-30

## Context

A real standard pickup chain on a disposable Fleet installation reached the
Codex CLI but stopped before clarification. The built-in `codex exec` command
inherited a read-only sandbox and could not answer MCP approval requests. The
CLI returned zero after explaining the refusal; the server correctly kept the
task at New, but an accepted launch was not a completed workflow. Codex also
had no live trace flag, so desktop clients could not attach to its progress.

## Decision

The built-in autonomous Codex command uses `--approve-for-me --json`. The first
option retains the workspace-write sandbox and routes permission requests to
Codex's automatic reviewer. It does not disable the sandbox, ignore rules or
approve every tool. Denials remain visible and authoritative. Interactive
conversations, user-authored templates, Claude and standalone registration
settings retain their own existing behavior.

Codex JSONL is normalized into the existing local trace protocol. Commands,
file changes and scoped MCP calls carry progress; assistant messages and errors
also reach the server activity. MCP arguments keep the existing size limit,
and unknown or truncated frames do not become answers. No new database,
renderer protocol, global CLI configuration or dependency is introduced.

The default requires a CLI with these attested options, verified using installed
Codex CLI 0.165.0. Older binaries rejecting them must be upgraded; silently
falling back to a read-only or unrestricted invocation would hide the problem.

## Verification

Regression tests cover both built-in command lines, unchanged interactive and
custom commands, model/folder forwarding, stream detection with quoted prompts,
Codex commands/MCP/file changes, answers, failures and argument limits. A local
trace routing test separates tool progress from server activity protocol noise.
Live workflow qualification and its exact scope are recorded in the Tauri V1
audit; a process exit alone is never evidence of a stage transition.

## References

- [Codex non-interactive mode](https://learn.chatgpt.com/docs/non-interactive-mode): sandboxing, automatic review and JSON Lines output.
- [Configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference): server-scoped MCP policy remains separate from CLI sandbox settings.
