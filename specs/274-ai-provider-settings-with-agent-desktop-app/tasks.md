# #274 — Implementation checklist

Ordered so the configuration and storage layers are solid and tested before exposing endpoints, and so the agent API is in place before the desktop UI is updated.

---

## 1. Storage & Configuration Layer (`internal/agentconfig`)

- [x] **T1** In `internal/agentconfig/local.go`, extend `Overrides` with `AIProviders map[string]string` (`json:"aiProviders,omitempty"`) and `AIModels map[string]string` (`json:"aiModels,omitempty"`).
- [x] **T2** In `internal/agentconfig/settings.go`, update `WriteSettings` to include `"aiProviders"` and `"aiModels"` in the key lists for deletion and persistence into `~/.config/sectile/settings.json`.
- [x] **T3** In `internal/agentconfig/local.go`, update `ApplyOverrides(c Config, overrides Overrides) Config`:
  - Check `overrides.AIProviders[c.ProjectID]`: if set and non-empty, override `c.AIProvider`. If provider changed and no command override exists for the project or globally, clear `c.AICommandTemplate` and `c.AICommandTemplateAutonomous`.
  - Check `overrides.AIModels[c.ProjectID]`: if set and non-empty, use it as the project base model in `MergeModels`.
- [x] **T4** In `internal/agentconfig/local_test.go` and `internal/agentconfig/settings_test.go`, add unit tests:
  - Verify project-level provider and model override precedence over global and server defaults.
  - Verify that resetting (omitting or deleting) falls back to server configuration.
  - Verify `WriteSettings` and `ReadSettings` round-trip correctly for `aiProviders` and `aiModels`.

---

## 2. Agent Desktop HTTP Endpoints (`internal/agent`)

- [x] **T5** In `internal/agent/agent_desktop.go`, update `desktopProject` (`GET /desktop/project`):
  - Return `aiProvider: effective.AIProvider`.
  - Return `aiModel: effective.AIModel`.
  - Return `aiProviderOverride: overrides.AIProviders[id] != ""`.
  - Return `aiModelOverride: overrides.AIModels[id] != ""`.
- [x] **T6** In `internal/agent/agent_desktop.go`, update `desktopProjects` (`POST /desktop/projects`):
  - Decode `aiProvider`, `aiModel`, `inheritAiProvider`, and `inheritAiModel`.
  - Validate `aiProvider` against the allowed providers set (`agy`, `claude`, `codex`, `gemini`, `cursor`, `vibe`, `custom`). If custom, verify that `{prompt}` is present in the command template.
  - Validate `aiModel` with `agentconfig.ValidModel`.
  - If invalid, respond with HTTP 400 and a clear error message.
  - If `inheritAiProvider` is true, remove project from `overrides.AIProviders`; otherwise save `*input.AIProvider`.
  - If `inheritAiModel` is true, remove project from `overrides.AIModels`; otherwise save `*input.AIModel`.
  - Persist via `agentconfig.WriteSettings(overrides)`.
- [x] **T7** In `internal/agent/agent_desktop_test.go`, add tests:
  - Test GET `/desktop/project` returns correct provider/model and override flags.
  - Test POST `/desktop/projects` persists valid overrides and deletes them when inheritance is requested.
  - Test POST `/desktop/projects` rejects invalid model characters or invalid providers with HTTP 400.

---

## 3. Desktop UI Integration (`desktop/src/`)

- [x] **T8** In `desktop/src/main.js` (`openProject` dialog):
  - Add an "AI Provider" selector dropdown with supported engines (`agy`, `claude`, `codex`, `gemini`, `cursor`, `vibe`, `custom`).
  - Add a reset button ("Reset AI provider to server default") that reverts to `config.aiProvider` and marks inheritance.
  - Display status hint indicating whether provider is inherited from server or set as local override.
- [x] **T9** In `desktop/src/main.js` (`openProject` dialog):
  - Add an "AI Model" text input field.
  - Add a reset button ("Reset AI model to server default") that clears the local override and marks inheritance.
  - Add client-side validation against `^[A-Za-z0-9][A-Za-z0-9._:@/-]*$`.
- [x] **T10** In `desktop/src/main.js`:
  - When AI Provider changes, if CLI command is empty or matches the default preset of the previous provider, auto-update command template to the new provider's default command.
  - Re-render command preview using effective provider and model via `renderCommandPreview()`.
- [x] **T11** In `desktop/src/main.js` (`form.onsubmit`):
  - Include `aiProvider`, `aiModel`, `inheritAiProvider`, `inheritAiModel` in the payload passed to `api.mapProject(...)`.
- [x] **T12** In `desktop/tests/`, add or update tests:
  - Verify UI elements render for provider and model.
  - Verify reset actions update the form state and labels.
  - Verify `mapProject` is called with the expected payload.

---

## 4. Quality Gates & Verification

- [x] **T13** Run Go test suite: `go test -v -race ./internal/agentconfig/... ./internal/agent/...`.
- [x] **T14** In `desktop/`, run test suite: `npm test`.
- [x] **T15** Verify each acceptance criterion in `spec.md` (US1 through US5).
