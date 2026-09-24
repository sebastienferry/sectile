- Where to write, and on which branch:
  - `SECTILE_SPEC_REPO` is the checkout to write the specification in, and `SECTILE_SPEC_BRANCH` the branch already checked out there. When `SECTILE_SPEC_WORKTREE` is `true`, it is this macro's own worktree, so another macro specified at the same time cannot touch your files.
  - When they are not set (the skill was invoked by hand), call the `prepare_macro_worktree` MCP tool with the project ID and the macro key, and use the `path` and `branch` it returns.
  - Do NOT create the branch, do NOT switch branch, and do NOT run `git checkout`: switching would carry your untracked files onto another branch. When the answer says `worktree: false`, check that the checkout is on that branch; if it is not, stop and say so.
  - The session's own directory is the code repository, which may not hold the specifications: work from `SECTILE_SPEC_REPO`, never relative to the current directory.
- The macro, read by its key, and its `todos`. That list is the truth to align on:

      curl -sS -H "Authorization: Bearer ${SECTILE_AGENT_TOKEN}" "${SECTILE_SERVER_URL:-http://localhost:8090}/api/projects/${SECTILE_MACRO_PROJECT_ID}/macros" | jq --arg key "${SECTILE_MACRO_KEY}" '.[] | select(.key | ascii_upcase == ($key | ascii_upcase))'

- The origin of each line, which tells you where to look and what to do:
  - `sourceKind` is `tasks` or `spec`: the artefact the line was imported from, and therefore the file to align.
  - `sourceEntry` is the entry's title as the file writes it, before Sectile cleaned it. That is how you find the entry: `text` has lost the group prefix, the story key and the trailing reference.
  - No `sourceKind` means the line was typed by hand. It is an addition, not an orphan.
  - `sourceKind: stories` means the line was taken back from an existing story. It points at a ticket, not at an entry: leave it alone and report it.
  - Any other `sourceKind` is an origin this skill does not know: leave the line alone and report it.
- The change folder of this macro, `openspec/changes/<MACRO-KEY>-<slug>/` in `SECTILE_SPEC_REPO`: its `tasks.md` carries the groups, and its `specs/<capability>/spec.md` the requirements.
- If no such change folder exists on the macro branch, say so and stop: there is nothing to realign, and writing one from the slicing alone would be a specification done badly.