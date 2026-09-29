# Plan #614 - Remove the Gemini, Cursor and Vibe AI engines

Behaviour: `spec.md`. This file says where and how.

## Stack

- Local agent and runner: Go (`internal/agentconfig`, `internal/agent`,
  `internal/runner`, `internal/models`).
- Desktop: Electron renderer in plain ES modules (`desktop/src`).
- Web: React and TypeScript (`web/src`), plus the module both clients import
  (`shared/mcpConfig.mjs`).
- No database migration, no server API change, no new dependency.

## Architecture

### One list of supported providers

Today the provider set is spelled out in many `switch` statements and maps.
The removal deletes the `gemini`, `cursor` and `vibe` branches from each, and
names the set of removed providers once, in `internal/agentconfig`, for the
drop on load:

```go
// RetiredProviders are the AI providers Sectile no longer runs (#614). A
// setting naming one is dropped when the settings are read.
var RetiredProviders = map[string]bool{"gemini": true, "cursor": true, "vibe": true}
```

`ValidProvider` then accepts `"", "agy", "codex", "claude", "custom"`. The
refusal message stays `unsupported AI provider %q`, which every save path
already wraps with the engine or level it is about.

### Drop on load

`readConverted` (`internal/agentconfig/settings.go`) already runs
`convertEngines` on every read and reports whether it changed anything;
`MigrateSettings` persists that report once at start, with a backup. The drop
joins the same path, after the conversion so legacy #305 fields are handled
by the same code:

```go
func readConverted(legacyRoot string) (Settings, bool, error) {
	settings, err := readFolded(legacyRoot)
	...
	changed := convertEngines(&settings)
	if dropped := dropRetiredProviders(&settings); !dropped.empty() {
		changed = true
	}
	return settings, changed, nil
}
```

`dropRetiredProviders` (new, in `engines_migration.go` or a new
`retired_providers.go` beside it):

1. removes every catalogue engine whose `provider()` is retired;
2. calls `pruneEngines` so project and task choices naming them go;
3. when `Engines.Default` no longer names an entry, sets it to the first
   remaining entry, or to `addImplicitEngine()` when the catalogue is empty;
4. clears `Seeded.DefaultEngine` when it named a removed engine;
5. deletes the retired keys of `Defaults.AIProviderModels` (nil when emptied)
   and of `MCPConnections` (nil when emptied);
6. clears `Defaults.InitializationProvider` when it is retired;
7. returns what it dropped (engine names and providers, model-list keys, MCP
   keys, the initialization provider) for the log line.

Legacy fields: `convertEngines` creates an engine from a #305 level naming a
retired provider, and step 1 removes it in the same read. The level is then
cleared by `convertEngines` as today, so it resolves to the workstation or
project default. No special case is needed in `resolveLegacyEngine`.

The log line is written where `MigrateSettings` is called at agent start
(`internal/agent/agent.go`, its only caller), from the report
`dropRetiredProviders` returns. To carry that report out, `readConverted`
returns it, and `MigrateSettings` returns it beside its `bool`; the caller
logs it when it is not empty. The log line follows the start-up log lines
around it, which are in English, for example:
`[Agent] Settings for retired AI providers (Gemini, Cursor, Vibe) removed: engines: Gemini [gemini]; model lists: cursor; MCP connections: gemini; initialization provider: vibe`.
`MigrateSettings` keeps its signature for its tests; `MigrateSettingsReport`
returns the report beside it.

A file with nothing to drop and nothing to convert keeps today's behaviour:
no rewrite, no backup.

### Server seed

`ApplyWorkstationSeed` and the project seed (`internal/agentconfig/seed.go`)
compose a legacy engine from the seed. When the composed provider is retired,
the seed creates no engine: the workstation default stays the implicit
engine, and a seeded project gets no engine choice. `NormalizeProviderModels`
(`internal/agentconfig/model.go`) skips retired keys, which covers the seed's
model lists. The desktop save of the execution defaults normalizes before it
validates, so a check inside `ValidProviderModels` would only ever see the
normalized map: a new `ValidProviderKeys` runs in the `PUT /desktop/workstation`
handler before normalization instead, so a desktop save naming a retired
provider fails loudly (US3.2) while a seed is filtered quietly (US4.2).

### Launch and setup paths

Delete the `gemini`, `cursor` and `vibe` branches from:

