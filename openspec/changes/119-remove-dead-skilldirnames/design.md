# Design

## Context
Change 113 moved skill installation from per-checkout `.taskflow/` directories to the agents'
user-level configuration, and stopped generating `.taskflow/config.json`. `skillDirNames`
(`internal/db/db.go:5529`) built the list of skill directory names that file used to carry.

A repository-wide search for `skillDirNames` returns exactly two hits: the comment and the
declaration. The function is unexported, so no caller outside `internal/db` is possible, and
`go vet` does not flag it — unused package-level functions are not a vet check, only unused
local variables and imports are compile errors.

## Decisions

### Delete rather than keep for a future caller
The `.taskflow/config.json` consumer is gone and is not coming back: user-level installation is
the shipped design as of 113 and 116. A helper kept "just in case" against a retired file is a
false trail, and the four lines it contains are trivial to rewrite should an unrelated need for
a directory-name list ever appear.

### Scope stays at one function
The ticket names one helper, and 113 deliberately deferred it rather than widening its own
diff. A broader dead-code sweep of `db.go` (a 5600-line file) is a different change with a
different risk profile; it is not folded in here.

### No new test
There is no observable behaviour to cover: removing an unreachable function cannot change any
output. The non-regression evidence is that the package still builds, `go vet` stays clean, and
the existing suite stays green — which is exactly what a test asserting "the function is gone"
would not add.

## Rejected alternatives
- **Mark it `//nolint` / keep with an updated comment.** Retains the false trail and still
  documents a file the product does not write.
- **Wire it into a new caller.** Inventing a consumer to justify existing code inverts the
  dependency; no current feature needs this list.
