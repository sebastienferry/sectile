# Plan #917 - `@` mentions

## Agent (`internal/agent/mentions.go`)

- `mentionRoots(ctx, projectID, runID)`: the run's directory (or the project's
  local root for a new prompt) labelled `Working folder`, then the attached
  folders (`attachedFolders`), labelled by base name; duplicates dropped.
- `mentionIndex(root)`: `git ls-files --cached --others --exclude-standard -z`
  through `gitLocal`; when it fails, a bounded `filepath.WalkDir` skipping
  `.git` and `node_modules`. Each entry is `Lstat`ed; a symlink whose target
  leaves the root is dropped. Folders are derived from file paths with the
  newest descendant `mtime`. Cached 10 s per root.
- `rankMentionFiles(entries, query, limit)`: tiers base-name prefix, path
  prefix, substring; newest first within a tier.
- `desktopMentionFiles` handler, registered at `/desktop/mentions/files`.
- `resolveMentions(ctx, runID, connectionID, message) string`: parses
  `@[label](sectile:type/id)` and `@path/` tokens; reads run data and
  conversation transcripts under a short lock, tickets through `readAPI`
  outside it; renders `<referenced-context>`.
- `former_context.go`: message selection split out as
  `recentFormerMessages(lines, budget)` so a mention reuses it with 30 000.
- `agent_conversation.go`: resolve before taking the queue lock for a POST
  message containing `@`; `withMentionContext(message, block)` before the
  former-context and shell wrappers in both send paths.

## Tauri

- `src-tauri/src/agent.rs`: `mention_files(project_id, run_id, query)`;
  registered in `lib.rs`.
- `src/lib/mentions.ts`: `mentionFiles()` (returns `[]` on a 404).
- `src/domain/mentions.ts`: `mentionContext`, `mentionToken`, `mentionChips`,
  `splitMentions`, `rankMentionEntities`.
- `src/components/PromptInput.tsx` (renamed from `ConversationInput`): one
  input for both composers with optional commands, history and mentions;
  `/` and `@` menus share `CompletionMenu`.
- `ProjectPrompt.tsx`: the "New prompt" composer uses `PromptInput`;
  `mentionSources` prop from `App.tsx` (conversations, tickets).
- `ConversationTranscript.tsx`: user messages render mention tokens as chips;
  `<referenced-context>` added to the injected trailers.

## Tests

- `internal/agent/mentions_test.go`.
- `tests/mentions.test.ts`, `tests/mentions.ui.test.tsx`; "Free prompt"
  tests moved to the editor helper.
