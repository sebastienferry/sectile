# #359 — Move Agents CLI settings to Desktop only

## Context

AI agent execution runs locally on the developer's workstation via the Sectile Desktop companion and the local agent daemon. Historically, CLI command templates (`aiCommandTemplate`, `aiCommandTemplateAutonomous`), presets, and CLI argument customization were configured on the central web server and presented in the Web UI within `ProfileModal.tsx` (`aiEngine` tab) and `ProjectModal.tsx` (`agent` tab).

Configuring workstation-specific execution command lines in a centralized database causes operational friction:
1. Different workstations have different binary installation paths, terminal shells, and local execution flags.
2. Centralizing CLI templates required an unnatural inheritance hierarchy where project settings on the desktop attempted to inherit from server templates.
3. Server-side pre-flight validations (such as `models.SupportsAutonomousRun` in `internal/db/db.go`) inspected server database columns rather than the actual local workstation capabilities, leading to false negatives or inability to execute headless runs with workstation-configured tools.

As decided in Round 2 of clarification (`docs/clarifications/359.md`):
- AI engine/provider (`aiProvider`) and model (`aiModel`, `aiProviderModels`) selection are retained in the Web UI (`ProfileModal.tsx` and `ProjectModal.tsx`) as high-level project tooling preferences.
- CLI command template configuration, fast presets, and live command preview are removed from the Web UI.
- Direct MCP configuration snippets (`MCPEngineConfig`) move from the `aiEngine` tab to the `workstations` tab in `ProfileModal.tsx`.
- The Desktop application adds an "Agents CLI" category in its main Settings dialog (`openSettings`) to manage workstation-wide default provider, default model, and default interactive and autonomous command templates stored in `~/.config/sectile/settings.json`.
- Desktop project settings inherit command templates and engine defaults from the local workstation defaults rather than server defaults, displaying "Inherited from workstation" and providing "Reset to workstation default" controls.
- The server-side autonomous run preflight in `EnqueueFullChainRun` is removed, delegating headless execution capability validation to the local agent daemon upon dispatch.

This specification describes user-observable behavior and acceptance criteria only. Technical architecture, data contracts, and implementation details are described in `plan.md`, with an implementation checklist in `tasks.md`.

---

## Decisions Being Specified

1. **Web UI CLI Template Removal**:
   - The CLI parameters section (interactive command template input, autonomous command template input, fast preset buttons, variable pills, and command mode preview) is removed from both `ProfileModal.tsx` and `ProjectModal.tsx`.
   - The AI engine selection (`aiProvider`) and model selection (`aiModel`, `aiProviderModels`) remain available in `ProfileModal.tsx` and `ProjectModal.tsx`.
   - The direct MCP instructions component (`MCPEngineConfig`) is moved to the `workstations` tab in `ProfileModal.tsx`.

2. **Global Workstation Settings in Desktop App**:
   - A new category titled "Agents CLI" is added to Desktop's Settings dialog (`openSettings`), sitting alongside "General", "User profile", "Agent connection", and "Agent logs".
   - The "Agents CLI" panel allows configuring workstation-wide defaults: AI Provider, AI Model, Interactive CLI command template, and Autonomous CLI command template.
   - Modifications made in this panel are persisted locally in `~/.config/sectile/settings.json`.

3. **Desktop Project Settings Workstation Inheritance**:
   - In the Desktop project configuration dialog, CLI command templates, AI Provider, and AI Model inherit from the workstation's global settings rather than server settings.
   - Status indicators read "Inherited from workstation" when no project override is defined, and "Local override" when a project-specific override is active.
   - Reset buttons revert project settings to the workstation defaults ("Reset to workstation default").

4. **Delegated Autonomous Execution Capability Check**:
   - The server does not reject full-chain autonomous runs based on server-stored CLI templates.
   - When a chained or autonomous run is received by the local agent daemon, the daemon validates that an autonomous command or attested autonomous provider is available on the local workstation before executing.

---

## User Stories

### US1 — Web Profile Modal retains AI Engine and moves MCP configuration to Workstations tab (P1)

**As a** developer configuring my user preferences in the Sectile web interface,  
**I want** to choose my default AI engine and proposed models without seeing workstation-specific CLI command templates,  
**So that** my profile focuses on high-level tool choices and does not clutter my web settings with machine-dependent shell commands.

- **Given** an authenticated user opening `ProfileModal` in the Web UI
- **When** the user selects the "Paramètres de l'agent" (`aiEngine`) tab
- **Then** the AI provider selector (AGY, Claude Code, Codex, Gemini, Cursor, Vibe, Custom) is visible and interactive.
- **And** the proposed models per provider (`ProviderModelsField`) and default model input (`AIModelField`) are visible and interactive.
- **And** no CLI command template inputs, variable buttons, fast presets, or live preview boxes are rendered.
- **And** direct MCP configuration snippets are not displayed on this tab.

