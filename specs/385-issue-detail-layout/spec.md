# Issue detail content hierarchy

## User story (P1)

As a Sectile WebUX reader, I want description and technical context directly below the title so I can understand the story before navigating its metadata.

## Requirements

1. The title spans the available detail width above the content area.
2. On a sufficiently wide detail view, the combined description and technical context appear on the left and metadata on the right.
3. Metadata includes status, stage, priority, project, assignee, creator, sprint, team, macro, due date and labels, subject to existing availability rules.
4. Pull requests follow the entire content and metadata area.
5. When space is narrow, the reading order is title, description/context, metadata, pull requests.
6. Editing, rewriting, saving, lookup selection and pull request management retain their existing behavior in panel and modal modes.

## Acceptance scenarios

- Given a wide panel or modal, when the Story tab opens, then the title spans both columns, content starts directly below it on the left, and metadata occupies the right column.
- Given long description content or many metadata values, when reading the detail, then pull requests appear below both columns rather than between content and metadata.
- Given a narrow viewport, when reading or editing the story, then the sections stack in content-first order without horizontal clipping introduced by the layout.
- Given a Jira story with creator and team, when its detail opens, then both metadata fields retain their existing controls; given a GitHub story without creator, their existing absence rules remain.
- Given existing editable values and pull requests, when editing and saving, then the same values and actions remain available.

## Non-goals

No new fields, split description model, workflow behavior changes, tracker API changes or redesign of other tabs.
