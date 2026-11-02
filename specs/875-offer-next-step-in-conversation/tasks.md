# Tasks #875 - Offer the next step in a task conversation

Order matters: each step leaves the tree buildable and `cd desktop && npm
test` green. Tests go with the step they cover. One commit per step.

## 1. Eligibility

- [x] T1.1 `desktop/src/next-step-offer.mjs`: `nextStepOffer`, `blocksLaunch`
      (FR1, FR3; plan §1).
- Tests: `desktop/tests/next-step-offer.test.mjs`, one case per rule:
  - offered: completed Specify at `specified` -> `Next: Implement` (AC1);
  - Implement at `implemented` without / with `prUrl` -> Create PR / Adjust
    (AC2, US2.1);
  - "Needs verification" (reported, stage not reached) -> none (AC3, US1.2);
  - failed, canceled, unconfirmed -> none (US1.3);
  - busy, approvals, `AskUserQuestion`, `waitingSince`, read-only -> none
    (AC4, US1.4, US4.3);
  - successor running / successor completed with its stage (US1.5);
  - handoff completed, task finished -> none (AC6, US2.2);
  - project not configured, skill not offered -> none (US2.3);
  - another active run, submitting -> none (US2.4);
  - providers `claude`, `codex`, `agy` -> offered (AC9);
  - free console, macro run, `discuss`, terminal run -> none (AC7);
  - `blocksLaunch` ignores the predecessor only.

## 2. Conversation view

- [x] T2.1 `conversation.js`: `onState` after each poll; `showOffer`,
      `offerState`; offer removed on `select` (FR2; plan §2).
- [x] T2.2 `style.css`: `.conversation-offer`.
- Tests: covered by step 3's UI test.

## 3. Launch and handover

- [x] T3.1 `main.js`: compute the offer in `renderNextStep` and on
      `onState`; toolbar code untouched (US4.1).
- [x] T3.2 `launchTaskWork(kind, force, {predecessor})`: guard with
      `blocksLaunch`, freshness re-check of the offer, `'conversation'` view
      (FR3, FR5, US3.1, US3.3).
- [x] T3.3 Handover: select the successor, then `api.stop(predecessor)`;
      `pendingHandover` for a successor that shows on a later poll; nothing
      stopped on failure or refusal (FR4, US3.4 to US3.6).
- Tests: `desktop/tests/conversation-next-step.ui.cjs` on the fake agent used
  by `conversation.ui.cjs`:
  - offer shows after a completed skill and a stage move, not before (AC1,
    AC3);
  - click launches one run with the next skill in the conversation view,
    selects it, stops the predecessor after the launch, predecessor reopens as
    `Read-only history` (AC1, US3.5);
  - double click launches once (US3.3);
  - server 409 `activeRunId`: predecessor still open and selected, error
    beside the offer, toolbar shows `Launch anyway` (AC5, US3.6);
  - approval pending: no offer (AC4);
  - toolbar `Next:` state identical with and without the offer (AC8).
  - the offer case of AC1 on a Codex and an Antigravity conversation (AC9).
- Existing suites still pass: `concurrent-launch.ui.cjs`,
  `declare-reviewed.ui.cjs`, `conversation.ui.cjs`, `codex-conversation.ui.cjs`,
  `agy-conversation.ui.cjs`, `conversation-mode.ui.cjs`.

## 4. Documentation

- [x] T4.1 `CHANGELOG.md`, `## [Unreleased]` / `### Added`: one line, e.g.
      "Desktop task conversations offer the next workflow step once the
      current one is done and validated: one click starts it in a fresh
      conversation in the same worktree and closes the finished one, which
      stays readable in the sidebar. (#875)" (FR7).
- [x] T4.2 `desktop/README.md`: describe the offer where conversations are
      described, if they are.

## Test plan

- `cd desktop && npm test` (unit), `cd desktop && npm run test:ui` (Electron).
- `make test` before the implemented transition.
- Manual: on a real project, run Specify in a Claude, a Codex and an
  Antigravity conversation; check the offer, the launch, the read-only
  predecessor (for Antigravity, that the skill ends its run and the offer
  shows); check that a launch refused while a terminal run is active keeps the
  predecessor open.
