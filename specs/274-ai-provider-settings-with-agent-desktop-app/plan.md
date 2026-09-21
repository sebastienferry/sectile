# #274 — Implementation plan

## Stack

- **Go 1.x agent and config layer**: `internal/agentconfig` (configuration structs, precedence, settings persistence, validation) and `internal/agent` (desktop HTTP endpoints, local daemon execution loop).
- **Electron desktop companion**: `desktop/src/main.js` (project settings UI), `desktop/src/command-preview.mjs` (live CLI command preview), `desktop/electron/preload.cjs`, and `desktop/electron/main.cjs`.

---

## Architecture decisions

### D1 — Workstation Settings Schema Extension (`agentconfig.Overrides`)

Per-project AI Provider and Model settings must live solely on the developer workstation. They are added as maps keyed by project ID to `agentconfig.Overrides`:

```go
type Overrides struct {
    DisconnectedProjects        map[string]bool   `json:"disconnectedProjects,omitempty"`
    Commands                    map[string]string `json:"commands,omitempty"`
    CommandsAutonomous          map[string]string `json:"commandsAutonomous,omitempty"`
    Parallelism                 map[string]int    `json:"parallelism,omitempty"`
    Worktrees                   map[string]bool   `json:"worktrees,omitempty"`
    Projects                    map[string]string `json:"projects"`
    AIProviders                 map[string]string `json:"aiProviders,omitempty"`
    AIModels                    map[string]string `json:"aiModels,omitempty"`
    AIProvider                  string            `json:"aiProvider"`
    AICommandTemplate           string            `json:"aiCommandTemplate"`
    AICommandTemplateAutonomous string            `json:"aiCommandTemplateAutonomous,omitempty"`
    AIModel                     string            `json:"aiModel,omitempty"`
    AISkillModels               map[string]string `json:"aiSkillModels,omitempty"`
    Terminal                    string            `json:"terminal"`
    Skills                      map[string]string `json:"skills"`
}
```

In `internal/agentconfig/settings.go`:
- `WriteSettings` includes `"aiProviders"` and `"aiModels"` in the list of managed override keys stripped and rewritten during atomic settings saves.
- When an override is deleted (reverted to server default), `delete(overrides.AIProviders, projectID)` and `delete(overrides.AIModels, projectID)` remove the key so the settings file remains clean.

### D2 — Unified Precedence in `agentconfig.ApplyOverrides`

All agent execution flows (`agent_run.go`, `agent_console.go`, `agent_operations.go`, `agent_desktop.go`) pass through `agentconfig.ApplyOverrides(config, overrides)`.

The resolution precedence is:
1. **Per-Project Local Override** (`overrides.AIProviders[c.ProjectID]`, `overrides.AIModels[c.ProjectID]`)
2. **Workstation Global Override** (`overrides.AIProvider`, `overrides.AIModel`)
3. **Server Project Default** (`c.AIProvider`, `c.AIModel`)

#### Provider switch command dropping:
When a per-project provider override differs from the server provider (`c.AIProvider`) or global workstation provider, and neither project commands (`overrides.Commands[c.ProjectID]`) nor global command templates are explicitly supplied, `ApplyOverrides` clears `c.AICommandTemplate` and `c.AICommandTemplateAutonomous`. This prevents a command template constructed for one CLI (e.g. `claude`) from being passed to another (e.g. `codex` or `agy`).

#### Model merging:
`effectiveModel` is determined by checking `overrides.AIModels[c.ProjectID]`, falling back to `overrides.AIModel`, and then `c.AIModel`. It is passed into `MergeModels` along with `overrides.AISkillModels` to maintain proper per-skill model inheritance while respecting the project's base model.

### D3 — Local Agent Desktop HTTP API Extension

#### `GET /desktop/project?id=<id>`
Returns the effective configuration alongside explicit override indicators:

```json
{
  "server": {
    "projectId": "ef5a2777-920f-4744-a7e8-a58a4c257a23",
    "projectName": "sectile",
    "aiProvider": "agy",
    "aiModel": ""
  },
  "aiProvider": "claude",
  "aiModel": "claude-opus-5",
  "aiProviderOverride": true,
  "aiModelOverride": true,
  "commandOverride": false,
  "worktreeOverride": false,
  "parallelism": 1
}
```

#### `POST /desktop/projects`
Payload is extended with:
```go
type projectMappingInput struct {
    ProjectID                   string  `json:"projectId"`
    Path                        string  `json:"path"`
    AIProvider                  *string `json:"aiProvider"`
    AIModel                     *string `json:"aiModel"`
    InheritAIProvider           bool    `json:"inheritAiProvider"`
    InheritAIModel              bool    `json:"inheritAiModel"`
    AICommandTemplate           *string `json:"aiCommandTemplate"`
    AICommandTemplateAutonomous *string `json:"aiCommandTemplateAutonomous"`
    InheritCommand              bool    `json:"inheritCommand"`
    InheritWorktrees            bool    `json:"inheritWorktrees"`
    Parallelism                 *int    `json:"parallelism"`
    UseWorktrees                *bool   `json:"useWorktrees"`
}
```

**Validation rules applied before persisting**:
- If `input.AIProvider != nil` and `!input.InheritAIProvider`:
  - Must be in `{"", "agy", "codex", "claude", "gemini", "cursor", "vibe", "custom"}`.
  - If `"custom"`, either `input.AICommandTemplate` or the existing command override must contain `{prompt}`.
