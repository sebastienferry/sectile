# Tasks

## 1. Removal
- [ ] 1.1 Delete `skillDirNames` and its comment from `internal/db/db.go`.
- [ ] 1.2 Confirm `ProjectSkillTemplate.DirName` still has callers and stays.

## 2. Gates
- [ ] 2.1 `go build ./...` clean.
- [ ] 2.2 `go vet ./...` clean.
- [ ] 2.3 `go test ./...` green, with any pre-existing failure identified as such.
- [ ] 2.4 `openspec validate 119-remove-dead-skilldirnames --strict`.
