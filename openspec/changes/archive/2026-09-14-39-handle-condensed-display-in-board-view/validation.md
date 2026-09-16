# Validation — issue 39

## Changes
- `web/src/components/TaskCard.tsx`: Compact-only key/menu header and single-line native title button; shared action portal; compact menu entries retain pin, advance, parent and PR operations. Escape restores focus and scrolling inside the menu no longer dismisses it. Existing metadata remains in detailed densities.
- `web/src/locales/translations.ts`: French and English labels for compact menu entries.
- `web/tests/condensed-card.browser.mjs`: isolated Chrome regression harness using real card/styles and mocked context operations. No application dependency or test framework was added. Requires an existing Playwright installation and Chrome; run from web with `node tests/condensed-card.browser.mjs`, or set PLAYWRIGHT_MODULE to an existing Playwright module path.

## Executed results
- `npm run build` in web: exit 0; `2098 modules transformed`; `built in 257ms` on the final build. Vite warns that some chunks exceed 500 kB (main JS about 1,434 kB); no build error.
- `npm run lint` in web: exit 0. Existing warnings concern other components and AppContext (React hooks/effects); no diagnostic for TaskCard, translations or the browser test.
- Targeted oxlint on TaskCard and translations: exit 0, no output.
- `go test ./internal/...`: exit 0. Actual output:

```text
ok  tasks/internal/db         0.839s
ok  tasks/internal/handlers   2.132s
?   tasks/internal/models     [no test files]
ok  tasks/internal/runner     1.191s
ok  tasks/internal/terminal   23.499s
ok  tasks/internal/tracker    0.811s
?   tasks/internal/webui      [no test files]
```

- `go build -o /tmp/sectile-39-bin ./cmd/server`: exit 0, no output.
- Browser regression: exit 0. Actual output:

```text
PASS: compact metadata, ellipsis, keyboard details, action isolation, pin/parent/PR, Escape focus, advance guards and arguments, finished state, activity border, detailed densities, local reference, drag payload.
```

The same script checks menu scrolling at 80%, 100% and 125% zoom with light styling; a screenshot of the rendered compact card was visually inspected. Tests run against controlled fixtures and do not change real tickets.
- `git diff --check`: exit 0.

## Acceptance coverage and boundaries
The browser exercises switching density, full metadata versus condensed content, a long title, local references, keyboard detail opening, Escape focus return, menu pin/parent/PR access, one call per advancement with correct task/auto arguments, busy/finished guards, running activity border and drag identifier payload.

Unchanged integration verified by code inspection: all three BoardView paths use TaskCard; AppContext continues to save/load density; BoardView retains existing column drop handlers, filters and sorting. The harness does not perform live tracker writes, a full settings-save/reload journey or a full cross-column board drop. These operations reuse unchanged existing handlers; no claim of live end-to-end tracker validation is made. No production backend change or migration is required.
