# #274 — Configure AI Provider and Agent settings per project in the desktop app

## Context

Currently, agent configuration for a project is fetched primarily from the server database defaults (`models.Project.AIProvider`, `models.Project.AIModel`). While the desktop application allows local workstation overrides for repository path, worktree usage, concurrency limit (parallelism), and CLI command templates, it provides no interface to configure or override the AI Provider or AI Model on a per-project basis.

Because different projects may target different toolings, LLM providers (e.g., AGY, Claude Code, Codex, Gemini, Cursor, Vibe, or Custom CLI), and models, and because developer workstations have differing local tool installations and credentials, AI Provider and Model settings must be configurable locally per project directly within the desktop application.

This specification describes behaviour only. Technical choices and data contracts are detailed in `plan.md`, and the ordered implementation checklist is in `tasks.md`.

## Decision being specified

1. **Per-Project Local Overrides**: The desktop application provides controls within the project configuration dialog to select an AI Provider and configure an AI Model specifically for that project.
2. **Workstation-Local Storage**: These per-project overrides are persisted strictly on the local workstation (in `~/.config/sectile/settings.json`) and are never written to the server's central SQLite database.
3. **Inheritance & Reset**: Each setting can inherit from the server default or be overridden locally. An explicit "Reset to server default" action is provided for both AI Provider and AI Model.
4. **Execution Precedence**: When an agent task or console execution runs for a project on this workstation, the project's local AI Provider and Model override take precedence over both global workstation defaults and server-fetched project configurations.
5. **CLI Command Synchronization**: When switching AI Provider in the desktop UI, if the current CLI command template is empty or matches a known default template preset, it updates to the new provider's default command to prevent flag mismatches.
6. **Validation & User Feedback**: Invalid provider selections, custom command templates lacking `{prompt}`, or model identifiers containing invalid characters are rejected with immediate, clear feedback.

---

## User stories

### US1 — Project configuration displays and edits AI Provider and Model (P1)

**As a** developer using the Sectile desktop application,  
**I want** to see and configure the AI Provider and AI Model for a project in its configuration dialog,  
**So that** I can tailor execution engines to each project's requirements.

- **Given** a project opened in the desktop application project settings dialog
- **When** the dialog renders the "Local" configuration tab
- **Then** an "AI Provider" selector displays the effective provider, with an indication of whether it is inherited from the server or set as a local override.
- **And** an "AI Model" input field displays the effective model identifier, with an indication of whether it is inherited from the server or set as a local override.
- **And** an "Inherit from server" reset button is available for both AI Provider and AI Model whenever a local override is active.

- **Given** an active local override for AI Provider or AI Model
- **When** the user clicks the respective reset button
- **Then** the value reverts to the server's configured project default.
- **And** the status label updates to "Inherited from server".

### US2 — Local persistence and project isolation (P1)

**As a** developer working on multiple projects on the same machine,  
**I want** my local AI Provider and Model overrides to be saved per project on my workstation,  
**So that** configuring one project does not affect other projects or push unvetted engine changes to the central server.

- **Given** project A configured with provider "claude" and model "claude-opus-5" as local overrides
- **And** project B configured to inherit server defaults ("agy")
- **When** the user saves the local configuration for project A
- **Then** the overrides for project A are persisted to the workstation settings file.
- **And** project B continues to inherit its server default ("agy").
- **And** closing and reopening the desktop app retains project A's overrides and project B's inherited configuration.
- **And** no changes are sent to the server's database schema or central project records.

### US3 — Task and console executions obey project overrides (P1)

**As a** developer running tasks or launching agent consoles,  
**I want** executions for a project to use its locally configured provider and model,  
**So that** local work actually runs with the engine I selected.

- **Given** a project with a local override setting AI Provider to "codex" and AI Model to "gpt-5-codex"
- **When** a task execution or skill run is launched for that project on this workstation
- **Then** the local agent daemon runs the task using Codex with model "gpt-5-codex".
- **Given** the same project where the local overrides are cleared (reverted to server defaults)
- **When** a task execution is launched
- **Then** the agent daemon executes using the server-defined provider and model.
- **Given** the user launches a free console from the desktop app for that project
- **When** the console initializes
- **Then** the console defaults to the project's effective locally overridden provider.

### US4 — Command template synchronization on provider change (P2)

**As a** desktop user switching the AI Provider for a project,  
**I want** CLI command templates to adapt automatically when I haven't customized them,  
**So that** I don't run a new provider with the old provider's incompatible command flags.

- **Given** the CLI command template is either empty or matches the default preset of the current provider
- **When** the user selects a different AI Provider from the dropdown
- **Then** the CLI command template is automatically updated to the default command template of the newly selected provider.
- **And** the live command preview updates to reflect the new provider and model.

- **Given** the user has typed a custom CLI command template with specific custom flags
- **When** the user selects a different AI Provider
- **Then** the custom CLI command template is preserved rather than silently overwritten.
- **And** the live command preview immediately validates and displays whether the custom template is compatible with the new provider.

### US5 — Validation and clear error feedback (P1)

**As a** desktop user configuring AI settings,  
**I want** immediate, understandable validation errors when entering invalid values,  
**So that** broken configurations are caught before attempting to run tasks.

- **Given** the user enters an AI Model identifier containing spaces or forbidden shell metacharacters (e.g. `claude opus; rm -rf /`)
- **When** the user attempts to save the configuration
- **Then** the save action is prevented.
- **And** a clear error message is displayed stating that model identifiers must only contain letters, digits, and allowed punctuation (`. _ - : @ /`).

- **Given** the user selects "custom" as the AI Provider
- **When** the custom command template is empty or does not contain `{prompt}`
- **Then** the save action is prevented.
- **And** an error message is displayed explaining that custom providers require a command template containing `{prompt}`.

- **Given** any validation failure
- **When** the error is presented
- **Then** the previous valid workstation settings remain untouched on disk.
