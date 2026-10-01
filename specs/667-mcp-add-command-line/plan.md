# Plan #667 - Show the `mcp add` command line in the MCP connection settings

Behaviour: `spec.md`. This file says how.

## Stack and boundaries

- Shared ES module `shared/mcpConfig.mjs` (plain JavaScript, typed by
  `shared/mcpConfig.d.mts`), imported by the web (React + TypeScript, Vite)
  and by Sectile Desktop (vanilla DOM in `desktop/src/mcp-settings.mjs`).
- No server, agent, database or migration change. No new dependency.
- Tests: `node --test` for `web/tests/*.test.mjs`, Playwright for
  `web/tests/*.browser.mjs` and the Electron UI tests `desktop/tests/*.ui.cjs`.

## Data contract

```ts
// shared/mcpConfig.d.mts
export function mcpCommand(provider: string, transport: string, server: string, local?: boolean): string | null
export function shellQuote(value: string): string
```

- Same parameters as `mcpSnippet`: `provider` is a key of `mcpProviders`
  (`claude`, `codex`, `agy`) or anything else; `transport` is `'http'` or
  `'stdio'`; `server` is the server URL of the mode (the local proxy URL
  when `local`); `local` drops the header or env-var reference and empties
  the STDIO token.
- Returns the command block, lines joined by `'\n'`, no trailing newline;
  `null` for `agy` and for any provider that is not `claude` or `codex`.
- `base = server.replace(/\/+$/, '')`, `token = '<SECTILE_API_KEY>'`, the
  same derivation as `mcpSnippet`. Keep both in one place (a small internal
  helper or two shared constants) so they cannot drift.

### Construction

Each command is an array of arguments mapped through `shellQuote` and joined
with a single space.

| Provider | Transport | Arguments |
| --- | --- | --- |
| claude | (first line, always) | `claude mcp remove --scope user sectile` |
| claude | http | `claude mcp add --transport http --scope user sectile <base>/mcp` + (`--header`, `Authorization: Bearer <token>`) unless `local` |
| claude | stdio | `claude mcp add --scope user sectile --env SECTILE_AGENT_TOKEN=<local ? '' : token> -- sectile-agent mcp --url <base>` |
| codex | http | `codex mcp add sectile --url <base>/mcp` + (`--bearer-token-env-var`, `SECTILE_API_KEY`) unless `local` |
| codex | stdio | `codex mcp add sectile --env SECTILE_AGENT_TOKEN=<local ? '' : token> -- sectile-agent mcp --url <base>` |

The order is load-bearing: `--env` and `--header` are variadic in
`claude mcp add`, so `--env` placed before the name swallows `sectile`
(`Invalid environment variable format: sectile`), and `--header` must stay
after the URL. The tests pin it.

### Quoting

```js
export function shellQuote(value) {
 return /^[A-Za-z0-9_./:=@%+-]+$/.test(value) ? value : "'" + value.replaceAll("'", "'\\''") + "'"
}
```

An empty string fails the test and yields `''`. Single-quoted strings are
literal in bash, zsh and PowerShell, so the line works in all three for any
value without a `'`; `cmd.exe` is not targeted. `web/src/lib/commandTemplate.ts`
keeps its own private `shellQuote`; merging the two is not part of this
ticket.

## Web: `web/src/components/MCPEngineConfig.tsx`

- Import `mcpCommand` next to `mcpSnippet`; compute
  `const command = mcpCommand(selectedProvider, transport, local ? localUrl : server, local)`
  with the same arguments as `snippet`.
- Under the snippet card, when `command` is not `null`, render a second card
  with the same classes as the snippet card:
  - `<h4>` `text.commandTitle`;
  - `<p>` help: `text.commandClaude` for Claude; for Codex,
    `text.commandCodexEnv` when `transport === 'http' && !local`, else
    `text.commandCodexReplace`;
  - `<pre><code>{command}</code></pre>`, same styling as the snippet;
  - a Copy button `text.copyCommand`, writing `command` to the clipboard and
    setting the notice to `text.commandCopied` or `text.copyError`.
- The custom-provider early return is unchanged and comes before, so no
  command is computed for it.

### Strings: `web/src/locales/mcpConfig.ts`

Add to both locales (wording may be polished, keys stay):

