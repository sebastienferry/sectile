# Shared Git worktree permission investigation (#921)

## Initial checkpoint: 2026-11-18

This initial checkpoint is partial investigation evidence; the subsequent
qualification below supersedes its pending next actions. The
accepted specification and branch `feat/921` are preserved. No production code,
launch policy or current-session permissions have changed. No PR or workflow
transition is justified yet.

## Environment and fixture

- macOS 27.2, build 26B123, arm64.
- Codex CLI/app-server 0.164.0; Git 2.50.1 (Apple Git-155).
- Disposable fixture: `.tasks/qualification-921` in the assigned worktree.
  Raw evidence, fixture Git state and the reproduction script are retained there.
- Primary repository: `$FIXTURE/main`; linked worktree: `$FIXTURE/task`,
  branch `feat/task`; common directory: `$FIXTURE/main/.git`;
  per-worktree directory: `$FIXTURE/main/.git/worktrees/task`.
- Synthetic identity `Fixture <fixture@example.invalid>`, no remotes or credentials.
  `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1` isolate Git configuration.
- A seed commit and linked worktree were created. `gc.auto=1` and
  `gc.autoDetach=false` make auto-GC observable; 882 real loose objects include
  three in the `17` bucket. These are reproducer settings, not a proposed remedy.
- Staging occurred during fixture preparation outside the measured sandbox.
  Sandboxed staging has not yet been qualified.

## App-server enforcement probe

The native app-server was launched with Sectile's built-in command shape:
`codex app-server --listen stdio:// -c features.default_mode_request_user_input=true`.
The parent probe was approved outside the enclosing session sandbox to measure
Codex's own native sandbox; the Git command ran under `workspaceWrite`, with
network access disabled. This did not change this session's sandbox.

The probe used `command/exec`, whose installed schema accepts the same
`sandboxPolicy` shape as `turn/start`. It did **not** run a conversation turn,
select the conversation's approval reviewer, or exercise autonomous `codex exec`.
It therefore supplies initial enforcement evidence only, not completion of T2/T3.
Companion MCP flags are immaterial to this standalone command probe and were not
attached; full launch integration remains to be qualified.

Requested policy, matching the current `startCodexTurn` roots for this fixture:

```json
{
  "type": "workspaceWrite",
  "writableRoots": [
    "$FIXTURE/task",
    "$FIXTURE/main/.git/worktrees/task",
    "$FIXTURE/main/.git/objects",
    "$FIXTURE/main/.git/refs",
    "$FIXTURE/main/.git/logs"
  ],
  "networkAccess": false
}
```

Command: `git commit -m 'fixture change'`, with `GIT_TRACE=1` and
`GIT_TRACE2_EVENT=$FIXTURE/task/trace.jsonl`.

Result: exit code **0**, new commit **2bf64c7** (`fixture change`), adding
`change` with one inserted line. The primary commit succeeded. Auxiliary lock
requests were denied; this is not a failed commit.

Sanitized stderr (fixture prefix replaced with `$FIXTURE`):

```text
03:39:55.044171 exec-cmd.c:145          trace: resolved executable path from Darwin stack: /Library/Developer/CommandLineTools/usr/bin/git
03:39:55.044501 exec-cmd.c:267          trace: resolved executable dir: /Library/Developer/CommandLineTools/usr/bin
03:39:55.046279 git.c:476               trace: built-in: git commit -m 'fixture change'
error: Unable to create '$FIXTURE/main/.git/packed-refs.lock': Operation not permitted
03:39:55.054223 run-command.c:673       trace: run_command: git maintenance run --auto --no-quiet --no-detach
03:39:55.054390 run-command.c:765       trace: start_command: /Library/Developer/CommandLineTools/usr/libexec/git-core/git maintenance run --auto --no-quiet --no-detach
03:39:55.060818 exec-cmd.c:145          trace: resolved executable path from Darwin stack: /Library/Developer/CommandLineTools/usr/libexec/git-core/git
03:39:55.061634 exec-cmd.c:267          trace: resolved executable dir: /Library/Developer/CommandLineTools/usr/libexec/git-core
03:39:55.063756 git.c:476               trace: built-in: git maintenance run --auto --no-quiet --no-detach
03:39:55.065468 run-command.c:673       trace: run_command: git gc --auto --no-quiet --no-detach
03:39:55.065619 run-command.c:765       trace: start_command: /Library/Developer/CommandLineTools/usr/libexec/git-core/git gc --auto --no-quiet --no-detach
03:39:55.073466 exec-cmd.c:145          trace: resolved executable path from Darwin stack: /Library/Developer/CommandLineTools/usr/libexec/git-core/git
03:39:55.073773 exec-cmd.c:267          trace: resolved executable dir: /Library/Developer/CommandLineTools/usr/libexec/git-core
03:39:55.074797 git.c:476               trace: built-in: git gc --auto --no-quiet --no-detach
Auto packing the repository for optimum performance.
See "git help gc" for manual housekeeping.
fatal: Unable to create '$FIXTURE/main/.git/gc.pid.lock': Operation not permitted
error: task 'gc' failed
```

