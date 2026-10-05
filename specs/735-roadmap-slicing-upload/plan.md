# Plan #735 - Roadmap slicing import from an uploaded file

## Stack and constraints

- Server: Go, `internal/handlers` (HTTP), `internal/db` (slicing).
- Web: React + TypeScript, `web/src/components/RoadmapView.tsx`,
  `web/src/context/AppContext.tsx`, catalogs in `web/src/locales/`.
- No migration, no agent protocol change, no new endpoint.
- New user-facing strings are English (clarification round 3); existing
  French messages are left as they are.

## Data contract

`POST /api/projects/{id}/macros/{key}/slicing`, body (JSON):

| Field | Type | Today | Change |
| --- | --- | --- | --- |
| `source` | string | `tasks`, `spec` or `stories` | unchanged |
| `content` | string, optional | absent | the uploaded file's text; when present, the server parses it instead of asking the agent |
| `fileName` | string, optional | absent | the uploaded file's name, used in the origin only |

Response: unchanged, `{ macro, epic, origin }`. For an upload, `origin` is
`imported file: <fileName>` followed by `describeAttached(entries)`.

Refusals: HTTP 400 `{ error }`, as today.

Limits:

- `content` at most `1 << 20` bytes (`db.SlicingUploadLimit`).
- The body is read through `http.MaxBytesReader` with room for JSON escaping
  (`2*SlicingUploadLimit + 64 KiB`). Exceeding it gives the same "exceeds the
  1 MiB limit" refusal as an oversized `content`.
- `content` must be valid UTF-8 (`utf8.ValidString`) and contain no NUL byte.
- `content` with any source other than `tasks` or `spec`, compared without
  case after trimming, is refused. `NormalizeSlicingSource` is not used for
  this check: it falls back on `tasks` and would hide a wrong source.

## Server design

### `internal/db/sddslicing.go`

Split `TodosFromSDD` into the read and the shared step.

```go
// SlicingUploadLimit is the largest file an upload import accepts.
const SlicingUploadLimit = 1 << 20

// sliceFromContent extracts the units of content for source, merges them
// into the macro's slicing, saves it and schedules the tracker mirror.
func (d *DB) sliceFromContent(ctx context.Context, userID string, proj *models.Project,
    macroKey string, source SlicingSource, content string, emptyRefusal func() error,
) (*models.MacroMeta, []SDDEntry, error)
```

- `TodosFromSDD` keeps the project check and the agent call, then calls
  `sliceFromContent` with its current French refusal
  (`"%s ne porte aucune %s : la découpe est laissée telle quelle"`). Its
  behaviour and messages are unchanged.
- New `TodosFromUpload(ctx, userID, projectID, macroKey string, source
  SlicingSource, fileName, content string) (*models.MacroMeta, string,
  error)`:
  - same project and key checks as `TodosFromSDD` (existing French messages
    reused, since they are not new);
  - validates size, UTF-8 and NUL with English refusals;
  - `origin := "imported file: " + uploadName(fileName, source)`;
  - calls `sliceFromContent` with the English refusal
    `"%s has no %s: the slicing is left unchanged"`, the unit being
    `task group` or `requirement or user story` (`sourceUnitNameEnglish`);
  - returns `origin + describeAttached(entries)`.
