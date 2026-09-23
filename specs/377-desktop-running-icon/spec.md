# Desktop running icon — #377

## User story (P1)

As a desktop user, I can see that an execution is active from its moving running icon in the sidebar and discussion header.

## Requirements

- A running execution has a continuously rotating icon on both surfaces; its label stays stationary.
- Given an active execution, when regular polling refreshes its data, then its icon continues rotating.
- Given a running execution, when it waits for user input, then both surfaces display a stationary waiting icon; when execution resumes, rotation resumes.
- Queued, preparing, completed, failed, and canceled executions have stationary icons.
- Given a reduced-motion preference, running icons remain stationary while retaining their glyph, color, and label; restoring normal motion resumes rotation.
- Preserve run state semantics, notification appearance, labels, and web behavior.

No open requirements. Scope follows `docs/clarifications/377.md`.