- **Given** an authenticated user opening `ProfileModal` in the Web UI
- **When** the user selects the "Postes de travail" (`workstations`) tab
- **Then** the direct MCP configuration snippets component (`MCPEngineConfig`) is displayed alongside workstation pairing and desktop companion controls.

---

### US2 — Web Project Modal retains AI Engine selection and removes CLI command templates (P1)

**As a** project maintainer editing project settings in the Web UI,  
**I want** to specify the target AI provider and model for my project without configuring CLI flags,  
**So that** project configuration specifies the desired AI tooling cleanly.

- **Given** a user opening `ProjectModal` for a project
- **When** the user navigates to the "Agent" tab
- **Then** the user can toggle between global inheritance and project-specific AI settings.
- **And** when project-specific settings are enabled, the AI Provider selector and project AI Model field are displayed and editable.
- **And** no CLI command template fields (interactive or autonomous), preset buttons, or preview boxes are rendered.
- **And** no direct MCP configuration snippet is rendered in `ProjectModal`.

---

### US3 — Workstation-wide Agents CLI settings panel in Desktop app (P1)

**As a** developer using the Sectile Desktop companion,  
**I want** a dedicated "Agents CLI" settings category in the desktop settings dialog,  
**So that** I can configure my default AI CLI commands, provider, and model for all local projects in one place.

- **Given** the Sectile Desktop application is open
- **When** the user clicks the Settings button in the sidebar footer or presses the settings shortcut
- **Then** the Settings dialog displays the "Agents CLI" tab among the category list (General, User profile, Agents CLI, Agent connection, Agent logs).

- **Given** the user navigates to the "Agents CLI" category in Settings
- **When** the panel renders
- **Then** the panel displays:
  - An "AI Provider" selector dropdown with supported engines (`agy`, `claude`, `codex`, `gemini`, `cursor`, `vibe`, `custom`).
  - An "AI Model" text input with validation against invalid shell characters.
  - An "Interactive CLI command" text area with placeholder documentation.
  - An "Autonomous CLI command (headless)" text area.
  - A live command preview box verifying interactive and autonomous command lines.
  - A save control persisting changes to `~/.config/sectile/settings.json`.

- **Given** the user updates the workstation AI provider or CLI templates in the "Agents CLI" panel
- **When** the user saves the settings
- **Then** the new values are written to `~/.config/sectile/settings.json`.
- **And** any local project configured to inherit workstation defaults immediately reflects the updated commands.

---

### US4 — Desktop Project Settings inherit from Workstation defaults (P1)

**As a** developer configuring a local project in the Desktop application,  
**I want** project CLI commands and AI provider settings to inherit from my workstation defaults,  
**So that** I do not have to re-enter command templates for every project, while still being able to customize them when needed.

- **Given** a project opened in the Desktop project settings dialog (`openProject`)
- **When** the project has not overridden the AI Provider, AI Model, or CLI command templates
- **Then** the AI Provider dropdown reflects the workstation default provider.
- **And** the hint label states "Inherited from workstation · Workstation default: <provider>".
- **And** the AI Model field reflects the workstation default model with hint "Inherited from workstation · Workstation default: <model>".
- **And** the CLI command template fields reflect the workstation default command templates with hint "Inherited from workstation".

- **Given** a project with local overrides configured for provider, model, or command templates
- **When** the user views the project settings
- **Then** the hint labels indicate "Local override".
- **And** reset buttons ("Reset AI provider to workstation default", "Reset AI model to workstation default", "Reset CLI commands to workstation defaults") are available.

- **Given** a project with a local override active
- **When** the user clicks a reset button
- **Then** the field reverts to the workstation default value.
- **And** the hint label updates to "Inherited from workstation".
- **And** saving the project clears the project-specific override from `~/.config/sectile/settings.json`.

---

### US5 — Server delegates Autonomous Run execution preflight to Local Agent (P1)

**As a** developer initiating a full-chain run on a task,  
**I want** the server to accept the chain and dispatch it to my local agent,  
**So that** the local agent checks my workstation's actual CLI capabilities and executes the autonomous chain without server-side configuration conflicts.

- **Given** a task on a project where CLI command templates are not configured in the server database
- **When** the user triggers a full-chain run via `EnqueueFullChainRun`
- **Then** the server does not fail with "le provider n'a pas de mode headless attesté" or command format errors.
- **And** the initial autonomous run is created and queued for the local agent.

- **Given** a queued autonomous run dispatched to a local agent workstation
- **When** the local agent prepares the execution
- **Then** the local agent verifies that the effective provider or effective command template supports autonomous execution locally.
- **And** if autonomous execution is supported, the run starts in headless mode.
- **And** if neither an autonomous template nor an autonomous-capable provider is configured, the local agent reports an explicit error before launching.
