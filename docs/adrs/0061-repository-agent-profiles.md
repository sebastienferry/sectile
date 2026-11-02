# ADR 0061: Repository-owned agent profiles

- Status: Accepted
- Date: 2026-10-27

## Context

Desktop project sessions need reusable instructions separate from workstation
provider/model configurations. Provider-native agent registries differ in their
format and semantics, while project instructions should remain reviewable and
portable through the repository's ordinary workflow.

## Decision

Store flat UTF-8 Markdown prompts at `.sectile/agents/<id>.md` in the mapped
primary checkout. The filename supplies the name and ID. Limit prompts to
64 KiB and validate IDs and contents on discovery, creation, and admission.
The local authenticated Desktop API accepts IDs, never renderer-supplied paths.
Creation publishes a complete temporary file through an exclusive hard link;
concurrent creation cannot overwrite an existing profile. Directory handles and
file identities pin filesystem access; symbolic links and path substitutions
inside the checkout are rejected before reading contents. The mapped checkout
path itself is the user's choice, so links at or above it are resolved once. This requires a filesystem supporting
hard links; unsupported publication fails without exposing a partial profile.

Snapshot selected instructions in the local controlled run. Keep public profile
metadata separate from engine metadata. Claude uses additive system instructions;
Codex combines effective native developer instructions with the profile and
carries that composition through thread start/resume and collaboration turns.
Keep ordinary repository instruction discovery. Reject custom templates and
unattested provider interfaces instead of silently dropping instructions.

Do not persist prompt snapshots in read-only history: the current run store
cannot resume a conversation after daemon restart. Active process reconnects
reuse the in-memory snapshot. Persist profile identity with the ordinary run
metadata, and do not send profile text to Sectile's server.

## Consequences

Profiles are engine-independent and shared through ordinary repository review.
There is no database catalogue, default-profile setting, automatic commit, or
provider-native registry synchronization. Users edit/delete files through their
normal tools. A checkout mapped through a symbolic link, such as `/tmp` on
macOS, works without being remapped to its canonical path.
Supporting another provider requires proving additive instruction delivery for
its console and conversation protocols first. Native Codex instructions are
snapshotted at admission, so later config changes affect new sessions only.
