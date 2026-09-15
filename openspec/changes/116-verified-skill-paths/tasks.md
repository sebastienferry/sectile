# Tasks

## 1. Correct the location table
- [x] 1.1 Codex `SkillDir` becomes `.agents/skills`.
- [x] 1.2 Antigravity `SkillDir` becomes `.gemini/config/skills`.
- [x] 1.3 Claude loses `CommandDir`; remove the field if nothing else uses it.
- [x] 1.4 Record the documented source for each path in a comment.

## 2. One installed file per skill
- [x] 2.1 `skillFiles` emits a single `SKILL.md` per skill, carrying `CommandContent` for Claude
      and `Content` for the other agents.
- [x] 2.2 Drop the command destination and the `create-pr` command alias that went with it.
- [x] 2.3 `managedPath` no longer recognises a command path.

## 3. Tests
- [x] 3.1 Assert the corrected table for every supported provider.
- [x] 3.2 Assert Claude installs exactly one file per skill and that it carries `$ARGUMENTS`.
- [x] 3.3 Assert an upgrade from the 113 layout retires `~/.codex/skills`, `~/.agy/skills` and
      `~/.claude/commands`, and populates the corrected directories.
- [x] 3.4 Update the tests that assert the 113 paths.

## 4. Documentation and gates
- [x] 4.1 Update the `README.md` location table and `docs/contracts/server-agent-v1.md`.
- [x] 4.2 Run build, vet, the Go suite and the web gate; validate the change with
      `openspec validate 116-verified-skill-paths --strict`.
