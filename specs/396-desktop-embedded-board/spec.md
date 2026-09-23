# #396: The board inside Sectile Desktop (spike)

Clarification: [`docs/clarifications/396.md`](../../docs/clarifications/396.md).
Plan: [`plan.md`](plan.md). Checklist: [`tasks.md`](tasks.md).

A time-boxed spike. It answers one question, *how should Sectile Desktop show the
connected server's board without sending the user to the browser*, and proves the
answer with a prototype. The board shown is the server's existing web board: the
desktop does not draw a board of its own, and nothing works without a server.

## User stories

### US1 (P1): open the board without leaving the desktop

As a workstation user connected to a server, I open the task board from the desktop
and it appears inside Sectile Desktop, already signed in as the user this workstation
is paired to.

1. Given the desktop is connected, when I use the board action (the **Connected**
   link, or the project heading's web action), then the connected server's board
   appears inside Sectile Desktop and no browser is launched.
2. Given the board appears, then it shows the paired user's identity and no sign-in
   screen, whether the server signs people in through OIDC or the local e-mail form.
3. Given the board is already shown, when I use the board action again, then the same
   board comes forward; a second copy is not opened.
4. Given the desktop is not connected, when I use the board action, then nothing opens
   and the same error the desktop shows today is displayed.

### US2 (P1): the board behaves as it does in the browser

As that user, everything I can do on the web board I can do on the board in the
desktop.

1. Given the board is shown, when I move a card to another column, then the move is
   saved and stays after a reload, exactly as on the web.
2. Given the board is shown, when I open a card, then its task detail opens with the
   same content and actions as on the web.
3. Given the board is shown, when I launch a skill from a card, then the launch
   reaches this workstation's agent and its console appears in the desktop's
   execution list, as a launch from the browser does today.
4. Given another user or the tracker changes a task, then the board updates without a
   manual reload, as the web board does.
5. Given the board is shown, when I follow a link that leaves the server (a pull
   request, a GitHub or Jira issue, a documentation link), then it opens in the
   default browser, not inside the desktop.

### US3 (P2): open a task from the desktop

As that user, when I open a task from the desktop's task list, it opens on the board
inside the desktop.

1. Given a task row, when I use its open-task action, then the board inside the
   desktop comes forward with that task's detail open.
2. Given the board was showing another task, when I open a different one, then the
   board shows the new one.

### US4 (P2): the desktop stays as safe as it is

1. Given the board is shown, then the board has no access to the desktop's own
   controls (consoles, settings, repository mappings, stored key) beyond what the web
   board has in a browser.
2. Given the workstation key expires or is revoked while the board is shown, then the
   board shows that the workstation is no longer authorized; it does not fall back to
   another identity and does not offer to paste a key.
3. Given the connected server changes (re-pairing to another server), then the board
   shows the new server, and nothing from the previous one stays signed in.

### US5 (P1): the spike leaves a decision behind

As the project owner, I get a written finding and a proposed decision I can accept or
reject.

1. Given the spike is done, then a finding document in English under `docs/` compares
   the ways of showing the board (where it is shown, how it is signed in, how it is
   isolated), states a recommendation, and lists what the prototype proved and what it
   did not.
2. Given the spike is done, then an ADR draft under `docs/adrs/`, status *Proposed*,
   records the recommended approach and the part of ADR 0003 it amends.
3. Given the spike is done, then the prototype on `feat/396` demonstrates US1 to US4
   well enough to judge the recommendation.

## Functional requirements

- **FR1** The board action and the open-task action show the connected server's board
  inside the desktop, not in the browser.
- **FR2** The board inside the desktop is signed in as the workstation's paired user,
  with no sign-in step.
- **FR3** Every interaction of the web board works unchanged: card moves, task detail,
  skill launch, live updates.
- **FR4** Links leaving the server's origin open in the default browser.
- **FR5** At most one board view exists at a time; the actions bring it forward.
- **FR6** The board cannot reach the desktop's own controls or stored credentials.
- **FR7** An expired or revoked key, a disconnection and a server change each leave the
  board in an explicit, non-authenticated state rather than a stale or foreign one.
- **FR8** The finding document and the ADR draft exist and agree with what the
  prototype does.

## Out of scope

- A board drawn by the desktop itself, or a board that works without a server.
- Changes to the web board, the workflow stages or the tracker adapters.
- Production hardening beyond what judging the recommendation needs; packaging,
  signing and release.
- Removing the browser fallback: whether the board actions keep a way to open the
  browser is for the ADR to propose.

## Open requirements

None from the clarification. The choices it left to the spike (where the board is
shown, how it is signed in, how it is isolated) are what US5 has the spike decide and
record; they do not change these acceptance criteria.