| File | What |
| --- | --- |
| `internal/agentconfig/validation.go` | `ValidProvider` |
| `internal/agentconfig/engines.go` | `providerNames` |
| `internal/agentconfig/workstation.go` | `DefaultProviderModels` |
| `internal/agentconfig/model.go` | model-flag providers (`case "claude", "codex", "gemini", "cursor"`) |
| `internal/agentconfig/locations.go` | `ResolveLocations`, which makes `init --provider`, the desktop Initialize action, `InitializationProvider` validation and `/desktop/mcp` refuse them |
| `internal/agentconfig/mcp.go` | `httpMCPProviders`, `mcpEntry`, the TOML writer's `vibe` case |
| `internal/agentconfig/mcp_migration.go` | provider-specific branches for `gemini`, `cursor`, `vibe` |
| `internal/agent/agent_desktop_engines.go` | `engineProviders` |
| `internal/agent/agent_console.go` | console launch line |
| `internal/agent/agent_config.go` | autonomous and interactive command lines |
| `internal/agent/init.go` | `--provider` help and error examples |
| `internal/runner/runner.go` | `CheckCliTools` details, autonomous `runAgent` branches, trace launch branches, interactive line and its error message |
| `internal/runner/specframework.go` | Spec Kit and OpenSpec mappings, which then fall to their defaults |
| `internal/models/models.go` | `SupportsAutonomousRun` (`vibe`), provider comments |

Kept on purpose (FR-010): `checkoutMCPFiles` in `projectcontext.go`, the
`.gemini` entries of `models.SkillAgentDirs`, `validation.go`'s skill path
roots and `taskmcp` skill directories, the `agy` locations under `~/.gemini`,
and `desktop/src/editors.mjs`.

`internal/agent/agent_desktop_settings.go` `workstationViewOf` needs no change:
it iterates `DefaultProviderModels` and the stored lists, both of which lose
the retired keys.

### Desktop

| File | Change |
| --- | --- |
| `desktop/src/main.js` | initialization provider select iterates `SETUP_PROVIDERS` order `agy, claude, codex` (or `PROVIDERS` ids) instead of the hard-coded six; `KNOWN_COMMANDS` loses the `gemini` and `vibe` lines |
| `desktop/src/command-preview.mjs` | `MODEL_FLAG_PROVIDERS` and the `gemini`, `cursor`, `vibe` cases |
| `desktop/src/engines.mjs` | `MARKS` loses `gemini`, `cursor`, `vibe` (an unknown provider still gets its first two letters) |

`desktop/src/execution-fields.mjs` already offers AGY, Claude and Codex; the
model lists section is driven by the agent view, so it follows FR-003 with no
renderer change.

### Web and shared

| File | Change |
| --- | --- |
| `web/src/types/index.ts` | `AIProvider = 'agy' \| 'claude' \| 'codex' \| 'custom'` |
| `web/src/lib/commandTemplate.ts` | `MODEL_FLAG_PROVIDERS` and the `gemini`, `cursor`, `vibe` cases |
| `shared/mcpConfig.mjs` | `mcpProviders` loses `cursor`, `gemini`, `vibe`; `mcpSnippet` loses the `gemini` URL field and the `vibe` TOML branch |

`web/src/lib/aiModels.ts` keeps `gemini` in `VENDOR_SEGMENTS`: it parses
model names (Antigravity's `gemini-*`), not providers. The web locale strings
naming Cursor as an MCP desktop client (`directMcpDesc`, `tabCursor`) are
about the Cursor app connecting to `/mcp`, not about an engine; they stay.
`web/src/components/icons/Cursor.tsx` is unused by an engine control and is
left alone.

### Documentation

- `docs/contracts/server-agent-v1.md`: the supported provider list of
  `/desktop/mcp`, and any other provider list in the file.
- `README.md` (line ~98, `init --provider` list), `desktop/README.md` where it
  lists providers, `docs/CAPABILITIES.md` (autonomous launch table).
- `CHANGELOG.md` `[Unreleased]` → `Removed`: one line.
- No ADR: this removes options without a new trade-off; the drop-on-load rule
  reuses the conversion mechanism ADR'd for #510.

## Data contracts

- `~/.config/sectile/settings.json`: no new key. Keys and entries naming a
  retired provider disappear on the first start after the upgrade. Backup
  name follows `MigrateSettings`: `settings.json.bak-layout<N>`, suffixed with
  a timestamp when one exists.
- Desktop API: `PUT` of the engine catalogue and of the execution defaults
  answer 400 with the validation message for a retired provider;
  `GET|POST /desktop/mcp?provider=<retired>` answer 400. The workstation view
  carries no retired key in `providerModels`.
- Server API: unchanged.

## Risks

- **A provider left in one switch.** Mitigated by a test that walks the
  retired set through `ValidProvider`, `ResolveLocations`, `ModelArgs`,
  `UsesHTTPMCP`, the console and autonomous line builders and asserts each
  refuses or ignores it, and by a final `grep` in the tasks.
- **Test fixtures.** About 25 Go test files and 4 desktop test files use
  `gemini`, `cursor` or `vibe` as a convenient second provider. They move to
  `codex` or `agy` unless they test the removal itself. Memory notes apply:
  the `internal/db` suite is close to the CI timeout, and the handlers
  keepalive test is flaky under load.
- **Custom template starting with `gemini`.** A `custom` engine whose command
  runs `gemini ...` keeps working: `custom` is supported and its command is the
  owner's. This is intended.
