# Tasks #700 - Desktop: sandbox configuration

Order matters: each step builds on the previous one. Tests go with the step
they cover.

## 0. Verify Claude Code

- [x] T0.1 Check the sandbox setting keys of the installed Claude Code
      against its settings schema; adjust the generated keys in `plan.md` if
      they differ.
- [x] T0.2 Capture one `can_use_tool` control request with
      `permission_suggestions` from a real Desktop conversation; save it as
      `internal/agent/testdata/conversation_always_allow.json`.

## 1. Workstation settings

- [x] T1.1 Add `ClaudeSandbox` and `ProjectSettings.ClaudeSandbox`
      (`internal/agentconfig/workstation.go`); include it in `IsZero`.
- [x] T1.2 Normaliser and `AddAllow` (trim, dedupe, keep order, refuse
      empty).
- [x] T1.3 Check the field survives `localProjectRoot` / `WithRepositoryFile`
      and a `WriteSettings` round trip.
- Tests: round trip; emptied section dropped; dedupe; empty entry refused;
  other keys of the file kept.

## 2. Generated settings file

- [x] T2.1 `ClaudeSettingsFile(projectID, sandbox)`: path under the Sectile
      config directory, `0700`/`0600`, atomic write, only set keys, no
      `sandbox` on Windows, `""` and stale file removed when zero.
- Tests: golden JSON for a full value and for rules only; zero value writes
  nothing and removes a stale file; Windows variant (build-tagged test).

## 3. Command lines

- [x] T3.1 `agentCommandContext.ClaudeSettings`; `--settings=<path>` in
      `headlessCommandLine` and in the interactive `claude` branch of
      `modeCommandLine`.
- [x] T3.2 `claudeConversationCommand` gains `settings`; callers pass it.
- [x] T3.3 Launchers generate the file per launch from the resolved project
      settings; a write failure fails the launch with the French message.
- Tests: existing command line tests unchanged without values; with values,
  the argument is present for claude (conversation, interactive, headless)
  and absent for codex, agy and a custom template.

## 4. "Always allow"

- [x] T4.1 Decode suggestions, collect `addRules`/`allow` rules, rewrite every
      destination to `session`.
- [x] T4.2 Persist the rules to the project's `Allow` after the response;
      trace notice in French.
- [x] T4.3 Undecodable suggestion: passed on for the session, logged, nothing
      persisted.
- Tests: from the T0.2 fixture, the response carries `destination:"session"`
  and the settings gain the rule; a second identical rule is not duplicated;
  a `setMode` suggestion persists nothing; "allow" and "deny" unchanged.

## 5. Headless refusals

- [x] T5.1 Read `permission_denials` from the result message; one French
      activity line per refusal, bounded to 10 plus a count.
- Tests: a result with two denials produces two lines; none produces no line;
  twelve produce ten lines and a count.

## 6. Desktop API and UI

- [x] T6.1 `desktopProject` GET returns `claudeSandbox` and
      `platformSandbox`; PUT validates and stores `claudeSandbox`.
- [x] T6.2 "Sandbox" category in `PROJECT_SETTINGS_CATEGORIES`, panel with the
      state, the two sandbox lists, the two rule lists, the hints and the
      both-lists warning; sandbox part disabled when `platformSandbox` is
      false.
- [x] T6.3 `command-preview.mjs`: optional settings path on the built-in
      Claude lines; the Execution preview passes it.
- Tests: Go handler test for GET/PUT and validation; Desktop UI test (build
  first, sandbox off: see memories) for add, remove, duplicate, empty entry,
  save and reopen, and the Windows-disabled state; preview unit test.

## 7. Documentation

- [x] T7.1 `CHANGELOG.md` under `[Unreleased]`: `Added` (project sandbox and
      permission settings in Desktop, applied to every Claude Code launch) and
      `Fixed` ("Always allow" rules no longer lost when the task worktree is
      removed).
- [x] T7.2 Mention the category in the Desktop section of `docs/` if one
      describes the project settings.

## Test plan (manual, before review)

1. Set On, a domain, a deny rule on a test project; start a Desktop
   conversation and ask for a curl to a domain not listed: approval asked or
   refused; to the listed one: allowed.
2. "Always allow" a `Bash(ls:*)` call; hand the task off; start another task:
   no prompt for `ls`; the rule is listed in the category; no
   `.claude/settings.local.json` in either worktree.
3. Headless run hitting the deny rule: the activity names it in French.
4. A project without values: launch lines identical to `main`.
