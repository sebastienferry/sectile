# Tasks #612 - Web card: model menu and copy-prompt button

Ordered checklist. Each group is one commit and leaves the tree buildable.

## 1. Shared prompt builder (FR7)

- [ ] T1.1 Create `web/src/lib/skillPrompt.ts` with `SKILL_COMMANDS`,
  `PICKUP_SKILL`, `PICKUP_COMMAND` and `taskSkillPrompt(command, taskId,
  projectId?)`, returning today's prompt text unchanged.
- [ ] T1.2 `CopyTaskSkillMenu.tsx`: import them, drop the local copies and
  `promptFor`'s template.
- [ ] T1.3 `web/tests/skillPrompt.test.mjs`: exact prompt with and without a
  project id; `SKILL_COMMANDS` covers clarify, specify, implement, adjust and
  handoff.

## 2. Model indicator menu (FR1-FR5, FR9)

- [ ] T2.1 `TaskCard.tsx`: extract `anchoredMenuPosition(anchor, width)` from
  `openMenuAt`; the `(...)` menu uses it with no behaviour change.
- [ ] T2.2 Extract `modelRadioItems()` from the `(...)` sub-list and render it
  there; `chooseModel` also closes the indicator menu.
- [ ] T2.3 Turn the indicator into a menu button when `cardModels.length >
  0`, with the `Cpu` placeholder when no model is known; keep `?` and the
  slot-less label passive.
- [ ] T2.4 Portalled indicator menu: `role="menu"`, focus on open, arrow
  keys, Escape with focus return, outside click, scroll and resize close;
  mutual exclusion with `(...)`; propagation stopped.
- [ ] T2.5 Rework the bottom row's `ml-auto` so the first of indicator / copy
  icon / `>` takes it.
- [ ] T2.6 Strings `chooseModel` in `shell.ts` (fr, en) and its type.

## 3. Copy icon (FR6-FR8, FR9)

- [ ] T3.1 Create `web/src/components/CopyStepPromptButton.tsx`: hidden when
  the stage has no skill, copies `taskSkillPrompt`, toast on success,
  anchored fallback panel on refusal.
- [ ] T3.2 Render it on the full card only, between the indicator and `>`.
- [ ] T3.3 Strings `promptCopied`, `promptCopiedDescription` (fr, en) and
  their types.

## 4. Tests

- [ ] T4.1 `web/tests/card-model-menu.browser.mjs` (real `TaskCard`, mocked
  `useApp`, `engine` reporting a model slot and models), covering:
  - AC1: open the indicator, choose a model; the indicator text and colour
    change, no `advance` call; `>` then calls `advance` with the model;
    choosing the configured entry clears it.
  - AC2: pick from the indicator, check it in `(...)` → "Modèle des
    lancements"; pick from `(...)`, check it in the indicator's menu.
  - AC5: `?` engine and slot-less engine give no button; models with no
    configured model show the chip icon, which opens the menu.
  - AC6: no `setSelectedTask` call from the new controls; Escape returns
    focus to the indicator; condensed card opens the same menu.
  - AC3: stub `navigator.clipboard.writeText`; the icon's text equals the
    `(...)` step entry's text for "new" and "clarified"; `addToast` called;
    no icon on a finished task.
  - AC4: reject `writeText`; the panel shows the prompt; Escape closes it
    and focus is on the icon.
- [ ] T4.2 `web/tests/condensed-card.browser.mjs`: assert the condensed card
  has no copy icon and its `(...)` still lists both copy entries.
- [ ] T4.3 Keep `web/tests/skillLaunchModel.test.mjs` green.

## 5. Changelog (FR11)

- [ ] T5.1 `CHANGELOG.md`, `## [Unreleased]`: one user-facing line, for
  example "On the web board, clicking a card's model now opens the model
  list, and full cards gain a button that copies the next step's prompt for
  an AI engine's desktop app. (#612)".

## 6. Verification

- [ ] T6.1 `npx tsc -b` and `npx oxlint` in `web/` (symlink the main
  checkout's `node_modules` if the worktree has none, then remove it).
- [ ] T6.2 `node --test web/tests/*.test.mjs`.
- [ ] T6.3 The two browser tests with `PLAYWRIGHT_MODULE` from the main
  checkout.
- [ ] T6.4 `npx vite build` in `web/`; restore `webui/.gitkeep` if the build
  deleted it.
