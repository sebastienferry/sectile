# ADR 0053: Explicit Claude sandbox autonomy and persistent directory approvals

- Status: Accepted
- Date: 2026-10-06
- Amends: [ADR 0048](0048-claude-sandbox-values-reach-claude-through-a-generated-settings-file.md) and [ADR 0050](0050-claude-sandbox-values-have-a-workstation-level.md)

## Context

Command allow lists require repeated exceptions, and inherited Claude sandbox policy can retain permission prompts. Directory suggestions accepted with Always allow previously lasted only for the running turn, unlike tool rules persisted by Sectile.

## Decision

Expose optional `autoAllowBashIfSandboxed` and `allowUnsandboxedCommands` values at the workstation and project levels. Unset values preserve inheritance; explicit false is retained. The opt-in Autonomy in sandbox action enables the sandbox, enables automatic sandboxed command approval and disables unsandboxed retries without changing existing allow or deny rules. Project scalar values override workstation values.

Persist explicitly approved `addDirectories` suggestions as `permissions.additionalDirectories` in project workstation settings. Normalize and deduplicate directory lists, merge concurrent additions during dialog saves, and apply the union of workstation and project directories. Keep Claude's suggestion destinations session-scoped so worktrees gain no settings files. Permission modes themselves remain unchanged.

Show Claude's decision reason, when provided, and the proposed access in the approval card. Native Windows receives directory permissions but no sandbox policy.

## Consequences

Routine commands inside the configured sandbox need fewer approvals; caches and network domains still require configuration. Commands that need unsandboxed execution fail under this profile. File, web and MCP tools remain governed by their own permissions. Existing configurations do not automatically adopt the profile. A subsequent message or launch receives saved values; no running process is reconfigured.
