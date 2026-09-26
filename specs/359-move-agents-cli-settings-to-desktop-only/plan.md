# #359 — Implementation Plan

## Stack

- **Web Frontend**: React 19, TypeScript, Tailwind CSS, Vite (`web/src/`).
- **Desktop Companion**: Electron, JavaScript, HTML/CSS, Node.js IPC, Vite (`desktop/src/`, `desktop/electron/`).
- **Go Server & Local Agent**: Go 1.24, SQLite (`internal/agent/`, `internal/agentconfig/`, `internal/db/`, `internal/models/`).

---

## Architecture Decisions

### D1 — Web UI Simplification (CLI Templates Removal)

1. **`web/src/components/ProfileModal.tsx`**:
   - In the `aiEngine` tab, remove the CLI Parameters section:
     - Remove `aiCommandTemplate` and `aiCommandAutonomous` text inputs.
     - Remove the available variables pill buttons (`{prompt}`, `{issueKey}`, etc.).
     - Remove the fast preset buttons (`COMMAND_PRESETS`).
     - Remove `CommandModePreview`.
   - Keep provider selection (`AI_PROVIDERS`), proposed models (`ProviderModelsField`), and default model selection (`AIModelField`).
   - Move `MCPEngineConfig` from the `aiEngine` tab to the `workstations` tab. In `workstations`, it provides direct connection code snippets for desktop AI tools (Claude Code, Cursor, Codex, etc.).
   - When saving profile settings, CLI command templates are no longer submitted.

2. **`web/src/components/ProjectModal.tsx`**:
   - In the "Agent" tab, remove the CLI parameters section (interactive command, autonomous command, preset buttons, variables, and `CommandModePreview`).
   - Retain the custom agent toggle, AI provider selection (`AI_PROVIDERS`), and project model input (`AIModelField`).
   - Remove `MCPEngineConfig` from `ProjectModal.tsx`.
   - Update project create and update payloads to omit `aiCommandTemplate` and `aiCommandTemplateAutonomous`.

3. **`web/src/components/AIModelField.tsx`**:
   - Make `commandTemplate` prop optional or adapt placeholder warnings so `{model}` presence check does not trigger warning banners when command templates are absent in the web UI.

---

### D2 — Workstation Global Settings in Desktop ("Agents CLI" Category)

1. **Category in `desktop/src/main.js`**:
   - Add `{id: 'AgentCli', label: 'Agents CLI', icon: '<path d="m5 7 5 5-5 5"/><path d="M12 19h7"/>'}` to `SETTINGS_CATEGORIES`.
   - Insert between `Profile` and `Connection` (or after `Profile`).
   - The panel renders:
     - **AI Provider**: Dropdown selecting the workstation default provider (`agy`, `claude`, `codex`, `gemini`, `cursor`, `vibe`, `custom`).
     - **AI Model**: Input specifying the workstation default model, validated against `^[A-Za-z0-9][A-Za-z0-9._:@/-]*$`.
     - **Interactive CLI Command**: Textarea for `aiCommandTemplate` with variable documentation.
     - **Autonomous CLI Command**: Textarea for `aiCommandTemplateAutonomous`.
     - **Command Preview Box**: Live preview of command formatting using `previewLines` from `command-preview.mjs`.
     - **Preset buttons**: Quick presets for standard tools (`claude`, `agy`, `codex`, `custom`).
     - **Save Button**: Commits values to `~/.config/sectile/settings.json`.

2. **IPC Integration for Settings Persistence**:
   - In `desktop/electron/preload.cjs`: expose `saveSettings: settings => ipcRenderer.invoke('save-settings', settings)`.
   - In `desktop/electron/main.cjs`: add handler for `save-settings`. Read existing `settings.json`, merge updated keys (`aiProvider`, `aiModel`, `aiCommandTemplate`, `aiCommandTemplateAutonomous`), write atomically using temporary file rename with mode `0o600`.
   - If the local agent is running, changes are immediately picked up on the next command execution because `agentconfig.ReadSettings` reads `settings.json` from disk dynamically.

---

### D3 — Desktop Project Settings Workstation Inheritance

1. **Re-anchoring Project Inheritance in `desktop/src/main.js`**:
   - In `openProject(id)`:
     - Read workstation defaults from `api.settings()` (stored in `wsSettings`).
     - Re-anchor initial fallbacks:
       - `selectedProvider = info.aiProvider || wsSettings.aiProvider || 'agy'`
       - `selectedModel = info.aiModel ?? wsSettings.aiModel ?? ''`
       - `command.value = info.aiCommandTemplate ?? wsSettings.aiCommandTemplate ?? ''`
       - `autonomousCommand.value = info.aiCommandTemplateAutonomous ?? wsSettings.aiCommandTemplateAutonomous ?? ''`
     - Re-label reset buttons:
       - "Reset AI provider to workstation default"
       - "Reset AI model to workstation default"
       - "Reset CLI commands to workstation defaults"
     - Re-label status hints:
       - Provider hint: `(inheritAiProvider ? 'Inherited from workstation' : 'Local override') + ' · Workstation default: ' + (wsSettings.aiProvider || 'agy')`
       - Model hint: `(inheritAiModel ? 'Inherited from workstation' : 'Local override') + ' · Workstation default: ' + (wsSettings.aiModel || '(none)')`
       - Command hint: `(inheritCommand ? 'Inherited from workstation' : 'Local override') + ' · Both empty runs the provider default for each mode.'`
     - Reset action behavior:
       - `providerReset.onclick`: revert to `wsSettings.aiProvider || 'agy'`, set `inheritAiProvider = true`, update hints.
       - `modelReset.onclick`: revert to `wsSettings.aiModel || ''`, set `inheritAiModel = true`, update hints.
       - `commandReset.onclick`: revert to `wsSettings.aiCommandTemplate || ''` and `wsSettings.aiCommandTemplateAutonomous || ''`, set `inheritCommand = true`, update hints.

