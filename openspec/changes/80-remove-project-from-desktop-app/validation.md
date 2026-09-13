# Specification validation

- `openspec validate 80-remove-project-from-desktop-app --strict` — passed: `Change '80-remove-project-from-desktop-app' is valid`.
- `git diff --check` — passed.
- Scope review: both user decisions are represented, including persistent local disconnection, active-work rejection, explicit re-add, data preservation, and execution/removal concurrency.
- No unresolved product requirements remain.
- Application build and tests are deferred to implementation because this change contains documentation only. The implementation checklist records the required commands.
- Assigned branch: `feat/80`. Specification PR is a draft; implementation tasks remain unchecked.
