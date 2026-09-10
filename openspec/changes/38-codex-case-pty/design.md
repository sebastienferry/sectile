## Context
`SkillCallLineWithCommand` forces a slash. Both `InjectSkillInTTY` and `runSkillThroughAgent` use it. `ttyContext` already resolves project settings over global settings. `ProjectSkillCommand` supplies the configured name. The terminal displays the shared skill command without adapting it.

## Decisions
Add a provider-aware formatter, retaining the existing formatter as a compatibility wrapper. Normalize provider whitespace and case. Strip one leading slash from the configured command for Codex; retain the existing normalization for other providers. Pass resolved settings through both PTY entry points. Do not mutate persisted command names or headless prompts.

Use the same project-over-global precedence and skill override in the terminal action labels. Add a small pure frontend formatter so its behavior can be checked without mounting xterm. Add Codex to the existing interactive launcher allowlist, using existing binary resolution.

## Alternatives
UI-only replacement would leave the injected text broken. Removing slashes globally would regress other engines. Rewriting stored skill commands would unnecessarily change headless execution and require data handling.

## Risks and verification
A session launched before its provider settings change may not match the configured provider; live provider switching is outside scope. Cover both PTY call sites with a fake terminal, provider fallback/override, renamed skills, preserved ticket context, and launcher resolution. Verify frontend labels independently, then run build, lint and Go tests. No live Codex invocation is required in automated tests.

## Validation finding
Fresh databases lack `settings.jira_api_token`, although settings reads and writes already require it. Add the missing column with the existing empty-string default migration pattern. This is needed for `ttyContext` to resolve settings at all; the PTY integration tests exercise fresh database settings reads and writes.
