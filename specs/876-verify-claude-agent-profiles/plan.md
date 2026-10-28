# Plan #876 - Verify Claude project agent profiles on an authenticated professional workstation

## Stack and nature

No production code. The implement stage runs a verification and publishes its
evidence. Tooling: Go 1.26 (agent, server, opt-in tests), Node and the
Playwright Electron driver already used by `desktop/tests/*.ui.cjs`, Claude
Code 2.1.309 signed in under the owner's account, `gh` for the comments.

Files committed on `feat/876`: `docs/clarifications/876.md` and the three files
of this directory. Nothing else. Driver scripts, fixtures, screenshots and
transcripts live under `$SANDBOX` and are never committed.

## Environment recorded before anything runs

| Item | Command |
|------|---------|
| OS, architecture | `sw_vers`, `uname -m` |
| Claude Code | `claude --version`; `claude --help \| grep -- --append-system-prompt` |
| Codex | `codex --version` (0.159.0 expected, below the 0.162 minimum) |
| Go | `go version` |
| Commit under test | `git -C "$WT869" rev-parse HEAD` (expected `0d0de5f6807f…`) |
| PR state | `gh pr view 872 --json headRefOid,isDraft,state` |
| Installed app untouched | PIDs of `Sectile.app` and its agent before and after |

If `headRefOid` differs from `0d0de5f6`, the run follows US14: it verifies the
new head and records both commits.

## Layout

```text
$WT869    a git worktree of origin/feat/869 at the commit under test
          (.tasks/worktrees/verify-869, detached; created with
          `git worktree add --detach`, removed at teardown)
$SANDBOX  mktemp -d under $TMPDIR
  server, tasks.db        throwaway Sectile server
  userdata/               Desktop data dir (SECTILE_DESKTOP_DATA_DIR)
  repo/                   disposable repository
  gocache/                GOCACHE for the opt-in tests
  shots/, transcripts/    evidence
  driver.cjs              long-lived Playwright driver
  evidence.md             the comment body, composed before posting
```

`$WT869` is a detached worktree, so no branch is created and nothing can be
pushed to `feat/869` from it.

## Build under test and isolation

Reuse the `ux-audit-desktop` sandbox recipe (`.claude/agents/ux-audit-desktop.md`):

1. Server: `go build -o "$SANDBOX/server" ./cmd/server` from `$WT869`, run
   from `$SANDBOX` under `env -i PATH="$PATH" HOME="$SANDBOX" PORT=<free port>
   DB_PATH="$SANDBOX/tasks.db" SECTILE_SECRET_KEY=<random>`; wait for
   `GET /api/version`. Never port 8090, never the repository `tasks.db`, no
   tracker token.
2. Agent and Desktop: from `$WT869`, `go build -o bin/agent ./cmd/agent`, copy
   it to `desktop/bin/sectile-agent` (the Makefile's `DESKTOP_AGENT`), then
   `cd desktop && npx vite build`. If `desktop/node_modules` is missing,
   symlink the main checkout's and remove the symlink at teardown.
3. Launch Electron through Playwright with
   `SECTILE_DESKTOP_DATA_DIR=$SANDBOX/userdata` and `ELECTRON_RUN_AS_NODE`
   removed. `HOME` stays the owner's: Claude must find its sign-in, and the
   ticket requires the same user and environment as the agent. The data dir
   keeps Desktop's settings and `agent-connection.json` away from the
   installed app's; the agent falls back to a free loopback port when its
   default one is taken (`internal/agent/agent.go:490`).
4. Pair the app with the throwaway server through its setup screen, create a
   project, and map `$SANDBOX/repo` as its primary checkout.
5. Record every spawned PID. If a port is taken, stop and report instead of
   freeing it.

Expected caveat: the built agent and the installed agent both write under the
owner's `~/.claude` (Claude's own session files). That is the Claude CLI's
normal behaviour and is not a Sectile setting; nothing in Sectile's provider
configuration is written (never press **Update provider configuration**).

## Disposable repository

```text
$SANDBOX/repo
  CLAUDE.md   "For a profile verification request, include REPO_TOKEN_876 in
              your reply. Do not use tools."
```

`git init`, no commit, no remote. `verify-a` is created through Desktop
(US4). `verify-b` is written by the driver straight to
`.sectile/agents/verify-b.md` asking for `PROFILE_B_876`, as the profile guide
says owners maintain profiles with their editor.

The request sent in every session:
"This is a profile verification request. Reply briefly with every token your
repository and profile instructions require. Do not use tools."

