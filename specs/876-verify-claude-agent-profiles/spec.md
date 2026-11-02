# Spec #876 - Verify Claude project agent profiles on an authenticated professional workstation

Ticket: https://github.com/sebastienferry/sectile/issues/876
Clarification: `docs/clarifications/876.md` (rounds 1 and 2, settled)
Branch: `feat/876`
Verifies: #869, draft PR #872 (`feat/869`, head `0d0de5f6807f73e5fa91c8bf73c7c56747d1264f`)

## Problem

#869 lets a repository own supplemental agent instructions
(`.sectile/agents/<id>.md`) and select one when a Claude Code or Codex session
is opened from Desktop. Its automated tests cover storage, the HTTP API, argv
delivery, shell quoting and the Desktop dialog against a mock agent. None of
them proves that a signed-in Claude Code actually follows the profile next to
the repository `CLAUDE.md`, in the real Terminal and Conversation views, nor
that the snapshot, isolation and fail-closed promises hold end to end. #869
deferred that proof to a workstation where Claude Code is signed in. This
workstation is one.

## Nature of the work

This ticket is a verification, not a feature. It adds no runtime behaviour,
changes no default and pushes nothing to `feat/869`. Its deliverable is
evidence: a verdict per checklist item, published on #876 and linked from #869.
The only files it commits are its clarification and this specification.

## Scope

In:

- The #869 implementation at PR #872 head `0d0de5f6`, built into a separate
  local agent and Desktop (decision 2).
- Built-in Claude Code sessions opened from a mapped project, in the
  Terminal view and in the Conversation view.
- The two opt-in Go checks shipped with #869, run from a `feat/869` checkout.
- The eleven items of the ticket's acceptance checklist.

Out:

- Any change to runtime code, to `feat/869` or to PR #872 (decision 4).
- Codex beyond recording its installed version: Codex CLI 0.159 is below the
  0.162 minimum of #869 and is not upgraded.
