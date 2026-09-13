# Implementation checklist

## 1. Inventory and baseline
- [x] Inventory current presentation, generated templates, package metadata, and technical identifiers; classify intentional legacy occurrences using the design table.
- [x] Preserve unrelated existing work, including skill-routing updates; establish baseline tests before code edits.

## 2. Product and distribution
- [x] Update UI/browser and server/CLI/terminal product text to Sectile without altering visual styling, icons, or machine identifiers.
- [x] Update `Makefile` build/run/release targets to produce `bin/sectile` and `dist/sectile-*`, and update `web/package.json` to `sectile-web`.
- [x] Update source generators and workflow template prose in `internal/db/projectcontext.go` and `internal/db/skilltemplates.go`; preserve `taskflow:project-context` delimiters.

## 3. Compatibility verification
- [x] Verify database selection order across explicit `DB_PATH`, working-directory `tasks.db`, and user config paths; confirm existing files are unmodified.
- [x] Verify environment variable compatibility (`TASKFLOW_*` and `TASKACAO_*`) and process environment propagation to custom commands.
- [x] Verify browser preference persistence and existing `taskacao_*` read fallbacks in localStorage.
- [x] Verify project instruction generation idempotence and preservation of unknown keys and custom instructions outside owned blocks.
- [x] Verify health check service identifier (`taskflow-api`) and terminal protocol markers (`__TASKFLOW_*`) remain unchanged.

## 4. Documentation and acceptance
- [x] Update `README.md` and current documentation in `docs/` with Sectile commands, architecture notes, and retained compatibility tables.
- [x] Document upgrade path for custom launchers and optional local aliases while retaining real GitHub URLs.
- [x] Run `go test ./...` and `make test` (Go internal tests, frontend tests, TypeScript check, oxlint).
- [x] Run `make build` and smoke test `bin/sectile` with an isolated database; inspect browser branding and embedded assets.
- [x] Run `make release` and verify all five release artifacts exist with expected names.
- [x] Review remaining old-name occurrences against the inventory, run `git diff --check`, and record validation results.

## 5. Workflow
- [x] Verify change compliance with `openspec validate 47-rename-taskflow-to-sectile --strict`.
- [ ] Commit and push the specification on the task branch `47-rename-taskflow-to-something-else` and create a draft PR.
- [ ] Complete stage transition via TaskFlow MCP tool `taskflow_transition_stage` to `specified` with PR URL.
