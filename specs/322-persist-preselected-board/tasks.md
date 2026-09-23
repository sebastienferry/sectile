# #322 — Implementation checklist

References: [`spec.md`](./spec.md), [`plan.md`](./plan.md).

## 1. Decisions (D1)

- [ ] T1.1 Add `recordedBoardId`, `suggestedBoardId` and `shouldImportBoard` to `web/src/lib/boardColumns.ts`, with the same doc
      comment style as `pickBoardId`.
- [ ] T1.2 Tests in `web/tests/boardColumns.test.mjs`:
      - `recordedBoardId` keeps a listed board, returns `''` for an empty or no longer listed one (FR1, FR6);
      - `suggestedBoardId` returns the default board only while nothing is recorded (FR2);
      - `shouldImportBoard` imports the suggested board when nothing is recorded, imports a different board, skips the recorded
        board and an empty value (FR3, FR4).

## 2. Editor (D2, D3)

- [ ] T2.1 In `BoardColumnsEditor.tsx`, add the `recordedBoard` state; initialise it and `boardId` from `recordedBoardId` when the
      boards load.
- [ ] T2.2 In `handleBoardChange`, guard with `shouldImportBoard(next, recordedBoard)`, set `recordedBoard` from the imported project
      on success, fall back to `recordedBoard` on failure (FR5).
- [ ] T2.3 Render the disabled placeholder while `recordedBoard` is empty and the "(suggéré)" marker on `suggestedBoardId`.

## 3. Verification

- [ ] T3.1 `npm test`, `npm run lint`, `npm run build` in `web/` — all green.
- [ ] T3.2 Re-read the diff against the spec's acceptance scenarios.
