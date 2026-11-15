# Spec #904 - Desktop (Tauri): a tabbed question form with proposals, a recommended option and per-question skip

Ticket: https://github.com/sebastienferry/sectile/issues/904
Clarification: `docs/clarifications/904.md` (rounds 1 to 3, settled; no open question)
Branch: `feat/904`

## Problem

When an agent running in a native Sectile Desktop conversation needs a decision
from the owner (Claude `AskUserQuestion`, Codex `item/tool/requestUserInput`),
the Tauri app shows every question of the request stacked in one long card.
The owner finds this layout hard to read, cannot tell which proposed answer the
agent recommends, and can only answer every question or skip the whole request.
The answers reach the agent and the conversation transcript, but the run's
activity on the task does not show what was decided.

## Scope

### In

- The native question form of the **Tauri** Desktop app, for Claude
  `AskUserQuestion` and Codex `requestUserInput` requests:
  1. one tab per question;
  2. the proposed answer labelled `(Recommended)` is highlighted;
  3. each question can be skipped on its own; "Skip request" stays;
  4. the answers go back to the waiting agent and are recorded as one step of
     the activity of the run the conversation belongs to.

### Out

- The Electron Desktop app (`desktop/`), which keeps its current form.
- The web app, which has no question form.
- Runs launched in a terminal (remote mode, native terminal): they keep asking
  in the terminal.
- `report_waiting` and its payload; no new MCP tool. #879 stays independent.
- Ticket comments: nothing is posted on the tracker.
- How a skill decides what to ask, and the shape of the questions agents send.
- MCP elicitation forms (`mcpServer/elicitation/request`), which are not
  question requests.

## Vocabulary

- **Question request**: one pending agent call carrying 1 to 32 questions.
- **Question**: a text, an optional `header`, proposed options (label and
  optional description), and the `multiSelect` and `isSecret` flags.
- **Recommended option**: an option whose label ends with `(Recommended)`
  (exact case, surrounding whitespace ignored).
- **Skipped question**: a question the owner chose not to answer; the agent
  receives the literal answer `Skipped by the owner`.
- **Skip request**: the existing action that declines the whole request.
- **Task-linked conversation**: a native conversation opened for a task's run
  (it carries the run ID and the task ID); other conversations are free-standing.

## User stories

### US1 - Answer one question at a time (P1)

As the owner, I want each question of a request on its own tab, so that I read
and answer them one by one and see at a glance which ones remain.

### US2 - See the agent's recommendation (P1)

As the owner, I want the proposed answer the agent recommends to stand out,
so that I can accept it quickly or knowingly choose another.

### US3 - Skip one question (P2)

As the owner, I want to leave one question unanswered without declining the
whole request, so that the agent continues with what I did answer.

### US4 - Keep the decision on the run (P2)

As the owner, I want the answers I gave to appear on the run's activity of the
task, so that the decision is visible after the conversation is closed.

## Functional requirements

### Tabs

- **FR-1** A request with two or more questions shows one tab per question, in
  the order the agent sent them. A request with one question shows no tab
  strip; the question is shown directly.
- **FR-2** A tab is labelled with the question's `header` when it is non-empty
  after trimming, else `Q1`, `Q2`, ... by position. The full question text is
  shown in the tab's panel.
- **FR-3** Each tab shows its state: *to answer*, *answered* (✓) or *skipped*.
  The state is conveyed by text or an accessible name as well as by an icon.
- **FR-4** The first tab is selected when the form appears. The owner can move
  between tabs at any time, with the mouse and with the keyboard (arrow keys
  in the tab strip), and values already entered are kept.
- **FR-5** In a single-choice question (`multiSelect` false, not secret),
  selecting an option marks the question answered and selects the next tab
  automatically. On the last tab, nothing moves. Selecting an option in a
  multiple-choice question, or typing a free answer, never moves the tab.
- **FR-6** There is no review tab. "Send answers" and "Skip request" stay
  visible under the tabs whatever tab is selected.

### Answer controls

- **FR-7** The existing controls are kept: single choice, multiple choice,
  option descriptions, free answer ("Other answer" when options exist), secret
  answers (masked input, no options shown).
- **FR-8** A recommended option is visually highlighted and carries a
  "Recommended" badge; its label is shown without the `(Recommended)` suffix.
  The answer sent for it is the original label, suffix included, so the agent
  reads back exactly what it proposed.
- **FR-9** A recommended option is never preselected. Several recommended
  options in one question are each highlighted. A question without one is
  shown as before.
- **FR-10** Each question offers a "Skip this question" action. A skipped
  question is marked *skipped*, its entered values are cleared, and choosing an
  option or typing an answer in it afterwards makes it *answered* again.
  Skipping a question in a request of two or more moves to the next tab under
  the same rule as FR-5.

### Submission

