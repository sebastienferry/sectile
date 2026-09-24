# #432: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Server (FR3)

- [ ] T1.1 `CreateStoryFromMacroTodo` returns the created task.
- [ ] T1.2 Todo branch of `POST /macros/{key}/story` adds `task` to its response.
- [ ] T1.3 Test the response carries `task` and `storyKey`.

## 2. Toast (FR1, FR4, FR5, FR6)

- [ ] T2.1 `ToastLink` type and `ToastMessage.link`.
- [ ] T2.2 `web/src/lib/toastTimer.ts` and `web/tests/toastTimer.test.mjs`.
- [ ] T2.3 `ToastItem`: timer with hover/focus pause, link button, tracker icon.
- [ ] T2.4 Translations `openTask`, `openInTracker` (fr, en).

## 3. Creation paths (FR2)

- [ ] T3.1 `addToast` default duration via `toastDuration`.
- [ ] T3.2 Link on quick add, Roadmap typed story, Roadmap todo story toasts.

## 4. Wrap-up (FR7)

- [ ] T4.1 `CHANGELOG.md` `Added` line.
- [ ] T4.2 `go build ./... && go test ./internal/...`, `npm test`, `tsc -b`, `oxlint`.
