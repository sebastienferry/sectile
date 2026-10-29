# Companion MCP verification

Verified on 2026-10-29 against the standalone desktop and its bundled Go agent.
See ADR 0011 for the connection boundary.

## Real provider connections

- Codex reported the isolated `sectile_companion` MCP connected. A real
  `list_projects` call initially exposed unsupported MCP form consent. After the
  handler fix, the desktop showed allow-once/refuse, and allow-once completed the
  same read with `COMPANION_MCP_OK`. No persistent permission was granted.
- Claude ran the workflow below using the isolated bridge. Its MCP operations
  read project/task context and submitted real run reports and stage transitions
  to the companion server.
- Electron's shared Claude registration remained unchanged. `claude mcp get
  sectile` reported connected. An authenticated MCP initialize and read-only
  `list_projects` call through that registration also succeeded, using the normal
  system certificate trust rather than disabling TLS verification.
- Digests of both shared `sectile` registrations in `.claude.json` and Codex's
  `config.toml` remained unchanged. The legacy shared Codex entry already points
  to a missing executable at this initial verification. The separately authorized
  repair below fixes its path without redirecting it to the companion.

## Desktop workflow

Project: `Test workflow MCP compagnon` (`3a4e48e5-e4fb-4572-96a9-31a3cbec1164`).
Successful final run: `c7f48526-44e1-4c24-a674-ee4c1dfa8c4f`.

The project uses a disposable GitLab HTTP fixture, its own ticket, a real bare Git
repository and an assigned worktree. Its project-only pickup skill exercises the
four stages without installing or changing global skills. Every run was launched
using the desktop's **Lancer la chaîne · jusqu’à Revu** control.

1. Clarification produced and committed the requirements report. The server
   accepted `clarified` and the fixture received its stage label and report.
2. Specification produced and committed the OpenSpec proposal, design, task list
   and greeting specification. The server accepted `specified`.
3. Implementation created `greeting.py` and its unittest, passed the test and
   committed/pushed `feat/1`. The real forge CLI verified the fixture's matching
   draft MR before the server accepted `implemented`.
4. Review inspected the code, reran the unittest and made that same MR ready. The
   server accepted `reviewed`; the desktop displayed **Skill terminée · résultat
   confirmé** and proposed Finalize as the next action.

The first attempts stopped at specified because the installed forge CLI did not
recognize the loopback fixture. Recovery retained the accepted stages, commits and
existing MR. Repository-local `glab` configuration completed the fixture's HTTP
integration; the successful run used the installed CLI and normal agent PATH.
A temporary fixture CLI wrapper was ineffective and was not used for the
successful validation. No production validation was relaxed to accept the test.

Final independent checks: one passing unittest, clean worktree, three stage
commits, pushed branch SHA matching HEAD, one ready/open MR, remote `#reviewed`
label and completed server activity. The issue remained open; no merge or
finalization was performed. No real tracker tickets were modified.

The fixture verifies real desktop/provider/Go/MCP/Git behavior against a test
tracker, not production GitLab authorization or hosting behavior.

## Shared Codex registration repair

On 2026-10-29, after explicit user authorization, the stale shared Codex
registration was repaired. Only `mcp_servers.sectile.command` changed, from the
absent source-checkout executable to Electron's installed bridge at
`/Applications/Sectile.app/Contents/Resources/sectile-agent` (v0.7.5). The server,
pairing key, arguments, enabled state and every other Codex setting were retained.
The configuration was replaced atomically with owner-only permissions.

Before the write, the candidate bridge completed MCP initialization and a real
`list_projects` call using the existing registration's identity. After the write:

- The saved shared registration completed the same real call over STDIO.
- Tauri's separately scoped bridge completed its own real call.
- Electron's unchanged Claude HTTP registration completed a real read as well.
- Two fresh ephemeral Codex app-server sessions independently called
  `mcpServer/tool/call` for `list_projects`: one using the shared `sectile`
  configuration, the other using Tauri's `sectile_companion` process overrides.
  Only the latter returned the disposable local workflow project, demonstrating
  that the two connections still reach different servers. No model prompt,
  tracker mutation or permanent approval was used for these checks.
- Digests confirmed Electron's Claude MCP entry, Tauri's installation and Tauri's
  agent settings stayed unchanged. The only shared Codex edit was the bridge
  command path. Existing provider processes must reload configuration to acquire
  the corrected path; no running user conversation was restarted for this repair.