- If `input.AIModel != nil` and `!input.InheritAIModel`:
  - Validated via `agentconfig.ValidModel(*input.AIModel)`.
- If invalid, returns HTTP `400 Bad Request` with an explanatory error message without mutating `overrides`.

### D4 — Desktop UI Integration in `desktop/src/main.js`

1. **AI Provider Control**:
   - A dropdown `<select>` or segmented button group offering supported providers:
     - `agy`: AGY CLI (Google Antigravity)
     - `claude`: Claude Code CLI
     - `codex`: Codex CLI
     - `gemini`: Gemini CLI
     - `cursor`: Cursor CLI
     - `vibe`: Mistral Vibe CLI
     - `custom`: Custom Command
   - Next to the label, a reset button ("Reset AI provider to server default") appears when `aiProviderOverride` is true.
   - Status hint displays: `Inherited from server · Default: <serverProvider>` vs `Local override`.

2. **AI Model Control**:
   - An `<input type="text">` for model identifier.
   - A reset button ("Reset AI model to server default") appears when `aiModelOverride` is true.
   - Live client-side validation against `^[A-Za-z0-9][A-Za-z0-9._:@/-]*$`.

3. **Command Template Auto-Synchronization**:
   - When the user changes the provider in the UI:
     - If the command textarea is empty or equals a known default template preset, it updates to the new provider's default template.
     - If the user had entered a custom command, it is preserved, and `renderCommandPreview()` displays any compatibility errors.

4. **Live Command Preview**:
   - `renderCommandPreview()` passes the selected provider and model to `previewLines(provider, command, model, autonomousCommand)`.

### D5 — Rejected Alternatives

- **Altering the server SQLite database (`projects` table)**: Rejected. Multiple engineers on a team share the same central project but operate on different OS/hardware with different installed agent CLIs and personal API keys. Modifying the central server setting would overwrite settings for all users.
- **Exposing the full `aiSkillModels` matrix per project in desktop UI**: Rejected. Configuring individual models for each skill stage (clarify, specify, implement, adjust, handoff) per project in the desktop dialog adds unnecessary visual noise. Project-level base model override satisfies all acceptance criteria while allowing global per-skill overrides to be inherited naturally.

---

## Data contracts

### 1. `~/.config/sectile/settings.json`

```json
{
  "projects": {
    "ef5a2777-920f-4744-a7e8-a58a4c257a23": "/Users/sferry/Sources/sectile"
  },
  "aiProviders": {
    "ef5a2777-920f-4744-a7e8-a58a4c257a23": "claude"
  },
  "aiModels": {
    "ef5a2777-920f-4744-a7e8-a58a4c257a23": "claude-opus-5"
  },
  "commands": {},
  "commandsAutonomous": {},
  "parallelism": {},
  "worktrees": {}
}
```

### 2. Desktop Local Agent Endpoint Contracts

#### `GET /desktop/project?id={id}`
```typescript
interface DesktopProjectResponse {
  server: {
    projectId: string;
    projectName: string;
    aiProvider: string;
    aiModel: string;
    aiCommandTemplate: string;
    aiCommandTemplateAutonomous: string;
    useWorktrees: boolean;
    specFramework: string;
    gitRemoteUrl: string;
  };
  monoRepo: boolean;
  path: string;
  configured: boolean;
  useWorktrees: boolean;
  worktreeOverride: boolean;
  parallelism: number;
  aiProvider: string;
  aiModel: string;
  aiProviderOverride: boolean;
  aiModelOverride: boolean;
  aiCommandTemplate: string;
  aiCommandTemplateAutonomous: string;
  commandOverride: boolean;
}
```

#### `POST /desktop/projects`
```typescript
interface DesktopProjectMappingRequest {
  projectId: string;
  path: string;
  aiProvider?: string;
  aiModel?: string;
  inheritAiProvider?: boolean;
  inheritAiModel?: boolean;
  aiCommandTemplate?: string;
  aiCommandTemplateAutonomous?: string;
  inheritCommand?: boolean;
  useWorktrees?: boolean;
  inheritWorktrees?: boolean;
  parallelism?: number;
}
```

---

## Target files

1. `internal/agentconfig/local.go`:
   - Extend `Overrides` with `AIProviders` and `AIModels`.
   - Update `ApplyOverrides` with project-level provider and model resolution.
2. `internal/agentconfig/settings.go`:
   - Include `aiProviders` and `aiModels` in `WriteSettings` deletion and update lists.
3. `internal/agentconfig/local_test.go` & `settings_test.go`:
   - Unit tests verifying per-project provider and model overrides, server fallback, and file round-trips.
4. `internal/agent/agent_desktop.go`:
   - Extend `desktopProjects` (POST) to parse, validate, and store `aiProvider` and `aiModel`.
   - Extend `desktopProject` (GET) to expose `aiProvider`, `aiModel`, `aiProviderOverride`, and `aiModelOverride`.
5. `internal/agent/agent_desktop_test.go`:
   - Integration tests testing GET and POST `/desktop/project(s)` with valid and invalid provider/model overrides.
6. `desktop/src/main.js`:
   - Project configuration dialog UI elements for AI Provider and AI Model.
   - Reset buttons for both settings.
   - Provider change template auto-synchronization and preview integration.
   - Form submission payload in `mapProject`.
7. `desktop/tests/main.test.mjs` (or relevant desktop test suite):
   - Tests covering UI state, reset actions, and submission payload.
