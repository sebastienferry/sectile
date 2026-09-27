# Design

The stage report publisher already selects a Markdown heading from the completed
stage before appending the stage note. Replace only the string constants for
`clarified`, `specified`, `implemented`, `reviewed`, and `finished`, preserving
the existing emoji, Sectile attribution, and Markdown formatting.

Tests will exercise the report-publishing path with a non-local tracker and
assert the exact generated headings. This protects the externally visible
comment format without changing unrelated stage behavior.

## Rejected alternatives

- Translating every French runtime string: outside this ticket's explicit
  scope, and it would alter existing product surfaces.
- Moving titles to a localization layer: the five fixed, workflow-owned English
  headings do not justify a new abstraction.
