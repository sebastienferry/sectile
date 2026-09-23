# Tasks — #249 Editing a project drops its autonomous CLI command

Spec: [`spec.md`](spec.md) · Plan: [`plan.md`](plan.md)

- [x] T1 Add `internal/db/projectcommands_test.go`: updating the autonomous command persists it,
      absent fields are kept, empty fields clear, the agent config carries it (fails first).
- [x] T2 `UpdateProjectAs`: apply `req.AICommandTemplateAutonomous` when non-nil; trim both commands.
- [x] T3 Add `projectAgentCommands` to `web/src/lib/commandTemplate.ts`, tested in
      `web/tests/commandTemplate.test.mjs`.
- [x] T4 `ProjectModal.tsx`: build the payload with the helper; include the autonomous command in `hasCustomAgent`.
- [x] T5 `CHANGELOG.md`: `Fixed` entry under `[Unreleased]`.
- [x] T6 Run Go build/vet/tests and web tests/type-check; quote the output.