- **FR-11** "Send answers" is enabled once every question is answered or
  skipped, the total size of questions and answers is within 16 KB, and no
  submission is in flight. Sending with every question skipped is allowed.
- **FR-12** On "Send answers", the agent receives one answer per question:
  the chosen options and free answer joined by `, ` for an answered question,
  `Skipped by the owner` for a skipped one. For Codex, a skipped question is
  sent as that same literal answer, not as an empty answer list.
- **FR-13** "Skip request" keeps its current meaning: the whole request is
  declined (Claude: denied; Codex: empty answers), whatever the tabs' states.
- **FR-14** The existing guarantees hold: no error message echoes a submitted
  value or a provider error; a failed submission clears secret values; a
  request that cannot be read offers "Skip request" only; the form is
  disabled while the agent is unavailable or the conversation is read-only.

### Run activity

- **FR-15** When a task-linked conversation sends answers, the activity of its
  run gains one step summarising them: one line per question, `question →
  answer`, a skipped question showing `Skipped by the owner`, a secret answer
  showing `[hidden]`. This holds for Claude and Codex.
- **FR-16** "Skip request" adds no step to the run's activity. *Open point*:
  the clarification settled the recording of answers only; whether a skipped
  request should also leave a step is not decided. This spec keeps today's
  behaviour (transcript only) and does not block on it.
- **FR-17** A free-standing conversation records nothing beyond its transcript.
  Recording on the activity never blocks or fails the answer: if it cannot be
  recorded, the agent still receives the answers.
- **FR-18** The conversation transcript records secret answers as `[hidden]`
  for Claude as it already does for Codex.

### Language

- **FR-19** New interface strings are in English, the Tauri app's language.

## Acceptance scenarios

1. **Given** a Claude request with three questions with headers "Scope",
   "Tests", "" **When** the form appears **Then** three tabs labelled "Scope",
   "Tests", "Q3" are shown, the first selected, all *to answer*, and "Send
   answers" is disabled.
2. **Given** a request with one question **When** the form appears **Then** no
   tab strip is shown and the question, its options and the actions are.
3. **Given** the first tab holds a single-choice question **When** the owner
   picks an option **Then** the first tab shows *answered* ✓ and the second tab
   is selected.
4. **Given** a multiple-choice question on tab 2 **When** the owner ticks two
   options **Then** tab 2 stays selected and shows *answered*.
5. **Given** a free-text answer is being typed **When** a character is entered
   **Then** the selected tab does not change.
6. **Given** an option labelled `Use SQLite (Recommended)` **When** the form
   appears **Then** it reads "Use SQLite" with a "Recommended" badge, is
   highlighted, and is not selected; **When** the owner picks it and sends
   **Then** the agent receives `Use SQLite (Recommended)`.
7. **Given** a three-question request **When** the owner answers Q1, skips Q2
   and answers Q3 and sends **Then** the agent receives Q2's answer as
   `Skipped by the owner`, and the other two as chosen.
8. **Given** every question is skipped **When** the owner presses "Send
   answers" **Then** the request is answered (not denied) with
   `Skipped by the owner` for every question.
9. **Given** Q2 is skipped **When** the owner picks an option in Q2 **Then** Q2
   shows *answered* and the skip no longer applies.
10. **Given** a request with unanswered questions **When** the owner presses
    "Skip request" **Then** the request is declined as today.
11. **Given** a Codex `requestUserInput` with a secret question **When** the
    owner answers and sends **Then** the agent receives the secret value and
    both the transcript and the run's activity show `[hidden]` for it.
12. **Given** a task-linked Claude conversation **When** the owner sends
    answers **Then** the run's activity on the task shows one new step listing
    each `question → answer`.
13. **Given** a free-standing conversation **When** the owner sends answers
    **Then** no activity is changed and the transcript shows the answers.
14. **Given** the server cannot record the step **When** the owner sends
    answers **Then** the agent still receives them and the form shows
    "Answer sent".
15. **Given** answers totalling more than 16 KB **When** the owner looks at the
    form **Then** "Send answers" is disabled and the 16 KB message is shown.
16. **Given** keyboard focus on the tab strip **When** the owner presses the
    right arrow **Then** the next tab is selected and values are kept.

## Edge cases

- A question whose header duplicates another's: both tabs keep that label;
  position disambiguates them.
- A question with no option: only the free answer is shown; it can be skipped.
- A request withdrawn by the agent while the owner is answering: the form
  disappears as today.
- Codex sends the same question text twice: the request is unreadable as
  today (FR-14).

## Success criteria

- Every acceptance scenario is covered by an automated test (UI tests for the
  form, Go tests for the agent and server paths).
- The Electron app's question form and the web app are unchanged.
- `CHANGELOG.md` carries one `Added` line under `[Unreleased]` for the tabbed
  question form with recommended options and per-question skip.
