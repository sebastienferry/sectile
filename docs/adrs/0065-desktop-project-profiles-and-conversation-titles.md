# ADR 0065: Desktop project profiles and conversation titles

- Status: Accepted
- Date: 2026-10-28

## Context

The standalone desktop needs reusable provider, model and mode choices for new
project prompts, and durable names for free conversations. These are local
preferences; tracker tickets remain authoritative for task names.

## Decision

Reuse the workstation engine catalogue and its project selections. The authenticated
`/desktop/conversation-profiles?projectId=...` GET/POST surface exposes only built-in
Claude Code and Codex profiles without command templates. Save, select and delete
operate under the existing settings lock and validation. An engine's optional
`conversationMode` is `plan` or `execute`; it affects new conversations only.
Settings layout 7 prevents older agents from silently dropping this field.
Profiles are shared across the workstation. Project defaults are independent;
profiles used by other projects, tasks or the workstation default cannot be deleted
through this simplified editor. Existing skill-model overrides remain intact.

Free conversations acquire an automatic title from their first message. The
`/desktop/conversation/title` POST surface accepts a bounded single-line title
and writes it through the existing synchronous conversation journal, without
sending a provider message. Read-only free histories may be renamed through the
run store. Task conversations cannot be renamed through this endpoint. Legacy
histories derive a title from the first durable user event on restore.

The native IPC bridge exposes fixed routes and a typed profile-action enum.
The React UI edits project profiles, preloads the default in a new prompt, and
continues to permit per-message model and mode choices. Late catalogue reads do
not overwrite choices already made by the user. Existing conversations retain
provider context, history and settings when a profile changes.

## Consequences

There is no second profile database or additional JavaScript dependency.
A profile edit affects future launches in every project using it; the UI explains
this scope. Project access depends on the existing local server configuration API.
Titles persist with local run histories and never update remote tracker titles.

## Validation

The desktop UI suite covers default preselection, late-loading preferences, profile
saving without execution, custom model editing and renaming read-only histories.
Go tests cover authenticated profile actions, protected catalogue entries, preserving
an implicit workstation default, launch settings independent of later edits, manual
provider switches, Unicode title bounds and title recovery in writable/read-only
histories.

Real companion checks used the disposable local project: Codex `gpt-6-luna` and
Claude Code `haiku`, both in Plan mode, returned the requested marker without tools.
After naming both conversations and gracefully restarting the companion, project
profiles, titles, model/mode and all prior events survived; only pause notices were
added. No provider message or tool was replayed.
