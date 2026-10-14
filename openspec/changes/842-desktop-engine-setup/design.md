# Technical design

## Existing integration points
- desktop/src/main.js: deploymentPanel, openSettings, workstation permission controls and project framework deployment.
- desktop/src/engines.mjs: catalogue/provider identity; Antigravity uses provider key agy.
- desktop/src/mcp-settings.mjs and shared/mcpConfig.mjs: supported providers, remote/local HTTP and advanced STDIO, redacted previews and explicit Claude repair.
- desktop/src/sandbox-settings.mjs: existing Claude sandbox controls and Windows limitations.
- desktop/electron/preload.cjs and main.cjs: engines, project, deployProject, mcpConfig/configureMCP and provider permission IPC.
- internal/agent/agent_desktop.go: provider-skills and framework operations, concurrency restrictions and structured initialization outcomes.
- internal/agent/agent_mcp_settings.go and init.go: existing configuration writers and skills deployment.

## Assistant architecture
Add a dedicated renderer module and pure state reducer rather than grow deploymentPanel into a second writer. General exposes the entry; existing advanced panels remain available. Steps: engine selection, optional skills, MCP, permissions, optional SDD, summary. Selecting an engine resolves a provider; disclose user-wide effects. Explicitly confirm a skills-source project and separately an SDD target; a preselected project is not authorization to deploy.

Use existing action APIs for all writes. Await structured step status: an HTTP-success response containing failed is a failed step. Disable duplicate submissions; capture provider/project identity for each operation and discard stale read responses using a generation token. Navigation/cancellation does not undo accepted writes or mark pending steps complete. Reopening reads current state and keeps verified successes and explicit skips.

## State and readback contract
Introduce an additive authenticated local-agent setup-state capability and Electron IPC, using the existing workstation settings persistence and locking patterns. No credentials cross to the renderer. Old agents return an unavailable capability state with update guidance; no fallback assumes incomplete or success.

The read view contains schemaVersion, normalized paired-server identity, provider records and project records. Each step has status unknown|pending|running|success|skipped|failed, a safe explanation, and applicable scope. MCP additionally returns target remote|local, transport http|stdio, configuration present/valid/needsRepair and a safe configuration fingerprint. Permission review records a fingerprint of the reviewed effective provider controls. Skills records source project identity and installed file manifest/fingerprint or explicit skip. SDD records project ID, configured local root and framework plus observed installation manifest/fingerprint or skip. Store durable outcomes only after successful readback; running is transient and becomes pending after interruption. Never store pairing keys, raw provider files or credential-bearing hashes in this metadata.

An acknowledgement write accepts provider identity, step, decision retained|skipped, and expected fingerprint; reject stale scope or state and return the refreshed view. Retain applies only to permission review; skips apply only to skills/SDD. Existing action results trigger a refresh, not arbitrary renderer declarations of success. Backend reads actual Sectile-owned skill/SDD assets and MCP configuration to validate saved outcomes. Reuse installer manifests where available; otherwise record paths and content digests of the assets installed by Sectile, without scanning unrelated plugins. Skipped skills need no plugin detection. No inference of live connectivity is made.

Provider completion requires verified MCP, reviewed current permissions, resolved optional skills and resolved optional SDD decision. An SDD install outcome is scoped to its selected project; a skip means no SDD requested for this provider and does not mark all projects installed. Introducing a new project does not automatically invalidate that skip. Changes to reviewed permission controls, relevant MCP configuration/pairing, installed assets, or the chosen project's root/framework invalidate only their affected outcome. Removed providers no longer contribute to startup aggregation. Missing agent/pairing or read failures produce unknown, not incomplete. No supported catalogue providers means no setup toast.

## Startup and persistence
After catalogue and setup-state reads are available, aggregate distinct supported providers. Show one toast per Desktop application launch if any provider is definitively incomplete; list those providers and open the assistant from its action. Unknown providers do not establish incompleteness; another definitively incomplete provider can still trigger it. Dismissal lasts for this launch only. Reconnection retries state reads without duplicating the toast. Persist provider/project decisions across Desktop restarts; never persist a global authoritative completed boolean.

## Permissions and preservation
Claude: reuse conversation permissions and workstation sandbox/whitelist, with explicit project scope when project controls are offered; explain Windows sandbox unavailability. Codex: existing approval-reviewer controls. Antigravity: existing conversation permission mode. Allow explicit retention without writing configuration. Save changes only after user action and acknowledge only refreshed settings. Reuse optimistic concurrency/base values where supported. Preserve unrelated provider settings, other MCP servers, plugin assets and project content. Claude stale-entry repair requires its existing explicit repair action. No new child process is planned; any required implementation process must use agentexec.Hidden and have Windows coverage per AGENTS.md.

## Validation and rejected alternatives
Configuration readback proves saved configuration only. Summary includes per-step outcomes and restart reminder, without launching AI sessions. Reject automatic startup modal, blanket completion flags, automatic permission presets, auto-installing plugins, silently selecting the first project, and a second provider configuration writer. Runtime additions follow the accepted French surface requirement; existing unrelated strings are not translated. Artifacts/comments/tests remain English.

Test the reducer and integration boundary with real outcome shapes, then Desktop UI and Go handlers. Include restart, changed-file invalidation, missing capability, partial failure and stale response cases. No production builds are required to validate this documentation-only stage; implementation checks are listed in tasks.md.
