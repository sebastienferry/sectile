# Specification #751 - The user guide's Claude desktop session section shows two screenshots

- Ticket: https://github.com/sebastienferry/sectile/issues/751
- Branch: `feat/751`
- Clarification: `docs/clarifications/751.md` (round 1, settled)
- Framework: Spec Kit

## Summary

The subsection `### What the session shows` in `docs/USER_GUIDE.md` shipped as
text only (#606). This change adds the two screenshots the owner captured on a
demo project, so a reader recognises the sidebar and the end of a run at a
glance.

## User story

### US1 - Recognise a skill session (P1)

- **Given** the subsection, **when** I read the emoji table, **then** a
  screenshot of the sidebar shows a project group with sessions titled ✅, ❌
  and ❓.
- **Given** the paragraph on the next step, **then** a screenshot shows the end
  of a completed run with its pull request link and the next step ready to copy.

## Functional requirements

- **FR-001** Both images are PNG files under `docs/images/user-guide/`,
  referenced by relative path from `docs/USER_GUIDE.md`.
- **FR-002** Each image has alt text that describes what it shows.
- **FR-003** The images show demo data only.
- **FR-004** The text of the subsection is unchanged; it still reads without the
  images.

## Open points

None.
