# Tasks

## 1. Removal
- [x] 1.1 Delete `skillDirNames` and its comment from `internal/db/db.go`.
- [x] 1.2 Confirm `ProjectSkillTemplate.DirName` still has callers and stays.

## 2. Gates
- [x] 2.1 `go build ./...` clean.
- [x] 2.2 `go vet ./...` clean.
- [x] 2.3 `go test ./...` green, with any pre-existing failure identified as such.
- [x] 2.4 `openspec validate 119-remove-dead-skilldirnames --strict`.
