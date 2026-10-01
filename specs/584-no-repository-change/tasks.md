# Tasks #584 - A task that changed no repository can be recorded as implemented

- [ ] T1 Flag through the DB transition and `validateStagePRs`, with the
  refusals and the notice (FR1 to FR4).
- [ ] T2 DB tests through `prEvidenceLookup`: implemented and reviewed
  succeed with a failing lookup; without the flag the current error; refused
  with `prUrl`, with a recorded PR, with a changed repository (US1, US2).
- [ ] T3 `transition_stage` input and schema; MCP test of the flag.
- [ ] T4 Skill fragments and golden files (US3).
- [ ] T5 Changelog line (AC3).
