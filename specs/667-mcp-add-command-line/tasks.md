# Tasks #667 - Show the `mcp add` command line in the MCP connection settings

Ordered checklist. Each group is one commit and leaves the tree buildable and
the tests green.

## 1. Shared generator (FR1-FR8, US5)

- [ ] T1.1 `shellQuote(value)` in `shared/mcpConfig.mjs`, exported.
- [ ] T1.2 `mcpCommand(provider, transport, server, local = false)` in
  `shared/mcpConfig.mjs`, sharing the `base` and token derivation with
  `mcpSnippet`; returns `null` for any provider other than `claude` and
  `codex`.
- [ ] T1.3 Declarations in `shared/mcpConfig.d.mts`.
- [ ] T1.4 Tests in `web/tests/mcpConfig.test.mjs`:
  - the six commands of spec US1.1-3 and US2.1-3, verbatim, on
    `https://sectile.example` (with a trailing `/` to prove the trim) and
    `http://127.0.0.1:8091`;
  - the Claude block is exactly two lines, `remove` first, no `&&`; the Codex
    block is one line with no `remove`;
  - Claude STDIO: `sectile` appears before `--env`; Claude HTTP: `--header`
    after the URL;
  - local STDIO gives `SECTILE_AGENT_TOKEN=` bare, for both providers;
  - local HTTP carries no `--header` and no `--bearer-token-env-var`;
  - `shellQuote`: a plain URL stays bare; `https://ex.test/a b$c'd` becomes
    `'https://ex.test/a b$c'\''d'`; `''` for an empty string; the placeholder
    argument is quoted;
  - `mcpCommand('agy', …)` and `mcpCommand('custom', …)` return `null`;
  - the existing snippet tests are untouched and pass.

## 2. Web tab (FR9, FR10, US3, US4)

- [ ] T2.1 Six keys in both locales of `web/src/locales/mcpConfig.ts`
  (`commandTitle`, `commandClaude`, `commandCodexEnv`,
  `commandCodexReplace`, `copyCommand`, `commandCopied`).
- [ ] T2.2 Command card in `web/src/components/MCPEngineConfig.tsx`, shown
  only when `mcpCommand` is not `null`, with its own Copy button and notice.
- [ ] T2.3 `web/tests/mcp-engine-config.browser.mjs` (new, Vite + Playwright
  harness like the other `*.browser.mjs`): Claude remote shows the two-line
  block and the Claude help; switching to STDIO updates it; Codex remote
  shows the env-var help; Antigravity shows no command; the command Copy
  button puts the block on the clipboard (stubbed `navigator.clipboard`) and
  the status reads the command notice; English then French strings (AC3).
- [ ] T2.4 `tsc` and `oxlint` on `web/` pass.

## 3. Desktop section (FR9, FR10, US3, US4.4)

- [ ] T3.1 Class `mcp-snippet` on the snippet card in
  `desktop/src/mcp-settings.mjs`.
- [ ] T3.2 Command card (`mcp-option mcp-command`) with heading, help, `pre`
  and "Copy command" button, after the snippet card, when `mcpCommand` is not
  `null`.
- [ ] T3.3 `desktop/tests/mcp-settings.ui.cjs`: scope the existing `pre`
  assertions to `.mcp-snippet`; assert no `.mcp-command` for `agy` (the
  initial provider); for `codex` local, the command reads
  `codex mcp add sectile --url http://127.0.0.1:4567/mcp`; for `codex` STDIO
  and remote, the command follows the mode; select `claude` and assert the
  two-line block; click "Copy command" and assert the status; the three
  **Update provider configuration** writes stay as they are.
- [ ] T3.4 `npx vite build` in `desktop/`, then `npm run test:ui` outside the
  sandbox.

## 4. Documentation (FR11, FR12)

- [ ] T4.1 `desktop/README.md`, **MCP connections**: the command as the
  alternative to **Update provider configuration**.
- [ ] T4.2 `CHANGELOG.md`, `[Unreleased]` → `Added`: one line (#667).

## Test plan

| Requirement | Covered by |
| --- | --- |
| FR1, FR2, FR3, FR4, FR5, FR6 | T1.4 verbatim commands |
| FR7, US5 | T1.4 quoting cases |
| FR8, US4.2 | T1.4 `null`; T2.3, T3.3 hidden for Antigravity |
| FR9, US3 | T2.3, T3.3 copy button and notice |
| FR10, AC3 | T2.3 both languages |
| US4.1 | T2.3, T3.3 mode and provider switches |
| US4.4 | T3.3 apply still writes only through its button |
| FR11, FR12, AC5 | T4.1, T4.2, review |
| AC4 | existing tests of `web/tests/mcpConfig.test.mjs` pass unchanged |

Manual check, once: paste the Claude STDIO block and the Codex remote line
into a terminal with a throwaway `HOME` / `CODEX_HOME` and read back
`claude mcp get sectile` and `~/.codex/config.toml`.
