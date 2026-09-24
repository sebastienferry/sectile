# #432: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Server (FR3)

- [x] T1.1 `CreateStoryFromMacroTodo` returns the created task.
- [x] T1.2 Todo branch of `POST /macros/{key}/story` adds `task` to its response.
- [x] T1.3 Test `CreateStoryFromMacroTodo` returns the task whose key the todo line records.

## 2. Toast (FR1, FR4, FR5, FR6)

- [x] T2.1 `ToastLink` type and `ToastMessage.link`.
- [x] T2.2 `web/src/lib/toastTimer.ts` and `web/tests/toastTimer.test.mjs`.
- [x] T2.3 `ToastItem`: timer with hover/focus pause, link button, tracker icon.
- [x] T2.4 Translations `openCreated`, `openInTracker` (fr, en).

## 3. Creation paths (FR2)

- [x] T3.1 `addToast` default duration via `toastDuration`.
- [x] T3.2 Link on quick add, Roadmap typed story, Roadmap todo story toasts.

## 4. Wrap-up (FR7)

- [x] T4.1 `CHANGELOG.md` `Added` line.
- [x] T4.2 `go build ./... && go test ./internal/...`, `npm test`, `tsc -b`, `oxlint`.
