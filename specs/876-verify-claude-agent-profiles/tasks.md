# Tasks #876 - Verify Claude project agent profiles on an authenticated professional workstation

Order matters. No step changes runtime code. A failed item does not stop the
run: it is recorded, and the remaining items are still verified.

## 1. Preconditions

- [x] T1.1 Record OS, architecture, Claude, Codex and Go versions; confirm
      `--append-system-prompt` in `claude --help` (US2.2).
- [x] T1.2 `gh pr view 872`: record head and draft state; if the head is not
      `0d0de5f6`, follow US14.
- [x] T1.3 Record the PIDs of the installed `Sectile.app` and its agent.
- [x] T1.4 Create `$SANDBOX` and the detached worktree `$WT869` at the commit
      under test.

## 2. Automated checks (from `$WT869`, `GOCACHE=$SANDBOX/gocache`)

- [x] T2.1 `SECTILE_TEST_PROFILE_PROVIDERS=1 go test ./internal/agent -run
      '^TestInstalledProfileProviders$' -v`; record output, Codex failure
      recorded as expected, never as passed (US2.1).
- [x] T2.2 `SECTILE_TEST_PROFILE_SMOKE=1 go test ./internal/agent -run
      '^TestInstalledProfileConversationSmoke$/^claude$' -v`; record output
      (US2.3).

## 3. Build under test

- [x] T3.1 Build and start the throwaway server; wait for `/api/version`.
- [x] T3.2 Build the agent, copy it to `desktop/bin/sectile-agent`, build
      Desktop.
- [x] T3.3 Create the disposable repository with its `CLAUDE.md`.
- [x] T3.4 Launch Electron through the Playwright driver with its own data
      dir; pair with the server; create the project and map the repository.
- [x] T3.5 Confirm the agent version reported to Desktop matches the commit
      under test (US1.2).

## 4. Desktop checklist

- [x] T4.1 Item 1: open the dialog from **+** and from **Project prompt**
      (US3).
- [x] T4.2 Item 2: create `verify-a`; check file, selection, no launch, no
      commit, no task (US4).
- [x] T4.3 Item 3: open a Claude Terminal session with `verify-a`; screenshot
      the idle prompt; owner answers a trust prompt if one appears (US5.1).
- [x] T4.4 Item 4: send the request; expect both markers (US5.2).
- [x] T4.5 Item 6 (Terminal): reload the window, reattach, send again (US7.2).
- [x] T4.6 Item 5: Conversation session with `verify-a`; expect both markers
      (US6.1).
- [x] T4.7 Item 6 (Conversation): second turn; expect both markers (US7.1).
- [x] T4.8 Item 7: edit `verify-a.md` to `PROFILE_A2_876`; next turn of the
      running session keeps `PROFILE_A_876`; a new session shows
      `PROFILE_A2_876` (US8).
- [x] T4.9 Item 8: write `verify-b.md`; one session with `verify-a`, one with
      `verify-b`, one with **None**; check marker isolation (US9).
- [x] T4.10 Item 9: select a profile, remove its file, confirm the launch is
      refused with an actionable error; repeat with an invalid file (US10).
- [x] T4.11 Item 10: compare engine/model labels and `claude` argv flags of a
      profiled and an unprofiled session; attach a context folder to a
      profiled session and check it is neither searched nor written (US11).

## 5. Evidence and publication

- [x] T5.1 Compose `$SANDBOX/evidence.md` from the template; sanitize (US12.3).
- [x] T5.2 Show it to the owner and get approval before posting.
- [x] T5.3 Post it on #876; post the link comment on #869 (US12.1, US12.2).
- [x] T5.4 On a failure: report on #869 and PR #872 with a minimal
      reproduction; do not report #876 as successful (US13).
- [x] T5.5 Tick the passed checklist items of #876 only (FR7).

## 6. Teardown

- [x] T6.1 Stop the recorded PIDs only; remove the `node_modules` symlink if
      any; `git worktree remove $WT869`.
- [x] T6.2 `git status --porcelain` empty in the main checkout and in
      `feat/876`; installed app PIDs unchanged (AC5).
- [x] T6.3 Delete `$SANDBOX` once the evidence is posted.

## Test plan

There is no new test to write: the opt-in Go tests of #869 (T2) and the
Playwright-driven checks (T4) are the test plan. `make test` on `feat/876` must
still pass, since only documentation changes on this branch.

## Re-verification after the #869 fixes

On PR #872 head `0d0de5f6`, item 9 failed (F2) and F1 forced items 1 and 2
onto the canonical `/private/tmp` path.
Once the fixes merged, T1, T2, T3, T4.1, T4.2, T4.10, T5 and T6 were run again
on `main` at `b8b8d943`, with the repository mapped under `/tmp` to cover F1.
All three items pass; the evidence is the second report on #876.