Relevant Trace2 events (same path substitution):

```jsonl
{"event": "start", "sid": "20261118T023955.044936Z-H4f93842b-P000010e5", "thread": "main", "time": "2026-11-18T02:39:55.045231Z", "file": "common-init.c", "line": 58, "t_abs": 0.001116, "argv": ["/Library/Developer/CommandLineTools/usr/bin/git", "commit", "-m", "fixture change"]}
{"event": "cmd_name", "sid": "20261118T023955.044936Z-H4f93842b-P000010e5", "thread": "main", "time": "2026-11-18T02:39:55.046363Z", "file": "git.c", "line": 477, "name": "commit", "hierarchy": "commit"}
{"event": "error", "sid": "20261118T023955.044936Z-H4f93842b-P000010e5", "thread": "main", "time": "2026-11-18T02:39:55.054178Z", "file": "usage.c", "line": 94, "msg": "Unable to create '$FIXTURE/main/.git/packed-refs.lock': Operation not permitted", "fmt": "%s"}
{"event": "child_start", "sid": "20261118T023955.044936Z-H4f93842b-P000010e5", "thread": "main", "time": "2026-11-18T02:39:55.054212Z", "file": "run-command.c", "line": 740, "child_id": 0, "child_class": "?", "use_shell": false, "argv": ["git", "maintenance", "run", "--auto", "--no-quiet", "--no-detach"]}
{"event": "start", "sid": "20261118T023955.044936Z-H4f93842b-P000010e5/20261118T023955.062193Z-H4f93842b-P000010e6", "thread": "main", "time": "2026-11-18T02:39:55.062404Z", "file": "common-init.c", "line": 58, "t_abs": 0.001842, "argv": ["/Library/Developer/CommandLineTools/usr/libexec/git-core/git", "maintenance", "run", "--auto", "--no-quiet", "--no-detach"]}
{"event": "cmd_name", "sid": "20261118T023955.044936Z-H4f93842b-P000010e5/20261118T023955.062193Z-H4f93842b-P000010e6", "thread": "main", "time": "2026-11-18T02:39:55.063893Z", "file": "git.c", "line": 477, "name": "maintenance", "hierarchy": "commit/maintenance"}
{"event": "child_start", "sid": "20261118T023955.044936Z-H4f93842b-P000010e5/20261118T023955.062193Z-H4f93842b-P000010e6", "thread": "main", "time": "2026-11-18T02:39:55.065455Z", "file": "run-command.c", "line": 740, "child_id": 0, "child_class": "?", "use_shell": false, "argv": ["git", "gc", "--auto", "--no-quiet", "--no-detach"]}
{"event": "start", "sid": "20261118T023955.044936Z-H4f93842b-P000010e5/20261118T023955.062193Z-H4f93842b-P000010e6/20261118T023955.074144Z-H4f93842b-P000010e7", "thread": "main", "time": "2026-11-18T02:39:55.074384Z", "file": "common-init.c", "line": 58, "t_abs": 0.000969, "argv": ["/Library/Developer/CommandLineTools/usr/libexec/git-core/git", "gc", "--auto", "--no-quiet", "--no-detach"]}
{"event": "cmd_name", "sid": "20261118T023955.044936Z-H4f93842b-P000010e5/20261118T023955.062193Z-H4f93842b-P000010e6/20261118T023955.074144Z-H4f93842b-P000010e7", "thread": "main", "time": "2026-11-18T02:39:55.074900Z", "file": "git.c", "line": 477, "name": "gc", "hierarchy": "commit/maintenance/gc"}
{"event": "error", "sid": "20261118T023955.044936Z-H4f93842b-P000010e5/20261118T023955.062193Z-H4f93842b-P000010e6/20261118T023955.074144Z-H4f93842b-P000010e7", "thread": "main", "time": "2026-11-18T02:39:55.076811Z", "file": "usage.c", "line": 75, "msg": "Unable to create '$FIXTURE/main/.git/gc.pid.lock': Operation not permitted", "fmt": "%s"}
{"event": "exit", "sid": "20261118T023955.044936Z-H4f93842b-P000010e5/20261118T023955.062193Z-H4f93842b-P000010e6/20261118T023955.074144Z-H4f93842b-P000010e7", "thread": "main", "time": "2026-11-18T02:39:55.076842Z", "file": "usage.c", "line": 89, "t_abs": 0.00343, "code": 128}
{"event": "error", "sid": "20261118T023955.044936Z-H4f93842b-P000010e5/20261118T023955.062193Z-H4f93842b-P000010e6", "thread": "main", "time": "2026-11-18T02:39:55.078554Z", "file": "usage.c", "line": 94, "msg": "task 'gc' failed", "fmt": "task '%s' failed"}
{"event": "exit", "sid": "20261118T023955.044936Z-H4f93842b-P000010e5/20261118T023955.062193Z-H4f93842b-P000010e6", "thread": "main", "time": "2026-11-18T02:39:55.078722Z", "file": "git.c", "line": 747, "t_abs": 0.018163, "code": 1}
{"event": "exit", "sid": "20261118T023955.044936Z-H4f93842b-P000010e5", "thread": "main", "time": "2026-11-18T02:39:55.083202Z", "file": "git.c", "line": 747, "t_abs": 0.039088, "code": 0}
```

