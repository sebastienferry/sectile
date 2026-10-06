# Specification #667 - Show the `mcp add` command line in the MCP connection settings

- Ticket: https://github.com/sebastienferry/sectile/issues/667
- Branch: `feat/667`
- Clarification: `docs/clarifications/667.md` (round 1, round 2 with the
  owner's answers; no open product question)
- Framework: Spec Kit

## Summary

The MCP connection tab, on the web and in Sectile Desktop, shows a
ready-to-run command line that registers Sectile in the selected AI engine,
for the selected mode, next to the configuration snippet it already shows.
The command has its own Copy button. It exists for Claude Code
(`claude mcp add`) and for Codex (`codex mcp add`); Antigravity, which has no
CLI, and custom providers show none.

The ticket title names only `claude mcp add`: the clarification extended the
scope to `codex mcp add` (round 2, Q2). The title is left to the owner.

## Scope

In scope: the web MCP connection tab, the desktop MCP configuration section,
the shared generator both use, their French and English strings (web) and
English strings (desktop), their tests, `desktop/README.md` and the changelog.

Out of scope:

- The configuration snippet itself (`mcpSnippet`): its content and its Copy
  button stay as they are.
- The agent-side writer (`internal/agentconfig/mcp.go`) and the desktop
  **Update provider configuration** action, which stays the primary action.
- The custom-provider fallback of the web tab.
- A command for Antigravity.
- A scope choice for Claude (`project`, `local`): the scope is fixed to `user`.
- `cmd.exe` as a target shell.

## Vocabulary

- **Mode**: the connection the tab is set to: **Remote HTTP** (the server
  over Streamable HTTP with the personal API key), **Local HTTP proxy** (the
  local agent's no-auth proxy) or **STDIO** (the `sectile-agent mcp` bridge
  started by the engine).
- **Server URL**: the URL the snippet uses for the mode, trailing slashes
  removed: the Sectile server for Remote HTTP and STDIO, the local proxy URL
  for Local HTTP proxy.
- **Key placeholder**: the literal `<SECTILE_API_KEY>` the snippet already
  shows in place of the personal key.
- **Command block**: the text the tab shows and copies under the snippet,
  one or two command lines separated by a line break.

## User stories

### US1 (P1) - A Claude Code user registers Sectile with one paste

As a person setting up Sectile in Claude Code, I copy the command block for
my mode and paste it into a terminal instead of editing `~/.claude.json`.

1. Given the provider is Claude and the mode is Remote HTTP on server
   `https://sectile.example`, when the tab is shown, then the command block
   reads exactly:

   ```
   claude mcp remove --scope user sectile
   claude mcp add --transport http --scope user sectile https://sectile.example/mcp --header 'Authorization: Bearer <SECTILE_API_KEY>'
   ```

2. Given the provider is Claude and the mode is Local HTTP proxy on
   `http://127.0.0.1:8091`, then the command block reads exactly:

   ```
   claude mcp remove --scope user sectile
   claude mcp add --transport http --scope user sectile http://127.0.0.1:8091/mcp
   ```

3. Given the provider is Claude and the mode is STDIO on server
   `https://sectile.example`, then the command block reads exactly:

   ```
   claude mcp remove --scope user sectile
   claude mcp add --scope user sectile --env 'SECTILE_AGENT_TOKEN=<SECTILE_API_KEY>' -- sectile-agent mcp --url https://sectile.example
   ```

4. Given the command block is shown for Claude, then the help text under it
   says to run both lines, and that the message
   `No MCP server named "sectile" in user scope` printed by the first line is
   expected when Sectile was not registered yet and does not prevent the
   second line from running.
5. Given Sectile is already registered in Claude Code's user configuration
   (by desktop pairing, for example), when the person pastes the block, then
   the existing entry is replaced by the one of the selected mode.

### US2 (P1) - A Codex user registers Sectile with one paste

As a person setting up Sectile in Codex, I copy one command line instead of
editing `~/.codex/config.toml`.

1. Given the provider is Codex and the mode is Remote HTTP on server
   `https://sectile.example`, then the command block reads exactly:

   ```
   codex mcp add sectile --url https://sectile.example/mcp --bearer-token-env-var SECTILE_API_KEY
   ```

2. Given the provider is Codex and the mode is Local HTTP proxy on
   `http://127.0.0.1:8091`, then the command block reads exactly:

   ```
   codex mcp add sectile --url http://127.0.0.1:8091/mcp
   ```

3. Given the provider is Codex and the mode is STDIO on server
   `https://sectile.example`, then the command block reads exactly:

   ```
   codex mcp add sectile --env 'SECTILE_AGENT_TOKEN=<SECTILE_API_KEY>' -- sectile-agent mcp --url https://sectile.example
   ```

4. Given the provider is Codex and the mode is Remote HTTP, then the help
   text under the command says to export `SECTILE_API_KEY`, set to the
   personal API key, in the environment that starts Codex, and that the key
   is not written to `~/.codex/config.toml`.
5. Given the Codex block, then it holds no `remove` line: `codex mcp add`
   replaces an existing `sectile` entry.

### US3 (P1) - The command is copied on its own

1. Given a command block is shown, then it has a Copy button of its own,
   distinct from the snippet's.
2. When the person clicks it, then the clipboard holds the command block
   exactly as displayed (both lines for Claude), and the tab's status line
   confirms the copy; when the clipboard refuses, the status line says so,
   as it does for the snippet.
3. Given the snippet's Copy button, then it still copies the snippet only.

### US4 (P2) - The command follows the tab

1. When the person switches mode, edits the local proxy URL (web), or
   changes provider, then the command block is regenerated at once from the
   same inputs as the snippet.
2. Given the provider is Antigravity, then no command block, no command help
   text and no command Copy button are shown; the snippet is unchanged.
3. Given a custom provider, then the tab shows its existing manual fallback
   and no command block.
4. Given Sectile Desktop, then **Update provider configuration** stays the
   primary action of the section; the command block is shown as an
   alternative, under the snippet, and copying it writes nothing on disk.

### US5 (P2) - Unusual values still produce a valid command

1. Given a server URL that contains a character outside
   `A-Z a-z 0-9 _ . / : = @ % + -` (a space, `$`, `&`, `"`, `'`...), when
   the command is generated, then that argument is enclosed in POSIX single
   quotes, a `'` inside it written as `'\''`, so a POSIX shell receives it as
   one argument with its exact value.
2. Given an argument made only of the characters above, then it is written
   bare, so a plain URL stays readable.
3. Given the key placeholder, then the argument carrying it is always quoted,
   since `<` and `>` are shell redirections.
4. Given an empty argument, then it is written `''`.

## Functional requirements

- **FR1** One shared generator produces the command block for both clients,
  from the same provider, transport, server URL and local flag as the
  snippet, so the web and Desktop always show the same text for the same
  inputs.
- **FR2** It produces, for Claude and Codex and each mode, exactly the
  commands of US1 and US2, with the argument order shown there: in the
  Claude STDIO form the server name `sectile` precedes `--env`; in the Claude
  HTTP form `--header` follows the URL; in both Codex forms the name follows
  `add`.
- **FR3** The Claude block is two lines, `remove` then `add`, joined by a
  line break and no `&&`. The Codex block is one line.
- **FR4** The Claude scope is always `user`. Codex takes no scope option.
- **FR5** Remote HTTP carries the key placeholder in the Claude
  `Authorization` header and refers to the `SECTILE_API_KEY` environment
  variable in Codex. Local HTTP proxy carries neither a header nor a token.
  STDIO passes `SECTILE_AGENT_TOKEN=<SECTILE_API_KEY>` through `--env`, or
  `SECTILE_AGENT_TOKEN=` (empty) when the local flag is set, as the snippet
  does.
- **FR6** The STDIO executable is the bare `sectile-agent`, as in the
  snippet; the existing hint about the installed binary path applies to the
  command too.
- **FR7** Arguments are quoted as US5 states.
- **FR8** For Antigravity, a custom provider or an unknown provider, the
  generator returns no command and both clients show no command block.
- **FR9** Each client shows the command block under the snippet, with a
  heading, a provider-dependent help text (US1.4, US2.4, and for the other
  Codex modes a line saying an existing entry is replaced), and its own Copy
  button (US3).
- **FR10** Web strings exist in French and English in the web MCP strings;
  Desktop strings are English, like the rest of its MCP section.
- **FR11** `desktop/README.md` (MCP connections) mentions the command as the
  alternative to **Update provider configuration**.
- **FR12** `CHANGELOG.md` gets one `Added` line under `[Unreleased]`.

## Acceptance criteria

- **AC1** Unit tests assert the six commands of US1.1-3 and US2.1-3 verbatim,
  the two-line Claude block, the Codex env-var form, the local STDIO empty
  token, the quoting rules of US5 on a URL holding a space, `$` and `'`, and
  no command for Antigravity and an unknown provider.
- **AC2** The desktop UI test shows the command block for Claude and Codex,
  hides it for Antigravity, follows a mode switch, copies the command
  through its own button, and still applies **Update provider
  configuration** as before.
- **AC3** On the web, with the language set to English then French, the
  command heading, help texts and Copy button appear in that language.
- **AC4** The existing snippet tests pass unchanged.
- **AC5** `CHANGELOG.md` and `desktop/README.md` are updated (FR11, FR12).

## Open points

None. Every product question of the clarification has an answer.
