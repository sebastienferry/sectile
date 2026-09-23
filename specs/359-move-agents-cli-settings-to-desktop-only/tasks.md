# #359 — Implementation Checklist

Ordered so the configuration and server validation layers are updated and tested first, followed by the Web UI simplification, and completed by the Desktop UI implementation and integration tests.

---

## 1. Server & Local Agent Layer (`internal/db`, `internal/agentconfig`, `internal/agent`)

- [x] **T1** In `internal/db/db.go`, remove the server-side `models.SupportsAutonomousRun` preflight check from `EnqueueFullChainRun(taskID string)`.
- [x] **T2** In `internal/db/`, update existing unit tests in `db_test.go` or `chain_test.go` to verify that `EnqueueFullChainRun` succeeds without requiring server-stored CLI templates.
- [x] **T3** In `internal/agentconfig/local.go`, update `ApplyOverrides`:
  - When `overrides.Commands[c.ProjectID]` is empty or omitted, fall back to workstation default `overrides.AICommandTemplate`.
  - When `overrides.CommandsAutonomous[c.ProjectID]` is empty or omitted, fall back to workstation default `overrides.AICommandTemplateAutonomous`.
  - Maintain provider change protection: clear inherited command templates when switching providers without explicitly providing new templates.
- [x] **T4** In `internal/agent/agent_run.go` (and/or `agent_operations.go`), add local preflight capability validation:
  - When preparing an autonomous run (`Headless` mode or `SkillModeAutonomous`), verify `models.SupportsAutonomousRun(effective.AIProvider, effective.AICommandTemplate, effective.AICommandTemplateAutonomous)`.
  - If unsupported, reject execution immediately with a clear error before launching child processes.
- [x] **T5** In `internal/agentconfig/local_test.go` and `internal/agent/agent_desktop_test.go`, add unit tests:
  - Test command template inheritance from workstation defaults (`overrides.AICommandTemplate` / `overrides.AICommandTemplateAutonomous`).
  - Test rejection of headless execution on the agent when the effective provider/template does not support autonomous mode.

---

## 2. Web UI Simplification (`web/src/`)

- [x] **T6** In `web/src/components/ProfileModal.tsx`:
  - Remove the CLI parameters section (interactive command, autonomous command, variables pill buttons, preset buttons, and `CommandModePreview`) from the `aiEngine` tab.
  - Move `MCPEngineConfig` from the `aiEngine` tab to the `workstations` tab.
  - Preserve the provider selection (`AI_PROVIDERS`), proposed models (`ProviderModelsField`), and default model input (`AIModelField`).
  - Remove CLI command templates from profile save payloads.
- [x] **T7** In `web/src/components/ProjectModal.tsx`:
  - Remove the CLI parameters section (interactive command, autonomous command, presets, variables, `CommandModePreview`) and `MCPEngineConfig` from the "Agent" tab.
  - Retain the custom agent toggle, AI provider selector, and project model field.
  - Remove `aiCommandTemplate` and `aiCommandTemplateAutonomous` from project create and update payloads.
- [x] **T8** In `web/src/components/AIModelField.tsx` and related components:
  - Update any props or warnings that assumed `commandTemplate` was passed from the parent modal.
- [x] **T9** In `web/tests/profileModal.test.mjs` and web test suite:
  - Update tests to verify that `ProfileModal` renders AI Engine and Workstations tabs correctly without CLI command inputs.
  - Verify that `npm test && npx tsc --noEmit && npx oxlint src` passes cleanly in `web/`.

---

## 3. Desktop Application Updates (`desktop/src/`, `desktop/electron/`)

- [x] **T10** In `desktop/electron/preload.cjs` and `desktop/electron/main.cjs`:
  - Expose `saveSettings: settings => ipcRenderer.invoke('save-settings', settings)`.
  - Implement the `save-settings` IPC handler in `desktop/electron/main.cjs` to merge and write updated settings (`aiProvider`, `aiModel`, `aiCommandTemplate`, `aiCommandTemplateAutonomous`) atomically to `settings.json`.
- [x] **T11** In `desktop/src/main.js`:
  - Add `{id: 'AgentCli', label: 'Agents CLI', icon: ...}` to `SETTINGS_CATEGORIES`.
  - Build the "Agents CLI" settings panel in `openSettings()`:
    - Provider dropdown with supported options (`agy`, `claude`, `codex`, `gemini`, `cursor`, `vibe`, `custom`).
    - Model input field with validation.
    - Interactive and Autonomous CLI command textareas with variable help.
    - Live command preview using `previewLines` from `command-preview.mjs`.
    - Fast preset buttons.
    - Save button committing changes via `api.saveSettings(...)`.
- [x] **T12** In `desktop/src/main.js` (`openProject` dialog):
  - Re-anchor fallback values for provider, model, and command templates to workstation defaults loaded via `api.settings()`.
  - Update status hint texts:
    - Provider: `(inheritAiProvider ? 'Inherited from workstation' : 'Local override') + ' · Workstation default: ' + (wsSettings.aiProvider || 'agy')`
    - Model: `(inheritAiModel ? 'Inherited from workstation' : 'Local override') + ' · Workstation default: ' + (wsSettings.aiModel || '(none)')`
    - Commands: `(inheritCommand ? 'Inherited from workstation' : 'Local override') + ' · Both empty runs the provider default for each mode.'`
  - Update reset buttons to reset to workstation defaults ("Reset AI provider to workstation default", "Reset AI model to workstation default", "Reset CLI commands to workstation defaults").
  - On reset, set field values to workstation defaults and flag inheritance.
- [x] **T13** In `desktop/tests/project-settings.test.mjs`:
  - Update assertions to expect workstation inheritance labels ("Inherited from workstation", "Reset AI provider to workstation default", etc.).
- [x] **T14** In `desktop/tests/`:
  - Add or update tests covering the "Agents CLI" panel in the Settings dialog and verify `saveSettings` calls.

---

## 4. Quality Gates & Verification

- [x] **T15** Run Go test suite:
  ```bash
  go test -v -race ./internal/agentconfig/... ./internal/agent/... ./internal/db/... ./internal/models/...
  ```
- [x] **T16** Run Web test suite and linters:
  ```bash
  cd web && npm test && npx tsc --noEmit && npx oxlint src
  ```
- [x] **T17** Run Desktop build and test suites:
  ```bash
  cd desktop && npx vite build && npm test && npm run test:ui
  ```
- [x] **T18** Verify all acceptance criteria from US1 through US5 in `spec.md`.
