# V1 desktop support matrix (candidate)

This matrix records what the migration branch has demonstrated as of
2026-10-30. It is not a release announcement. A V1 release must name its exact
Tauri and Go commits, publish signed macOS artifacts, and repeat the release
binary checks in the [release checklist](v1-release-checklist.md).

## Platform and delivery

| Target | V1 position | Current evidence and limit |
| --- | --- | --- |
| macOS arm64 | First release target | The latest retained candidate is Tauri `d69c3ac` with clean Go `f55e8768`: a 103.41 MiB app and 56,371,343-byte DMG. App and mounted-DMG service hashes, arm64 architecture, port 52963, 52 license files, `hdiutil verify` and the retained checksum pass. Configured Local native stop-and-quit and conversation resume pass in the separately identified port-52964 qualification. There is no Developer ID/notarization qualification or clean-user installation proof. Historical package results below do not qualify this candidate by implication. |
| macOS x86_64 | Not promised for the first release | No native package or runtime qualification is recorded. |
| Linux | Experimental | A Debian ARM64 package opened a visible window under Xvfb/DBus. Real user desktop, provider and migration workflows across distributions remain unqualified. |
| Windows | Deferred | Go services cross-build, but native desktop startup, migration, process execution and installer behavior remain unqualified. |

## Runtime combinations

| Client and backend | Demonstrated behavior | Boundary for V1 |
| --- | --- | --- |
| Tauri Local with the embedded Go companion | Native Local project/profile screens, folder mapping and the Local board were exercised in isolated macOS bundles. A disposable Local service harness with real Claude Code and Codex confirmed responses, tool activity, interruption, resume and persistence. A native Settings sync using the existing Jira OAuth grant reported 1,988 PE tickets and displayed them on the board. A separate current-source native window created a GitLab source and project, showed a completed 106-ticket synchronization, and displayed the live `taskativ` board. Isolated Go API tests created, updated and read back real issues on the authorized GitHub `sectile` and GitLab `taskativ` repositories; the issues were deleted. Go `e1b80fe8` also passed a live GitLab create/update check with a saved website URL lacking `/api/v4`. | Fresh OAuth consent, Jira writes, a native tracker write workflow, clean-user installation and the signed release path remain open. The native GitLab read used a token supplied to the isolated backend API before opening Settings; live forge writes used an isolated Go API server, not the Tauri window. |
| Tauri Fleet with current Go server and agent | Native pairing, profile update, fake GitLab credential/tracker/project setup, reconnect and persistence passed against a disposable current-Go backend. The candidate’s exact Go `84e19b1b` server/agent binaries also passed isolated Fleet SIGKILL recovery for real Codex and Claude Code: no delayed tool write or replay, with history, context, model and execution mode retained. This was an agent API check, separate from native setup. | A release may claim this pairing only after its exact Tauri/server/agent revisions and real tracker/OAuth workflows are qualified. The isolated fixture does not establish general Fleet support. |
| Tauri Fleet with the existing `660-main` backend | Pairing and authenticated project reads passed. | OAuth setup is unsupported by that backend version. Do not list it as a fully supported V1 Fleet backend. |
| Electron 0.7.5 with current Go server and agent | An isolated window loaded projects and a task board. A tracked task against Go `25a8e77d` displayed real Codex activity; Electron Stop canceled its delegated shell tool and the 90-second marker remained absent after 170 seconds. | Remote tracker mutations from the Electron window and execution against a signed release package remain unqualified. Electron remains a compatibility client; no retirement date is set. |
| Existing MCP clients with the tested current/v0.7.6 server-agent pairings | The binary harness passed its 21-tool contract and read-only STDIO bridge call in both directions. | This does not qualify every MCP client registration, provider-specific consent or remote mutation. |

A later isolated Tauri ticket-editor retake on bundled Go `8d40b2aa` confirmed
native create and edit forms, an existing ticket's ordered PR/MR links, and
an add/remove interaction without saving. Source tests cover the Go write
contracts. No native tracker write was submitted in this retake, so the Local
native write qualification in the table remains open.

Managed native Fleet MCP isolation was added after the retained candidate:
Tauri launch now selects a session-scoped endpoint with Go `9b0b68f6` or a
qualified successor. Real Claude/Codex project-context calls passed through that
agent fix with shared MCP registrations unchanged. The old candidate and
externally attached agents do not acquire this behavior by implication. Native
Fleet execution and the rebuilt package still require qualification.

The isolated native `8d8dd70`/packaged Go `9b0b68f6` retake additionally passed
managed Fleet pairing and automatic agent startup, native folder-only project
creation, real Claude/Codex scoped MCP reads and responses, and Codex desktop
close/reopen with context/model/mode/history retained and no tool replay. A
replacement of that disposable bundle with `d25915f` also preserved Claude’s
transcript prefix, folder, context, model and mode; its native follow-up needed
no replay or new tool call. Its
backend used disposable state on loopback; it does not qualify real remote TLS,
OAuth, tracker chains or a signed installation.

A later native Local free prompt in the isolated QA app confirmed automatic
companion startup and a real Codex `get_project_context` MCP response. It also
exposed a first call without a project identifier, which failed before Codex
used `list_projects` to retry. Go `8d40b2aa` now supplies the process-bound
project for that omitted argument, while preserving explicit targets. A real
STDIO bridge integration test passes; this fix is newer than the retained
Tauri/Go candidate. A later isolated unsigned bundle at Tauri `be93c3c` with Go
`8d40b2aa` passed fresh native Claude and Codex calls to
`get_project_context({})`, returning the selected project without listing
projects first. The same bundle showed both provider cards connected with their
brand marks; the preceding `1982df5` bundle created, renamed and removed a
disposable tracker from native Local Settings. These checks do not qualify the
retained signed-release candidate or remote Fleet.