- The Desktop `aiEngine` defaults (#874).
- Any change to sandbox, approval or permission settings, and any flag that
  weakens them.
- The installed `Sectile.app` and the agent hosting Sectile runs: neither is
  replaced, restarted nor used for a checklist item.
- Copying credentials between devices.

## Vocabulary

- **Build under test**: the agent and Desktop built from PR #872 head
  `0d0de5f6`, launched with their own data directory against a throwaway
  Sectile server.
- **Disposable repository**: a fresh `git init` under `$TMPDIR`, mapped as the
  primary checkout of a throwaway project, never committed to nor pushed.
- **Repository marker**: a distinctive token that the repository `CLAUDE.md`
  asks the agent to include in replies (`REPO_TOKEN_876`).
- **Profile marker**: a distinctive token a profile asks the agent to include
  (`PROFILE_A_876`, `PROFILE_B_876`, and `PROFILE_A2_876` after the edit).
- **Verdict**: `pass`, `fail` or `not verified`, with the evidence it rests on.

## User stories

### US1 - Run the build under test without touching the installed app (P1)

- **US1.1** Given the owner's workstation, when the verification starts, then
  the agent and Desktop built from `0d0de5f6` run with their own data
  directory and their own port, against a throwaway server, and the installed
  `Sectile.app`, its agent and the agent hosting Sectile runs keep running
  unchanged.
- **US1.2** The evidence names the commit actually built (`git rev-parse HEAD`
  of the `feat/869` checkout) and the agent's reported version.
- **US1.3** When the verification ends, every process it started is stopped,
  only those, and no tracked file of either checkout is modified.

### US2 - Automated checks (P1)

- **US2.1** Given the `feat/869` checkout and a writable `GOCACHE`, when
  `SECTILE_TEST_PROFILE_PROVIDERS=1 go test ./internal/agent -run
  '^TestInstalledProfileProviders$' -v` runs, then its output is recorded
  verbatim (sanitized). Its Codex half is expected to fail with "Agent profiles
  require Codex CLI 0.162 or newer"; that failure is recorded as expected and
  the combined command is never reported as passed.
- **US2.2** The Claude interface is checked on its own: `claude --version` is
  recorded and `claude --help` lists `--append-system-prompt`.
- **US2.3** When `SECTILE_TEST_PROFILE_SMOKE=1 go test ./internal/agent -run
  '^TestInstalledProfileConversationSmoke$/^claude$' -v` runs, then it passes,
  and its output is recorded.
- **US2.4** A sandbox or policy denial met by either command is answered
  through the supported approval mechanism or reported; it is never bypassed.

### US3 - Same dialog from both entry points (P1, checklist item 1)

- **US3.1** Given the disposable repository mapped to a project, when the
  project row's **+** is pressed, and then when its **Project prompt** menu
  item is chosen, then the same session dialog opens both times, titled for
  that project, with an **Agent profile** selector.

### US4 - Create a profile from Desktop (P1, item 2)

- **US4.1** Given the dialog, when **Create profile** is used with the name
  `verify-a` and instructions asking for `PROFILE_A_876`, and **Save profile**
  is pressed, then exactly one file appears, `.sectile/agents/verify-a.md` in
  the disposable repository, with the instructions as its whole content.
- **US4.2** After the save, `verify-a` is selected in the **Agent profile**
  selector.
- **US4.3** The save launches no session, makes no commit (`git log` empty,
  `git status` shows only the new untracked file), pushes nothing, and creates
  no task on the throwaway server.

### US5 - Terminal view with a profile (P1, items 3 and 4)

- **US5.1** Given `verify-a` selected and the built-in Claude Code engine, when
  the session opens in the Terminal view, then Claude waits at its prompt: no
  request was submitted on the user's behalf.
- **US5.2** When a harmless request asks for the tokens the instructions
  require, then the reply contains both `REPO_TOKEN_876` and `PROFILE_A_876`.

### US6 - Conversation view with a profile (P1, item 5)

- **US6.1** Given `verify-a` selected and the Conversation view, when the same
  harmless request is sent, then the reply contains both markers.
- **US6.2** The opt-in conversation smoke of US2.3 passes.

### US7 - Later turns and reconnection keep the instructions (P1, item 6)

- **US7.1** In the Conversation view, a second turn (which resumes the Claude
  session with a new process) still yields both markers.
- **US7.2** In the Terminal view, after Desktop reconnects to the running
  console (window reload), a further request still yields both markers.

### US8 - Snapshot on edit (P1, item 7)

- **US8.1** Given a running `verify-a` session, when
  `.sectile/agents/verify-a.md` is changed on disk to ask for `PROFILE_A2_876`,
  then the next turn of that session still yields `PROFILE_A_876` and not
  `PROFILE_A2_876`.
- **US8.2** A new session opened with `verify-a` afterwards yields
  `PROFILE_A2_876` and not `PROFILE_A_876`.

### US9 - Isolation between sessions (P1, item 8)

- **US9.1** Given a second profile `verify-b` asking for `PROFILE_B_876`, when
  one session runs with `verify-a` and another with `verify-b`, then each reply
  carries its own profile marker only, and both carry `REPO_TOKEN_876`.
- **US9.2** A session opened with **None** carries `REPO_TOKEN_876` and no
  profile marker.

### US10 - Fail closed on a missing or invalid profile (P1, item 9)

- **US10.1** Given a profile selected in the dialog, when its file is removed
  before the launch is confirmed, then the launch is refused with an error
  that says what to do, and no session starts without the profile.
- **US10.2** The same holds when the file is made invalid instead (emptied, or
  holding a NUL byte).

### US11 - Nothing else changes (P1, item 10)

- **US11.1** The engine and model shown for a profiled session are the ones
  the engine selects without a profile.
- **US11.2** Claude's sandbox and approval configuration for the session is
  the one of an unprofiled session: same settings files, no added flag beyond
  `--append-system-prompt`.
- **US11.3** A folder attached as context to a profiled session stays attached
  and is not searched for profiles nor written.

### US12 - Evidence (P1, item 11 and completion)

- **US12.1** One comment on #876 records: OS and architecture, Claude Code
  version, Codex version and why it was not verified, tested commit, the
  command lines and their outcomes, one row per checklist item with its view,
  verdict and evidence, and sanitized excerpts.
- **US12.2** One short comment on #869 links that evidence.
- **US12.3** Excerpts carry markers, file names and outcomes only: no token,
  key, account identifier, e-mail address or professional data. Screenshots are
  cropped or omitted when they would show any of these.

### US13 - On a failure (P1, decision 4)

- **US13.1** When an item fails, it is reported on #869 and on PR #872 with a
  minimal reproduction and its evidence.
- **US13.2** #876 is then not reported as successful: its evidence comment says
  which items failed, and the ticket waits for the fix before the failed items
  are verified again.
- **US13.3** Nothing is pushed to `feat/869` from this ticket.

### US14 - A new head on PR #872 (P2, decision 1)

- **US14.1** When PR #872 receives commits after `0d0de5f6` and before it
  merges, the automated checks of US2 are run again on the new head, and only
  the Desktop items whose code the new diff touches are run again. The
  evidence names both commits and which items were re-run.

## Functional requirements

- **FR1** No runtime source file changes on `feat/876` or `feat/869`.
- **FR2** No process the verification did not start is stopped or restarted.
- **FR3** No prompt or test output contains a secret; markers are the only
  distinctive content.
- **FR4** No sandbox, approval or permission setting is changed, and no flag
  that skips approval or trust prompts is used. A Claude folder-trust prompt is
  answered by the owner (decision 3).
- **FR5** A checklist item is `pass` only with observed evidence. An item that
  could not be run is `not verified`, with the reason; it is never `pass`.
- **FR6** The interactive items are driven by the agent through Playwright;
  the owner intervenes only for what cannot be automated and confirms it.
- **FR7** The ticket's checklist is ticked on #876 only for the items that
  passed.

## Acceptance criteria

- **AC1** The evidence comment exists on #876 with every field of US12.1 and a
  verdict for each of the eleven checklist items.
- **AC2** A comment on #869 links it.
- **AC3** US2.3 passed, or its failure is reported per US13.
- **AC4** Every Desktop item (US3 to US11) has a verdict backed by a
  transcript excerpt, a file listing or a screenshot.
- **AC5** After teardown, the installed `Sectile.app` and its agent still run,
  `git status --porcelain` is empty in both checkouts, and no process started
  by the verification remains.
- **AC6** No `CHANGELOG.md` entry: nothing a user can see changes.

## Open requirements

None. The four product questions were settled in round 2; the verification is
not blocked on any of them.
