1. Reuse the assigned worktree and branch (including a shared batch branch). Only create <KEY>-<title-slug> when no work branch is assigned. Preserve existing work; never write on the default branch.
2. Select the SDD framework from {sdd_framework} argument, flag, or project detection:

   **If using OpenSpec SDD:**
   - Create change directory `openspec/changes/<KEY>-<title-slug>/`
   - Write `proposal.md` (problem, value, in/out scope)
   - Write `design.md` (technical decisions, rejected alternatives)
   - Write `tasks.md` (ordered implementation checklist)
   - Write `specs/<capability>/spec.md` (requirements with Given/When/Then)
   - Validate with `openspec validate <change-id> --strict`

   **If using Spec Kit SDD:**
   - Write `specs/<KEY>-<title-slug>/spec.md` (prioritised user stories, functional requirements, Given/When/Then)
   - Write `plan.md` (stack, architecture, data contracts, target files)
   - Write `tasks.md` (ordered implementation checklist with test plan)
   - Use `/speckit.specify`, `/speckit.plan`, `/speckit.tasks` if available.
3. Dropped artefacts: before committing the specification, run `git check-ignore -q` on one of its
   files (`specs/<KEY>-<title-slug>/spec.md` or `openspec/changes/<KEY>-<title-slug>/proposal.md`).
   When it succeeds, the project drops its specification artefacts on this workstation: write the
   files in the worktree, never commit them, never force them with `git add -f`, and put the
   requirements and the open points in the transition note, saying that the files stay local to the
   worktree. Open no pull request at this stage then, even when the project creates it after
   specification: say in the report that it is deferred to the implemented stage.