| Key | en | fr |
| --- | --- | --- |
| `commandTitle` | Or register it from a terminal | Ou enregistrez-le depuis un terminal |
| `commandClaude` | Run both lines. The first removes an existing sectile entry; when there is none it prints `No MCP server named "sectile" in user scope`, which is expected, and the second line still runs. | Exécutez les deux lignes. La première retire une entrée sectile existante ; s’il n’y en a pas, elle affiche `No MCP server named "sectile" in user scope`, ce qui est attendu, et la seconde s’exécute quand même. |
| `commandCodexEnv` | Export SECTILE_API_KEY with your personal Sectile API key in the environment that starts Codex. The key is not written to ~/.codex/config.toml. | Exportez SECTILE_API_KEY avec votre clé personnelle Sectile dans l’environnement qui lance Codex. La clé n’est pas écrite dans ~/.codex/config.toml. |
| `commandCodexReplace` | Codex replaces an existing sectile entry. | Codex remplace une entrée sectile existante. |
| `copyCommand` | Copy command | Copier la commande |
| `commandCopied` | Command copied. | Commande copiée. |

The existing `key` paragraph (replace `<SECTILE_API_KEY>`) and the STDIO
binary-path hint keep covering the command too.

## Desktop: `desktop/src/mcp-settings.mjs`

- Import `mcpCommand`.
- In `render()`, after appending the snippet card, compute
  `mcpCommand(provider, mode, local ? info.localURL : info.server, local)`;
  when not `null`, append a second `div.mcp-option.mcp-command` holding an
  `h4` "Or register it from a terminal", a `p.hint` with the English help
  text of the table above (Claude, Codex env-var, Codex replace), a `pre`
  with the command and a button "Copy command" (`disabled = busy`) that calls
  `api.copyText` and sets the notice to "Command copied." or the error
  message.
- The snippet card, its "Copy … example" button and **Update provider
  configuration** are unchanged; the command card adds no write.
- Give the snippet card a class of its own (for example `mcp-snippet`) so
  tests target each `pre` without relying on order.

## Docs

- `desktop/README.md`, section **MCP connections**: one or two sentences
  saying that, for Claude and Codex, the section also shows the
  `claude mcp add` / `codex mcp add` command for the selected mode, an
  alternative to **Update provider configuration** that writes the key
  placeholder (Claude, STDIO) or reads `SECTILE_API_KEY` (Codex remote).
- `CHANGELOG.md`, `[Unreleased]` → `Added`: one line, for example
  "**The MCP connection settings show the command that registers Sectile.**
  On the web and in Desktop, Claude Code and Codex get a ready-to-copy
  `claude mcp add` / `codex mcp add` command for the selected mode, next to
  the configuration snippet. (#667)"

## Target files

| File | Change |
| --- | --- |
| `shared/mcpConfig.mjs` | `mcpCommand`, `shellQuote` |
| `shared/mcpConfig.d.mts` | their declarations |
| `web/src/components/MCPEngineConfig.tsx` | command card |
| `web/src/locales/mcpConfig.ts` | six keys, FR and EN |
| `desktop/src/mcp-settings.mjs` | command card, snippet card class |
| `web/tests/mcpConfig.test.mjs` | generator and quoting tests |
| `web/tests/mcp-engine-config.browser.mjs` (new) | web rendering, both languages |
| `desktop/tests/mcp-settings.ui.cjs` | command card assertions; `pre` selectors scoped to the snippet card |
| `desktop/README.md` | MCP connections |
| `CHANGELOG.md` | `Added` line |

## Risks and notes

- The existing desktop UI test asserts `section.locator('pre')` has count 1
  and reads `pre` without scoping; with a second card those assertions must
  target the snippet card (`.mcp-snippet pre`), or they fail or pass for the
  wrong `pre`.
- Desktop UI tests load `dist/index.html`: run `npx vite build` in `desktop/`
  first, and run them outside the sandbox (Electron keychain access).
- Web browser tests need `PLAYWRIGHT_MODULE` pointing to the main checkout's
  Playwright; worktrees may have no `node_modules`.
- The component renders `MCPApiKeyForm` in Remote mode; the browser test
  stubs `fetch` (or renders under a mocked context) so that form does not
  reach a server.
- The commands depend on the Claude Code and Codex CLI parsers as checked on
  2026-10-01 (clarification rounds 1 and 2). A CLI change shows up in use,
  not in the tests; the verbatim tests at least make the intended form
  explicit.
