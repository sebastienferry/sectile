# Tasks #699 - Animate the conversation status indicator

Ordered checklist. Each group is one commit and leaves the tree buildable and
the tests green.

## 1. Status kind (FR4, FR5, FR6, FR8)

- [ ] T1.1 Add `showStatus(text,kind='idle')` in `desktop/src/conversation.js`,
  setting `status.textContent` and `status.dataset.kind`.
- [ ] T1.2 Rewrite line 142 so the precedence chain returns text and kind
  together (notice/read-only/ready `idle`, question/approval `asking`, busy
  `working`).
- [ ] T1.3 Route lines 160 (`idle`), 167 (`working`), 329 (`idle`),
  351 (`working`) and 359 (`working`) through `showStatus`; check that no
  other `status.textContent=` remains (`grep -n "status.textContent"`).

## 2. Indicator styles (FR1, FR2, FR3, FR7, FR9)

- [ ] T2.1 In `desktop/src/style.css`, after `.conversation-status`: the
  `::before` dot for `working` and `asking`, the `run-state-pulse` animation
  for `working`, the `--waiting` colour for `asking`.
- [ ] T2.2 Reduced-motion override for the working dot.

## 3. Tests (AC1-AC5)

- [ ] T3.1 `desktop/tests/conversation.ui.cjs`: kind and computed
  `::before` animation after a send; kind, colour and still dot at the
  approval and question checkpoints; idle with no dot at `Read-only history`.
- [ ] T3.2 Same file: reduced motion emulated, a working status has
  `animation-name` `none`.
- [ ] T3.3 `desktop/tests/run-folders.ui.cjs`: `Ready` idle, busy working,
  folder notice while busy idle.
- [ ] T3.4 `npx vite build`, restore `webui/.gitkeep`, run
  `npm run test:ui` and `npm test` in `desktop/` (unsandboxed); every existing
  status text assertion passes unchanged.

## 4. Changelog (FR10, AC6)

- [ ] T4.1 `CHANGELOG.md`: a `### Changed` section under `## [Unreleased]`
  with one line for the conversation status indicator (#699).
