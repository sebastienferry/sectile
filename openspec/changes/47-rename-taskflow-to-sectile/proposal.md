# Rename TaskFlow to Sectile

## Why
The product owner selected **Sectile** (slug: `sectile`) because the previous product name "taskflow" is already taken and overloaded in the software ecosystem. The name *Sectile* derives from *Opus Sectile*—the Roman art of precisely cutting and fitting distinct pieces together into a seamless mosaic—fittingly capturing modular agent orchestration and task composition. This change brings consistent product presentation, executable names, and release distribution while preserving full backwards compatibility for existing installations, databases, and automation.

Source: [issue #47](https://github.com/sebastienferry/taskflow/issues/47) and [settled clarification](../../../docs/clarifications/47.md).

## What Changes
- Use `Sectile` for public display branding, UI titles, browser page headers, server startup banners, and current documentation.
- Use `sectile` as the executable output name (`bin/sectile`) and distribution binary artifact prefix (`dist/sectile-<os>-<arch>`).
- Use `sectile-web` as the frontend package identifier in `web/package.json`.
- Update generated workflow prose, default prompts, CLI usage examples, API landing-page presentation, and installation instructions.
- Preserve existing database discovery order (`DB_PATH`, working-directory `tasks.db`, user-config `taskflow/tasks.db`, then `taskacao/tasks.db`).
- Preserve all existing configuration file paths (`.taskflow/`), environment variables (`TASKFLOW_*`, `TASKACAO_*`), localStorage preference keys, machine-facing health checks (`taskflow-api`), and terminal execution protocol markers (`__TASKFLOW_*`).

## Capabilities
### New Capabilities
- `product-identity`: Consistent Sectile product presentation, CLI/server branding, and distribution artifacts with preserved backwards compatibility contracts.

### Modified Capabilities
None. Existing workflow execution and persistence semantics serve as the compatibility baseline.

## Impact
- **Frontend**: `web/index.html`, `web/src/locales/translations.ts`, and `web/package.json`.
- **Backend / CLI**: `cmd/server/main.go`, `internal/runner/runner.go`, `internal/db/projectcontext.go`, and `internal/db/skilltemplates.go`.
- **Build / Packaging**: `Makefile` build, run, and release targets.
- **Documentation**: `README.md` and current guides under `docs/`.
- No database schema migrations or breaking external API changes.

## Out of Scope
- Visual design overhaul, logo replacement, new color palettes, or taglines.
- Renaming the remote GitHub repository (`sebastienferry/taskflow`) or Go module (`tasks`).
- Relocating or migrating existing user SQLite databases (`tasks.db`).
- Modifying machine protocol markers (`__TASKFLOW_*`) or health check service IDs (`taskflow-api`).
- Rewriting historical reports, closed change specifications, or changelogs.
- Any changes to separate existing skill-routing work.

## Open Questions
None. All product decisions, naming choices, and technical compatibility contracts are fully settled.