The first error has the root commit process's SID and precedes its first
`child_start`. Thus `git commit` itself requested `packed-refs.lock`; attribution
to the maintenance subprocess would be incorrect for this reproduction.
The later error belongs to the distinct `commit/maintenance/gc` process and
requests `gc.pid.lock`. The internal ref-backend trigger still needs source-level
investigation; the fixture does not establish which loose-object settings are
necessary for the first error.

The emitted policy alone does not establish general precedence. This probe
establishes only that the specified roots permit the commit's primary write while
these two root-level common Git lock creations remain denied. Protected
hooks/configuration, unrelated paths, alternative supported scopes and effective
policy precedence remain unqualified.

## Stop and resume

The probe was stopped after the new denials under the owner's AGENTS.md rule.
There was no retry, alternative execution route, permission broadening,
maintenance suppression or sandbox bypass after observing the denial.
The earlier owner authorization covered only the optional PATH-alias startup
warning from `codex --version`.

Next action: obtain owner direction for further investigation of this denied
intention, including expected-denial boundary tests and verification of any
provider-supported narrow correction. Then independently qualify actual
conversation turns and autonomous CLI launches, add regression coverage, run
project checks and complete T2-T15 before requesting an implemented transition.

## Primary protocol reference

[Codex app-server command execution](https://learn.chatgpt.com/docs/app-server#command-execution)
documents sandboxed `command/exec`. The installed 0.164.0 JSON schema was also
exported to `/tmp/sectile-921-schema` and inspected; current online documentation
alone is not treated as proof of the installed contract.


## Continued investigation and full-path qualification

The owner subsequently authorized continuing the investigation, including
expected-denial boundary tests and testing a supported narrow correction.
The earlier stop was respected; these are separately authorized probes.

### Lock requester and trigger

The ordinary fixture reproduced the same `packed-refs.lock` denial without
forcing auto-GC (`6a1b90c`, commit exit 0). Git Trace2 attributes the error to
the root commit process, before its maintenance child starts. `GIT_TRACE_REFS=1`
on the corrected full launches shows the HEAD/branch transaction completing,
then the `AUTO_MERGE` deletion transaction, followed by maintenance.

The Git 2.50.1 source establishes the internal call chain:

1. [`cmd_commit`](https://github.com/git/git/blob/v2.50.1/builtin/commit.c)
   calls `sequencer_post_commit_cleanup` after updating HEAD.
2. [`sequencer_post_commit_cleanup`](https://github.com/git/git/blob/v2.50.1/sequencer.c#L2799-L2831)
   unconditionally deletes `AUTO_MERGE`, ignoring that call's return value.
3. [`files_transaction_prepare`](https://github.com/git/git/blob/v2.50.1/refs/files-backend.c)
   queues deletion against the packed backend and locks it before checking
   whether rewriting packed refs is necessary.

This explains why the original primary commit succeeds while an absent cleanup
ref still requires the shared lock. It is not evidence that maintenance requested
that first lock. The separately forced GC fixture requested `gc.pid.lock` in a
maintenance child; general GC bookkeeping is a distinct scope and is not granted
by this correction.

### Supported correction and effective precedence

The corrected scope contains seven paths, all canonicalized from Git's
per-worktree and common directories:

```text
$COMMON/worktrees/task
$COMMON/objects
$COMMON/refs
$COMMON/logs
$COMMON/packed-refs
$COMMON/packed-refs.lock
$COMMON/packed-refs.new
```

The app-server turn also includes its worktree as a writable root. The CLI uses
its cwd as workspace plus one `--add-dir=<path>` for each of those seven paths.
The three files need grants before they exist: commit cleanup creates a lock,
and packing uses a temporary file plus atomic replacement of the packed store.
Pre-existing symlinks or directories at these file paths are excluded.

An exploratory command probe first tested seven extra files including GC
bookkeeping. After isolating the ordinary-commit trigger, the full-path runs
below used only the three packed-reference files, the narrow supported correction.
No whole-common-directory, hooks, config, GC-log, GC-pid or unrelated-root grant
was introduced. Maintenance stays enabled; stderr is not suppressed.

Observed precedence on this installation: the worktree/common Git protection
still blocks hooks/configuration, while the explicit narrower metadata paths
permit their intended writes. A similarly named file (`packed-refs.lock.unrelated`)
remains denied. Thus the tested override is path-specific, not a broad permission
for the common directory. The live results, rather than policy serialization
alone, establish this behavior. Additional managed policies may still constrain
other installations; this is not a claim that Sectile can override admin rules.

### Full launch results

All runs used installed Codex 0.164.0, Git 2.50.1 (Apple Git-155), macOS 27.2
arm64. The configured default model was `gpt-6.1-sol`; no model override was
selected. The installation routes model traffic through a local provider proxy.
Its WebSocket handshake reported HTTP 426 and the provider continued via its
supported HTTP transport; this is separate from filesystem enforcement.
No tool escalation or approval request was accepted in these runs.

| Path / configuration | Result and Git state |
| --- | --- |
| App-server baseline: `thread/start` workspace-write, reviewer `user`, turn with current four metadata directories | Commit `cefb643` succeeded. Cleanup emitted the shared lock denial; subsequent explicitly scripted ref packing failed, making the script exit 128. The final answer distinguished commit success from packing failure. |
| CLI baseline: `codex exec --approve-for-me --json` without metadata flags | Staging failed at the per-worktree `index.lock`, script exit 128; HEAD stayed at seed `05904e3`. Provider process exit was 0, but its answer correctly reported the command failure. |
| App-server corrected: same initialization/reviewer, turn receives seven metadata paths | Staging, commit `3947c32`, cleanup and `pack-refs --all` succeeded; command exit 0, no filesystem denial. |
| CLI corrected: same CLI flags plus seven `--add-dir` paths | Staging, commit `b65ea8c`, cleanup and `pack-refs --all` succeeded; command and provider exit 0, no filesystem denial. |
| App-server corrected plus protected-boundary checks | Commit `3787fee` and packing succeeded; all five negative checks returned expected errno 1, script exit 0. |
| CLI corrected plus protected-boundary checks | Commit `5dca24f` and packing succeeded; all five negative checks returned expected errno 1, script/provider exit 0. |

Read-only verification of all four corrected full-launch fixtures confirmed
`HEAD:change` contains `change`, and the staged diff is empty. The ordinary
fixtures intentionally retain untracked `qualify.sh` and `trace.jsonl`; boundary
fixtures additionally retain untracked `boundary.py`. These evidence files were
not included in the synthetic commits.

Each full launch executed `/bin/sh qualify.sh` once in its disposable worktree.
The script staged only `change`, committed it, packed refs and printed HEAD;
ref traces establish that the branch was resolved from the packed store afterward.
The boundary variant then tried opening five disposable sentinel locations for
write: hooks, config, the metadata sibling, the primary checkout's unrelated
file and a path outside both repositories. All remained denied. The negative
checks treat those denials as expected results and never retry or escalate them.
Shell-tool output combines stdout/stderr; the initial command probes retain
separate streams. Neither combined output nor provider process exit is used
alone to declare a primary operation successful.

### Reproduction artifacts and remaining checks

Retained ignored artifacts under the assigned worktree:

- `.tasks/qualification-921`: initial forced-GC probe, exact policy, Trace2,
  result streams and script.
- `.tasks/qualification-921-candidates`: ordinary-commit baseline, candidate
  scope and individual native command/boundary responses.
- `.tasks/qualification-921-full`: full app-server/CLI requests, fixture scripts,
  JSONL, provider stderr, ref traces and resulting repositories.
- `.tasks/qualification-921-baselines`: full baseline runs and resulting Git state.
- `.tasks/qualification-921-boundaries`: full corrected runs with negative checks.

The full launch driver scripts are preserved beside these fixtures. Runtime
artifacts are intentionally not committed: this document records sanitized,
durable commands, policy, causal attribution, outcomes and platform limits.
The reusable native regression lives in `agent_codex_sandbox_test.go`; its recipe
is in [TESTING.md](../TESTING.md#codex-linked-worktree-sandbox-qualification).

Production regression/build results and final acceptance disposition are appended
after the required checks. The initial Go check could not access the workstation's
normal cache; owner direction was requested before changing that test execution
intention. No build/test success is claimed from that failed setup attempt.

### Disposition and scope limits

This is a supported local configuration correction, not an unfixable upstream
restriction. The app-server was missing packed-reference files; autonomous CLI
also lacked the existing per-worktree/shared-directory scope. The same helper
now feeds both built-in paths. Custom templates and interactive terminal CLI
commands retain their existing policy; read-only turns receive no write grants.
There is no fallback, retry, maintenance suppression, unrestricted mode, persisted
permission setting, schema change or runtime diagnostic for an upstream limitation.
The existing conversation result parser retains a successful command's output and
auxiliary denial without relabeling its exit as failed; regression covers that
case. No new claim is made for Linux, Windows, other CLI versions, custom templates,
all maintenance commands or arbitrary administrator restrictions.


## Build and regression validation

The owner authorized a writable `/tmp` Go cache and scoped local test execution.
The new CLI-template test now explicitly isolates launcher MCP URL settings;
existing command-line tests also require those launcher-only URLs to be unset.
The initial focused invocation compiled but reported four environment-sensitive
assertion failures (one new test, three existing tests) when it inherited the
live companion URL. No production behavior was changed to make those assertions
pass. With the new test isolated and launcher URLs unset:

```text
$ env -u SECTILE_COMPANION_MCP_URL -u SECTILE_DESKTOP_MCP_URL \
    GOCACHE=/tmp/sectile-921-go-cache go test ./internal/agent \
    -run 'TestCodexGitScope|TestCodexAutonomousLaunchSharesGitScope|TestCodexReadOnlyTurn|TestCodexSuccessfulCommand|TestWorktreeGitWritableRoots|TestCodexConversationTurnGrantsWorktree|TestHeadlessCommandLine' -count=1
ok  tasks/internal/agent  4.580s

$ env -u SECTILE_COMPANION_MCP_URL -u SECTILE_DESKTOP_MCP_URL \
    GOCACHE=/tmp/sectile-921-go-cache \
    SECTILE_TEST_CODEX_SANDBOX_ROOT="$PWD/.tasks/codex-sandbox-tests" \
    go test ./internal/agent -run '^TestCodexNativeSandboxLinkedWorktree$' -count=1 -v
=== RUN   TestCodexNativeSandboxLinkedWorktree
--- PASS: TestCodexNativeSandboxLinkedWorktree (1.30s)
PASS
ok  tasks/internal/agent  1.537s

$ GOCACHE=/tmp/sectile-921-go-cache go build -o /tmp/sectile-921-agent ./cmd/agent
(exit 0, no output)

$ env -u SECTILE_COMPANION_MCP_URL -u SECTILE_DESKTOP_MCP_URL \
    GOCACHE=/tmp/sectile-921-go-cache go vet ./...
(exit 0, no output)
```

The full Go race suite is recorded separately after completion. Frontend sources
and dependencies were not changed, so frontend checks are outside the validation
scope. No Windows subprocess launch was introduced or its visibility changed:
Git discovery reuses `gitLocal`, and the native test uses the existing hidden
Codex command builder.


### Baseline environment failures and formatting repair

The first full `go test -race -timeout 60m ./...` run passed the changed agent
package (`ok tasks/internal/agent 221.319s`) but failed in unchanged companion,
database and handler OAuth tests. The session's inherited
`SECTILE_LOCAL_RUNTIME_KEY` enables fixed companion callbacks, conflicting with
the fixtures' synthetic callback URLs. This is environment contamination, not a
new Git permission failure. Baseline `aa932746` was exported to a separate
`/tmp/sectile-921-baseline-*` source copy without changing the assigned checkout.
The same environment reproduced:

```text
$ go test -race ./internal/companion -run '^TestUpgradeRecoversEveryDurableBoundaryAndPreservesIdentity/prepared$' -count=1
--- FAIL: TestUpgradeRecoversEveryDurableBoundaryAndPreservesIdentity (1.34s)
    --- FAIL: TestUpgradeRecoversEveryDurableBoundaryAndPreservesIdentity/prepared (1.34s)
        upgrade_test.go:44: l’URL de rappel du compagnon est définie par le serveur Go
FAIL
FAIL tasks/internal/companion 1.839s

$ go test -race ./internal/db ./internal/handlers -run '^(TestTheForgeOAuthAppConfiguration|TestTheForgeOAuthAppIsAnAdminsAndItsSecretWriteOnly|TestTheOAuthAppConfiguration|TestTheJiraOAuthAppIsAnAdminsAndItsSecretWriteOnly)$' -count=1
FAIL tasks/internal/db       2.287s
FAIL tasks/internal/handlers 2.348s
```

All selected baseline OAuth configuration tests emitted the same fixed-callback
refusal. The relevant baseline/current companion, database and Local runtime
sources were unchanged. The subsequent full run removes inherited `SECTILE_*`
launcher variables from the child environment while preserving deliberately
configured `SECTILE_TEST_*` values. No production OAuth behavior or runtime
setting was changed. Raw full-run logs are retained as
`.tasks/921-go-test.log` (failed contaminated environment) and
`.tasks/921-go-test-clean.log` (isolated environment).

The repository-wide `make fmt-check` also failed identically on the accepted
baseline, naming `internal/agent/agent_provider_login_test.go` and
`internal/agentconfig/engines.go`. Their only repairs remove one trailing blank
line and add one separator blank line, respectively. `make fmt-check` and
`git diff --check` now exit 0 without output. These formatting-only changes
allow the required repository check to pass and change no behavior.

### Acceptance mapping

| Requirement | Evidence / disposition |
| --- | --- |
| FR1 | Full native app-server and autonomous CLI launches qualified independently on macOS. |
| FR2 | Commands, versions, requested roots, ref/process traces, outputs/status and resulting Git state are recorded above and retained in the fixtures. |
| FR3 | Ordinary commit cleanup of `AUTO_MERGE` requests the lock; supported explicit file scopes succeed under native enforcement, establishing a Sectile scope omission. |
| FR4 | Only three packed-reference files supplement the existing metadata directories; hooks/configuration, metadata siblings and unrelated paths remain denied. |
| FR5 | The opt-in native Go regression runs real Git add/commit/pack-refs in a real linked worktree, checks commit/ref/index state and native boundaries, and passed. |
| FR6 | Both paths have a supported local correction. General GC bookkeeping, other platforms/versions, custom templates and administrative restrictions remain explicit limits. |
| FR7 | Testing recipe, capability documentation, launch ADR and English Unreleased entry updated. No installation or persisted configuration change requires a README update. |

Additional memory recommendation: record that ordinary Git commit cleanup of
an absent `AUTO_MERGE` can still lock shared packed refs, and that launched
sessions' runtime/MCP variables must be isolated from ordinary repository tests.


## Final verification result

The isolated full repository race suite completed with exit **0**:

```text
$ go test -race -timeout 60m ./...
?   	tasks	[no test files]
?   	tasks/cmd/agent	[no test files]
?   	tasks/cmd/companion	[no test files]
?   	tasks/cmd/render-skills	[no test files]
?   	tasks/cmd/sectile-migrate	[no test files]
?   	tasks/cmd/sectile-plugin	[no test files]
ok  	tasks/cmd/server	(cached)
ok  	tasks/internal/agent	211.503s
ok  	tasks/internal/agentattach	1.452s
ok  	tasks/internal/agentconfig	(cached)
ok  	tasks/internal/agentexec	(cached)
?   	tasks/internal/agenthttp	[no test files]
ok  	tasks/internal/agentmcp	1.274s
ok  	tasks/internal/agentprotocol	(cached)
ok  	tasks/internal/atlassian	(cached)
?   	tasks/internal/atlassian/atlassiantest	[no test files]
ok  	tasks/internal/auth	(cached)
ok  	tasks/internal/backend	(cached)
ok  	tasks/internal/companion	14.610s
ok  	tasks/internal/db	339.552s
ok  	tasks/internal/forgeoauth	(cached)
?   	tasks/internal/forgeoauth/forgeoauthtest	[no test files]
ok  	tasks/internal/handlers	83.921s
ok  	tasks/internal/httpgzip	(cached)
ok  	tasks/internal/localruntime	1.401s
?   	tasks/internal/mcptest	[no test files]
ok  	tasks/internal/metrics	(cached)
ok  	tasks/internal/models	(cached)
ok  	tasks/internal/runner	(cached)
ok  	tasks/internal/sddfiles	(cached)
ok  	tasks/internal/secrets	(cached)
ok  	tasks/internal/skills	(cached)
ok  	tasks/internal/storelock	(cached)
ok  	tasks/internal/taskmcp	(cached)
ok  	tasks/internal/terminal	(cached)
?   	tasks/internal/testhome	[no test files]
?   	tasks/internal/testsqlite	[no test files]
ok  	tasks/internal/tracker	(cached)
ok  	tasks/internal/trackerapi	(cached)
ok  	tasks/internal/version	(cached)
ok  	tasks/internal/webui	(cached)
ok  	tasks/internal/workspace	(cached)
```

This command ran in a child environment with launcher-only `SECTILE_*` values
removed, intentionally configured `SECTILE_TEST_*` values preserved, and
`GOCACHE=/tmp/sectile-921-go-cache`. The earlier failing run and baseline
reproductions remain documented above. No failures remain in the isolated
suite. Environment-dependent opt-in tests retain their normal skips; the native
Codex opt-in regression was executed and passed separately. No frontend sources
changed. The only changes after Go execution began were documentation and the
two formatting-only blank-line repairs described above.

All FR1-FR7 requirements have an evidenced disposition. Both built-in paths
receive the supported local correction. There is no upstream-only completion
claim and no new platform guarantee beyond the recorded macOS qualification.
