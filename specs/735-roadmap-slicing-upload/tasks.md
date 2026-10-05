# Tasks #735 - Roadmap slicing import from an uploaded file

Order matters: the server first, the web on top of it, then docs.

## 1. Shared slicing step (server)

- [ ] 1.1 Extract `sliceFromContent` from `TodosFromSDD` in
  `internal/db/sddslicing.go`; `TodosFromSDD` calls it with its current French
  refusal. Its existing tests pass unchanged.
- [ ] 1.2 Add `SlicingUploadLimit`, `uploadName`, `sourceUnitNameEnglish` and
  `TodosFromUpload` (size, UTF-8 and NUL checks; English refusals; origin
  `imported file: <name>`).
- [ ] 1.3 Tests in `internal/db/sddslicing_test.go`:
  - an uploaded `tasks.md` gives the same lines and source kinds as the
    agent-read import of the same content, and the agent is never called
    (`SetAgentOperations` fails the test if invoked);
  - an uploaded `spec.md` gives its requirements, and a Spec Kit one its user
    stories;
  - existing lines are kept and matched lines keep their story;
  - the origin is `imported file: <base name>`; a path-like or empty name is
    reduced or replaced;
  - empty content, content with no unit, oversized content, invalid UTF-8 and
    a NUL byte are refused in English and leave the saved slicing unchanged.

## 2. Slicing request (server)

- [ ] 2.1 In `internal/handlers/handlers.go`, read the body through
  `http.MaxBytesReader`, add `content` and `fileName`, and route a request
  with `content` to `TodosFromUpload`; refuse it with any source other than
  `tasks` or `spec`.
- [ ] 2.2 Tests in `internal/handlers/macroslicing_test.go`:
  - with `content` and no agent connected, the request answers 200 with the
    macro and `imported file: …` as origin;
  - `content` with `stories`, an unknown source, more than 1 MiB, or a body
    over the reader limit, answers 400 with an English message;
  - a request without `content` keeps today's behaviour (the existing test
    stays green).

## 3. Web client

- [ ] 3.1 `produceMacroSlicing` accepts an optional `{ fileName, content }`
  and sends it; update its type in the context interface.
- [ ] 3.2 Catalog keys `uploadTasksTitle`, `uploadSpecTitle`,
  `uploadTooLarge`, `uploadNotText` in both languages of
  `web/src/locales/planning.ts`.
- [ ] 3.3 In `RoadmapView.tsx`, add the upload icon button and hidden file
  input to the `tasks` and `spec` options, with the browser checks, the shared
  post-import handling and the same disabled expression.
- [ ] 3.4 Browser test `web/tests/roadmap-slicing-upload.browser.mjs`, on the
  `roadmap-framing.browser.mjs` harness:
  - both upload buttons are present with their titles; none for "stories";
  - picking a file with the `tasks.md` upload calls `produceMacroSlicing`
    with `tasks` and the file's name and content; same for `spec`;
  - a file over 1 MiB and a non-UTF-8 file raise the error toast and make no
    call;
  - picking the same file twice calls twice;
  - while an import is pending, both upload buttons are disabled (hold the
    mocked `produceMacroSlicing` until the test releases it).

## 4. Docs

- [ ] 4.1 `CHANGELOG.md`, `[Unreleased]` → `Added`: the Roadmap framing panel
  can produce a macro's slicing from a `tasks.md` or `spec.md` picked on the
  user's machine, without Sectile Desktop (#735).
- [ ] 4.2 `docs/API_AND_DATA_SPEC.md`: one sentence on the slicing request's
  optional `content` and `fileName`.

## Test plan

- `go test ./internal/db/ -run 'Slicing|Decoupe|Upload|Fichier|Source'` and
  `go test ./internal/handlers/ -run Slicing` (outside the sandbox; GOCACHE
  under `$TMPDIR`).
- `go vet ./...` and `gofmt -l` on the touched files.
- In `web/`: `node --test tests/planningCatalog.test.mjs`, `npx tsc -b`,
  `npx oxlint` (with the main checkout's `node_modules` when the worktree has
  none), and `node tests/roadmap-slicing-upload.browser.mjs` plus
  `node tests/roadmap-framing.browser.mjs` (Playwright from
  `desktop/node_modules`, `PLAYWRIGHT_MODULE` set).
- `npx vite build` in `web/`, then restore `internal/webui/.gitkeep` if the
  build removed it.
