# Accepted implementation refinements

The owner accepted the following refinements during manual desktop testing on
2026-09-26. These supplement the original specification without discarding it.

- Configuration replaces the whole workspace, including the normal sidebar,
  while preserving the application header. Back is a compact arrow button.
- Global settings and all project sections share the configuration sidebar.
  Project headings toggle an accordion with at most one expanded project.
- Every configuration category has a compact title with consistent height and
  top position. Redundant page headings are removed.
- Project General comes first and contains Git remote, SDD framework, default
  engine and Remove from desktop. Folders contains local repository mappings.
  The former Server and AI agent categories and read-only skill view are removed.
- AI engine catalogue management has a dedicated global category, separate from
  Execution defaults. Skill command names and the initialization provider are
  workstation settings; legacy agent compatibility is preserved.
- Deployment separates global AI engine setup from local project SDD setup.
  Global setup selects an engine's provider and writes user-level skills and MCP;
  the current project's server skills remain its explicitly displayed source.
  Engines sharing a provider share that installation. Local SDD setup selects a
  project and installs its framework in the repository.
- Switching categories and projects retains the configuration shell and cleans
  up footer actions, preventing duplicate Refresh from server buttons.

The owner confirmed satisfaction after these refinements. Workflow stage remains
implemented; manual acceptance does not substitute for the review stage.