- `uploadName` keeps the base name only (`path.Base` after replacing `\` with
  `/`), trims it, caps it at 200 runes, and falls back on `sddFileName(source)`
  when it is empty.

### `internal/handlers/handlers.go` (slicing branch)

- The request struct gains `Content *string` and `FileName string`.
- The body is decoded through `http.MaxBytesReader`. A `*http.MaxBytesError`
  is answered with the 1 MiB refusal. Other decoding errors keep today's
  tolerance (an absent body is the default source).
- When `req.Content != nil`:
  - the source must be `tasks` or `spec`, otherwise
    `an uploaded file is sliced as tasks.md or spec.md, not as "<source>"`;
  - call `h.db.TodosFromUpload(r.Context(), h.webSessionUser(r), id, key,
    source, req.FileName, *req.Content)`, like `TodosFromSDD`: the tracker
    mirror gets its acting user from `userID` in `sliceFromContent`
    (`tracker.WithActingUser`), as today;
  - `slicingReadError` is not applied: no agent is involved.
- Otherwise the current routing is unchanged.

## Web design

### `web/src/context/AppContext.tsx`

`produceMacroSlicing(projectId, macroKey, source, upload?: { fileName:
string; content: string })`: when `upload` is given, the body is `{ source,
content, fileName }`. The toasts are unchanged. Update the type in the
context interface (line 423).

### `web/src/components/RoadmapView.tsx` (import row, around line 3413)

- For the `tasks` and `spec` options, render the existing button followed by
  an icon-only button (`Upload` from `lucide-react`, size 12), joined to it
  visually. It has `aria-label` and `title` from the catalog
  (`uploadTasksTitle` / `uploadSpecTitle`), and the same `disabled`
  expression as the other buttons (`slicingSource !== null ||
  slicingLocked`).
- One hidden `<input type="file">` per option, `accept=".md,.markdown,.txt,text/markdown,text/plain"`,
  opened by the icon button through a ref.
- On change:
  1. take `files[0]`, then reset `input.value = ''` so the same file can be
     picked again;
  2. `file.size > 1 << 20` → `addToast` error, title
     `t.operations.notifications.macros.slicingFailed`, description
     `strings.framing.uploadTooLarge`; stop;
  3. `new TextDecoder('utf-8', { fatal: true }).decode(await file.arrayBuffer())`;
     a throw → same toast with `strings.framing.uploadNotText`; stop;
  4. `setSlicingSource(option.source)`, call `produceMacroSlicing(..., {
     fileName: file.name, content })`, `setSlicingSource(null)`, then the
     same `setMacroMeta` / `setSelectedKey` handling as the existing buttons.
- Extract the post-import handling the existing buttons and the upload share
  into one local function, so the two cannot drift.
- `addToast` is already taken from `useApp()` in this component; check it, and
  take it if not.

### Catalog `web/src/locales/planning.ts` (framing section, FR and EN)

| Key | EN | FR |
| --- | --- | --- |
| `uploadTasksTitle` | `Import a local file as tasks.md` | `Importer un fichier local comme tasks.md` |
| `uploadSpecTitle` | `Import a local file as spec.md` | `Importer un fichier local comme spec.md` |
| `uploadTooLarge` | `The file exceeds the 1 MiB limit.` | `Le fichier dépasse la limite de 1 Mio.` |
| `uploadNotText` | `The file is not UTF-8 text.` | `Le fichier n'est pas du texte UTF-8.` |

`web/tests/planningCatalog.test.mjs` checks both catalogs carry the same keys;
the new keys go in both.

## Rejected alternatives

- **A new endpoint** (`.../slicing/upload`): duplicates routing and response
  handling for one optional field.
- **`multipart/form-data`**: the web client sends JSON everywhere; a text
  file under 1 MiB does not need multipart.
- **Inferring the source from the file name**: rejected by the owner (round
  2); the control decides.
- **Reading the file in the agent from a browser-given path**: browsers do not
  expose paths, and the server may be remote.

## Target files

- `internal/db/sddslicing.go`
- `internal/db/sddslicing_test.go`
- `internal/handlers/handlers.go`
- `internal/handlers/macroslicing_test.go`
- `web/src/context/AppContext.tsx`
- `web/src/components/RoadmapView.tsx`
- `web/src/locales/planning.ts`
- `web/tests/roadmap-slicing-upload.browser.mjs` (new)
- `CHANGELOG.md`
- `docs/API_AND_DATA_SPEC.md` (one sentence on the slicing request)