---

### D4 — Configuration Precedence & Fallbacks in Agent Daemon

1. **`internal/agentconfig/local.go` (`ApplyOverrides`)**:
   - Precedence order for commands:
     1. Per-project override: `overrides.Commands[c.ProjectID]`. If non-empty, use it.
     2. Workstation default: `overrides.AICommandTemplate`. If non-empty and project did not set an override, use it.
     3. Provider built-in default command (if both are empty).
   - Precedence order for autonomous commands:
     1. Per-project override: `overrides.CommandsAutonomous[c.ProjectID]`.
     2. Workstation default: `overrides.AICommandTemplateAutonomous`.
     3. Fallback to interactive command or provider default autonomous flags.
   - Provider change protection:
     - If per-project provider override differs from workstation provider, and neither project command nor workstation command is explicitly tailored for the new provider, clear command templates so default flags of the new provider apply.

2. **`internal/agent/agent_desktop.go`**:
   - Ensure `desktopProject` (`GET /desktop/project`) correctly reflects workstation overrides and inheritance states.

---

### D5 — Delegated Autonomous Run Capability Validation

1. **Server Side (`internal/db/db.go`)**:
   - In `EnqueueFullChainRun(taskID string)`:
     - Remove the preflight check `if !models.SupportsAutonomousRun(project.AIProvider, project.AICommandTemplate, project.AICommandTemplateAutonomous)`.
     - The server enqueues the first autonomous step with `models.SkillModeAutonomous` and the task's stop stage without rejecting based on database columns.

2. **Agent Side (`internal/agent/agent_run.go` / `agent_operations.go`)**:
   - When the agent prepares to launch a headless/autonomous run:
     - Resolve the effective project configuration via `agentconfig.ApplyOverrides`.
     - Validate `models.SupportsAutonomousRun(effective.AIProvider, effective.AICommandTemplate, effective.AICommandTemplateAutonomous)`.
     - If autonomous execution is not supported by the effective configuration, fail the run immediately with an explicit error before launching any subprocess or allocating resources.

---

### D6 — Database & Request Payloads Compatibility

1. **SQLite Columns**:
   - In accordance with ADR 0021 and project constraints, columns `projects.ai_command_template`, `projects.ai_command_template_autonomous`, `settings.ai_command_template`, `settings.ai_command_template_autonomous` remain in the SQLite tables unwritten.
   - No destructive database migration is performed.

2. **Models & Handlers (`internal/models/models.go`, `internal/handlers/`)**:
   - Web API project update/create handlers omit or ignore incoming command template fields.

---

## Target Files

| Layer | File | Changes |
|---|---|---|
| **Web UI** | `web/src/components/ProfileModal.tsx` | Remove CLI parameters section from `aiEngine` tab. Move `MCPEngineConfig` to `workstations` tab. |
| **Web UI** | `web/src/components/ProjectModal.tsx` | Remove CLI parameters section and `MCPEngineConfig`. Retain AI Provider & Model fields. |
| **Web UI** | `web/src/components/AIModelField.tsx` | Update validation / warning logic when command templates are not provided. |
| **Web UI** | `web/src/locales/translations.ts` | Clean up or adapt translation keys for CLI parameters if no longer used. |
| **Web Tests** | `web/tests/profileModal.test.mjs` | Update tests for ProfileModal tab contents and fields. |
| **Desktop App** | `desktop/src/main.js` | Add "Agents CLI" category to `SETTINGS_CATEGORIES`. Re-anchor project settings inheritance to workstation defaults. |
| **Desktop Electron** | `desktop/electron/preload.cjs` | Expose `saveSettings` to renderer. |
| **Desktop Electron** | `desktop/electron/main.cjs` | Add `save-settings` IPC handler saving to `settings.json`. |
| **Desktop Tests** | `desktop/tests/project-settings.test.mjs` | Update assertions to expect workstation inheritance ("Inherited from workstation", "Reset ... to workstation default"). |
| **Desktop Tests** | `desktop/tests/*.ui.cjs` | Add or update tests covering the "Agents CLI" panel in Settings dialog. |
| **Agent Config** | `internal/agentconfig/local.go` | Update `ApplyOverrides` fallback hierarchy for command templates. |
| **Agent Config** | `internal/agentconfig/local_test.go` | Unit tests for workstation command inheritance. |
| **Agent Daemon** | `internal/agent/agent_desktop.go` | Ensure `/desktop/project` and `/desktop/projects` handle workstation inheritance. |
| **Agent Daemon** | `internal/agent/agent_run.go` | Enforce local `SupportsAutonomousRun` preflight on headless run launch. |
| **Server DB** | `internal/db/db.go` | Remove server-side `SupportsAutonomousRun` preflight in `EnqueueFullChainRun`. |
| **Server DB** | `internal/db/db_test.go` | Update or remove tests asserting server-side full chain refusal on missing CLI templates. |

---

## Verification & Testing Plan

1. **Go Test Suite**:
   ```bash
   go test -v -race ./internal/agentconfig/... ./internal/agent/... ./internal/db/... ./internal/models/...
   ```
2. **Web Test & Lint Suite**:
   ```bash
   cd web && npm test && npx tsc --noEmit && npx oxlint src
   ```
3. **Desktop Unit & UI Test Suite**:
   ```bash
   cd desktop && npx vite build && npm test && npm run test:ui
   ```