## Driving the views

- **Dialog** (US3, US4, US10): selectors from
  `desktop/tests/agent-profiles.ui.cjs`: `Start project prompt for <project>`,
  the **Project prompt** menu item, the `Agent profile` select, `Create
  profile`, `Profile name`, `Reusable agent instructions`, `Save profile`,
  `Open console`.
- **Terminal view** (US5, US7.2, US8, US9): type into the xterm surface with
  `page.keyboard`; read the reply from the rendered terminal rows and from the
  console history the agent exposes to Desktop. Screenshot each reply.
- **Conversation view** (US6, US7.1, US8, US9): send through the view's input;
  read the transcript from the view and from `GET /desktop/conversation?id=`.
- **Reconnect** (US7.2): `page.reload()` on the Desktop window while the
  console runs, then reattach to the same session from the sidebar.
- **Trust prompt**: if Claude shows a folder-trust prompt for `$SANDBOX/repo`,
  the driver pauses and the owner answers it in the Terminal view (decision 3,
  FR4). The pause is recorded in the evidence.
- **Arguments** (US11.2): `ps -o args= -p <claude pid>` for each session,
  sanitized to flag names, to show `--append-system-prompt` is the only
  difference between a profiled and an unprofiled launch.

## Checklist mapping

| Ticket item | Stories | View | Evidence |
|-------------|---------|------|----------|
| 1 Same dialog from + and Project prompt | US3 | Desktop | 2 screenshots |
| 2 Create profile writes only the file | US4 | Desktop | `find`, `git status`, `git log`, server task count |
| 3 Terminal waits for the request | US5.1 | Terminal | screenshot before typing |
| 4 Repository and profile instructions apply | US5.2 | Terminal | transcript excerpt |
| 5 Conversation view, smoke test passes | US6, US2.3 | Conversation | transcript, `go test` output |
| 6 Later turns and reconnect | US7 | both | transcript excerpts |
| 7 Snapshot on edit | US8 | both | excerpts before and after the edit |
| 8 Two profiles, then None | US9 | both | three excerpts |
| 9 Missing or invalid profile fails closed | US10 | Desktop | error screenshot, no new session in the sidebar |
| 10 Defaults, sandbox, folders unchanged | US11 | both | engine/model labels, argv flags, attached folder listing |
| 11 Evidence recorded | US12 | - | the comment itself |

## Evidence template (`$SANDBOX/evidence.md`)

```markdown
## Verification of #869 on an authenticated workstation

- Date, OS and architecture, Claude Code version, Codex version (not verified: below 0.162)
- Commit under test: <sha> (PR #872, draft|merged)
- Build: separate agent and Desktop from that commit, throwaway server, own data dir

### Automated checks
| Command | Outcome | Note |

### Checklist
| # | Item | View | Verdict | Evidence |

### Excerpts
<sanitized transcripts, markers only>

### Failures
<none, or one line each with the link to the report on #869 / PR #872>
```

## Publication

1. Show the evidence to the owner before posting (comments are public).
2. Post it on #876 through Sectile's `add_comment`; post a short link comment
   on #869 the same way.
3. On a failure, post the minimal reproduction on #869 and on PR #872
   (`gh pr comment 872`), and leave #876 open and unsuccessful (US13).
4. Tick the passed checklist items of #876 only (FR7).

## Teardown

Close Electron, stop the agent it started and the throwaway server (recorded
PIDs only), remove the `node_modules` symlink if one was made, remove `$WT869`
with `git worktree remove`, check `git status --porcelain` in the main
checkout and in `feat/876`, and check the installed app's PIDs are unchanged.
`$SANDBOX` is kept until the evidence is posted, then deleted.

## Risks

- **Model variance**: a reply may omit a marker by chance. A missing marker is
  retried once with the same request; a second miss is a `fail`.
- **TUI rendering**: reading xterm rows may truncate long replies. The console
  history read from the agent is the authoritative transcript; the screenshot
  is supporting evidence.
- **Inference cost**: about fifteen short turns on the owner's account.
- **PR #872 moves during the run**: handled by US14.

## Rejected alternatives

- Replacing the installed `Sectile.app` with the #869 build: changes the
  owner's daily tool and kills the agent hosting Sectile runs (decision 2).
- Owner-driven manual script: rejected for agent-driven Playwright with owner
  intervention only where automation cannot act (decision 3).
- Fixing a regression from this ticket: rejected for reporting on #869
  (decision 4).
