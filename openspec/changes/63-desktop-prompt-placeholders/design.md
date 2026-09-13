# Design

Use the existing configuration response to add optional `githubRepo` and `issueTracker` fields, with project-over-global precedence matching server effective settings. Keep schema version 1 because absent fields have defined fallbacks. Do not expose credentials or server filesystem paths.

Return the fetched task from local preparation and pass a launch context (task, resolved local branch and absolute execution directory) through the shared dispatcher. Build values per launch; never persist expanded commands. Tracker uses lowercase task source, effective tracker, then github. Repository uses effective GitHub repository, then local directory basename.

Expand recognized tokens in a single pass over the original template. Preserve shell argument data in unquoted, single-quoted and double-quoted arguments, including embedded and repeated tokens. Do not recursively process substituted values. Keep unknown tokens literal and the existing required prompt token. This is argument interpolation in trusted shell templates, not a general shell parser or support for placeholders as executable shell syntax.

Rejected: server paths (wrong machine), raw replacement (shell injection), a second project fetch (unnecessary extra request), and expanding saved configuration (stale task data).

Verify shell argument round trips, empty and adversarial values, all task launch routes, metadata precedence, local worktree selection, inheritance and desktop help. Run Go tests/build/vet, web build/tests/lint, desktop build/UI tests and strict OpenSpec validation. No unresolved product requirements.