The controlled Codex service-level chain now reaches Reviewed with standard
installed skills and scoped MCP, after repairing the disposable forge's feedback
and MR-description responses. Three tests, preserved commits, one ready open MR,
matching pushed HEAD and unchanged shared MCP tables were independently checked.
Go `88154064`/`72119df5` add sandboxed automatic review and live Codex trace/error
handling; `85da2f24` adds lifetime ownership and a mandatory initial checkpoint.
Real Codex and Claude headless SIGKILL tests then prevented delayed writes after
95 seconds, restored terminal run identity and did not replay tools. Native
Codex launch/result inspection is deferred by the owner, and these development
checks do not qualify an older retained candidate or a signed package.

The later isolated native Codex retake on Tauri `d69c3ac`/Go `f55e8768`
closes that specific inspection gap. The native ticket button launched the
standard pickup skill with `gpt-6-astra` through scoped companion MCP; the
loopback issue reached Reviewed, two independent unit tests passed, and one
ready open MR matched the pushed worktree HEAD. The desktop displayed the
completed run, reached target and eight committed changed files. Shared
Claude/Codex MCP registrations and fixture main remained unchanged. The QA
companion used a distinct identifier/port and external fixture CLI environment;
real tracker writes, managed-launcher environment propagation and signed
release behavior remain unqualified.

A refreshed unsigned arm64 candidate pairs Tauri `c425821` with clean Go
`85da2f24`. App/DMG resource verification and six packaged migration checkpoint
recoveries passed; the current/v0.7.6 protocol harness also passed against this
Go source. Native install/service replacement and Codex-button/result inspection
are explicitly deferred by the owner. The candidate is not signed/notarized and
has not replaced the regular installation. The prior candidate stays separate.

## Latest candidate and update preparation

Latest retake: canonical candidate `2026-10-30-d69c3ac-gof55e8768` pairs clean
Tauri `d69c3ac` with clean Go `f55e8768`. It includes both idle-conversation
shutdown fixes. A separate native qualification with an isolated identifier,
port 52964 and test versions 0.1.3→0.1.4 passes stop-and-quit, bundle replacement,
Claude/Codex history/context/model/mode resumption, no tool replay and final
native cleanup. The canonical DMG's app/resources/hash verification passes;
its standard port/identifier was not activated against the regular installation.
Jira and signing remain excluded. Earlier candidate limitations below are
historical; do not infer they include these fixes.


The latest source-level v0.7.6/current compatibility refresh uses Go `ba4c512e`.
It passes both agent/server directions, 27 advertised operations, 21 MCP tools
through direct/STDIO clients, run lifecycle, distinct task-spec worktrees and
fake tracker writes. This refresh does not add native Fleet setup, remote TLS
or external OAuth coverage.


Candidate `2026-10-30-d69c3ac` contains Go `ba4c512e` and the configured Local
Services stop-and-quit action. The helper authenticates its Local runtime,
honors busy refusal and proves transport closure plus store ownership release.
A disposable binary replacement from the earlier Go `85da2f24` companion
retained a project, installation identity and encryption key. The replacement
executable's hash matches the one in the retained app. This is service-level
evidence; native rendering/exit, regular-installation activation, signed DMG
installation, tracker/OAuth retention and provider-session upgrade are still
separate qualifications. The initial unpaired stop path and multi-instance
updates remain unqualified slices. No regular installation was changed.

The Tauri `73deeba` source passes 28 Node tests, 151 UI tests, 43 Rust tests,
TypeScript checking and the production frontend build. Its workflow-stage board
was then checked in an isolated unsigned native QA app and documented at
`230bccd`. These source checks do not replace the retained candidate's package
or release qualification. Details and hashes for that candidate are in the
[release checklist](v1-release-checklist.md).

## Product ownership and updates

Interactive Tauri tracker conversations require Go server/agent `920e313f` or
a separately qualified successor: the task launch response must expose the
dispatched `runId`, and the agent must correlate that run with the initiating
desktop connection in either dispatch/response order. Older Fleet servers
without this field do not qualify for this interaction. The isolated native
retake on Tauri `0711eb8` approved real project/ticket reads, run registration
and a clarification comment on GitHub `sectile` issue `#899`. A conversation
follow-up then completed the clarified transition, confirmed through the Go API,
direct GitHub label read and native next-step display. A completed-run restart
attempt was refused before that successful transition. The full chain remains
unqualified; this is not a general tracker-workflow support claim.

The separate Tauri `51e93f2`/Go `84e19b1b` native Claude Code chain reached
Reviewed using standard skills against a loopback GitLab and real Git HTTP
fixture, with a passing unittest and one ready unmerged MR. Retries recovered
from fixture authentication/metadata failures while preserving accepted commits.
Native result and committed-file review were verified. This controlled chain
does not establish real GitLab or Codex full-chain support; those qualifications
remain open.

Tauri is the intended V1 desktop for operational boards, prompts and agentic
execution. The current web app remains the V1 owner of product planning and is
unchanged. Electron stays available as a compatibility client while its
support window is decided separately. Desktop updates use a user-initiated
replacement of a signed and notarized DMG; no in-app updater is planned.

The [migration evidence](sectile-v1-migration-plan.md#stage-5-compatibility-evidence)
contains the source revisions and exact checks behind each row. The release
checklist remains open until the release binaries and external integrations are
qualified.
