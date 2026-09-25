# #391: Name an outdated local agent instead of failing on an unknown operation

Ticket: https://github.com/sebastienferry/sectile/issues/391
Branch: `feat/391`.
Clarification: [`docs/clarifications/391.md`](../../docs/clarifications/391.md).

## Context

The server asks the connected local agent to run operations on the
workstation (`git_evidence`, `pr_evidence`, `macro_spec_file`, ...). The agent
is a detached process that outlives the desktop app, and the desktop reuses any
agent already running. When the server is newer than the agent, it asks for an
operation the agent does not know, and the caller receives the agent's raw
`unknown local operation "pr_evidence"`. On 2026-09-23 this blocked every
`transition_stage` carrying a `prUrl` until the agent was restarted, with an
error that named neither the cause nor the fix.

Out of scope: an automatic update channel for the agent binary, a protocol
version bump for existing operations, refusing old agents at connection time,
and any indicator on the web board.

This file states behaviour and acceptance criteria only. Implementation choices
are in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

---

## Decisions being specified

Settled by the owner during the clarification (round 2).

- **D1: Prompt, never restart silently.** When the desktop finds that the
  running agent is not the binary it bundles, it asks, through the existing
  restart confirmation, and shows an "agent outdated" mark until the agent is
  restarted.
- **D2: Generic check.** Every operation the server relays to an agent is
  checked, not only `pr_evidence`.
- **D3: Status, no board UI.** `/api/agent/status` exposes the agent build and
  its outdated state; the web board shows nothing new.
- **D4: SFE-360 is an operational check.** Replaying the SFE-360 transition
  with MR !97 is a manual verification, not an acceptance criterion.

Settled from the code during the clarification (round 1).

- **D5: Capabilities, not versions.** Whether an agent can run an operation is
  decided by the list of operations it announces, never by comparing version
  strings: between two tags every build reports the same version.
- **D6: One error for one situation.** Every "the agent cannot do this
  operation" failure produces the same error, whichever operation and whichever
  caller.
- **D7: Old agents keep working.** An agent that announces nothing is still
  sent operations; its `unknown local operation` reply becomes the same error.
- **D8: Language per surface.** The MCP and API error is English. A message a
  surface shows keeps that surface's language: French for the web app's macro
  slicing import, English for the desktop app.

---

## User stories

### US1: Understand why a transition is refused (P1)

As someone running a Sectile skill, when a stage transition needs an operation
my local agent is too old to run, I want the error to say which agent is
outdated and what to do, so that I restart or update it instead of treating
the failure as an opaque integration bug.

**Acceptance**

1. **Given** an agent that announced its operations without `pr_evidence`,
   **when** `transition_stage` is called with a `prUrl` that needs a pull
   request lookup, **then** the call fails with an English error that names the
   agent's device, the agent's build, the operation `pr_evidence`, and the fix
   "restart or update the Sectile desktop app"; the text contains neither
   `unknown local operation` nor any wording that reads as "no pull request
   found".
2. **Given** an agent built before this change (it announces nothing), **when**
   the same transition is attempted and the agent answers
   `unknown local operation "pr_evidence"`, **then** the call fails with the
   same error, the build being described as unknown.
3. **Given** either agent, **when** the transition fails this way, **then** the
   task stays at its current stage and nothing is sent to the agent a second
   time.
4. **Given** an agent that announces `pr_evidence`, **when** the same
   transition is attempted, **then** it behaves exactly as before this change.

### US2: The same explanation for every operation (P1)

As a user of any feature that relies on the local agent, I want the same
explanation whichever operation is missing, so that a future operation does not
reproduce the incident.

**Acceptance**

1. **Given** an agent whose announcement lacks an operation, **when** the
   server needs that operation for any feature, **then** the request is not
   sent to the agent and the caller receives the error of US1.1 naming that
   operation.
2. **Given** the macro slicing import in the web app and an agent that lacks
   `macro_spec_file` (announced or legacy), **when** the import runs, **then**
   the user sees the French message the import already shows for an outdated
   desktop app ("Votre app desktop Sectile est trop ancienne pour importer la
   découpe : mettez-la à jour puis réessayez.").
3. **Given** a deployment with several server instances, **when** the agent is
   held by another instance than the one serving the request, **then** the
   caller receives the same error as if the agent were held locally.

### US3: See which agent build is connected (P2)

As an operator, I want the server to tell me which build each connected agent
runs and whether it is outdated, so that I can diagnose a workstation without
logging into it.

**Acceptance**

1. **Given** an agent that announced itself, **when** `/api/agent/status` is
   read on the instance holding it, **then** its entry carries the agent's
   version, its commit when known, the operations it announced, and
   `outdated: false` when it supports every operation this server may request,
   `true` otherwise.
2. **Given** a legacy agent, **when** `/api/agent/status` is read on the
   instance holding it, **then** its entry carries no version and no operation
   list, and `outdated: true`.
3. **Given** an agent held by another instance, **when** `/api/agent/status` is
   read, **then** its entry is listed as today, without build fields and
   without an `outdated` value (unknown there, never guessed).
4. The fields added to an entry are additive: an existing client that ignores
   them keeps working.

### US4: The desktop app notices an outdated agent (P1)

As a desktop user, after the desktop app was upgraded or its bundled agent was
rebuilt, I want the app to tell me the running agent is not the one it ships
and offer to restart it, so that I do not keep running an old agent without
knowing.

**Acceptance**

1. **Given** a running agent started from a binary whose content differs from
   the binary the desktop app bundles, including a rebuild that reports the
   same version, **when** the desktop app connects to it (at launch or on
   reconnection), **then** it opens the existing restart confirmation, with a
   line saying the running agent is not the bundled one, and the active
   executions it would stop listed as today.
2. **Given** that confirmation, **when** the user confirms, **then** the
   running agent is stopped and the bundled binary is started in its place,
   even when the running agent had been started from another path; the
   outdated mark disappears once the new agent answers.
3. **Given** that confirmation, **when** the user cancels, **then** nothing is
   restarted, the settings panel shows an "outdated" mark next to the local
   agent's version, and the confirmation is not reopened for the same running
   agent during that desktop session.
4. **Given** a legacy agent (it cannot report which binary it runs), **when**
   the desktop app connects to it, **then** it is treated as outdated, as in
   US4.1.
5. **Given** a running agent that is the bundled binary, **when** the desktop
   app connects, **then** no confirmation opens and no mark is shown.
6. The desktop app never restarts the agent without the user's confirmation.

---

## Functional requirements

- **FR1** On connection, the agent announces its build (version, and commit
  when known) and the list of operations it can run. The list is the one the
  agent actually dispatches on, so the two cannot differ.
- **FR2** The announcement is additive on the wire: a server that predates it
  accepts the connection unchanged, and a legacy agent is accepted by a new
  server.
- **FR3** The server keeps the announcement with the connection for as long as
  it lives; nothing is persisted.
- **FR4** Before relaying an operation to an agent that announced its list, the
  server refuses an operation missing from it with the error of FR6, without
  sending anything to the agent.
- **FR5** For an agent that announced nothing, the server relays as today and
  converts a reply of exactly `unknown local operation "<the operation sent>"`
  into the error of FR6. Any other reply is left as it is.
- **FR6** The error is one typed error, recognisable by every caller without
  reading its text, whose English message names the device, the build ("unknown
  build" for a legacy agent), the operation and the fix: restart or update the
  Sectile desktop app.
- **FR7** The error keeps its type and its message when it crosses from the
  instance holding the agent to the instance serving the request.
- **FR8** A pull request lookup that fails with this error is reported as a
  failed lookup, never as the absence of a pull request, as today.
- **FR9** The macro slicing import recognises the error by its type, not by
  matching its text, and keeps its French message. The foreign-repository
  "agent too old" case of the pull request lookup is reported through the same
  type, with the same fix.
- **FR10** `/api/agent/status` exposes, for each connection held by the
  answering instance, the agent version, commit, announced operations and an
  `outdated` flag, as described in US3.
- **FR11** The agent's local desktop interface reports an identity of the
  binary it was started from that changes whenever the binary's content
  changes, captured when the agent starts.
- **FR12** The desktop app compares that identity with the one of the binary it
  would start, on every successful connection to an agent, and applies US4.
- **FR13** `CHANGELOG.md` gets one `Fixed` line under `[Unreleased]`.

## Edge cases

- An agent that announces an empty list is not legacy: every operation is
  refused with the error of FR6.
- An operation name the server does not know is not refused by the server; the
  agent's answer stands.
- Skill launches and terminal openings are not operations in this sense (they
  travel as their own messages, which every agent knows); they are not checked.
- An agent rebinding the same slot replaces the previous announcement with its
  own.
- A desktop app whose bundled binary cannot be read (missing file) shows no
  prompt and no mark; the start path already reports a missing binary.

## Acceptance criteria

1. The agent announces its build and operations at connection; the server keeps
   them on the connection and `/api/agent/status` exposes them with the
   `outdated` flag (US3, FR1-FR3, FR10).
2. An operation the agent lacks is refused, before relaying for an announcing
   agent and after its reply for a legacy one, with one typed English error
   naming device, build, operation and fix, never `unknown local operation`
   (US1, US2.1, FR4-FR7).
3. `transition_stage` with a `prUrl` on such an agent fails with that message;
   the macro slicing import keeps its French message, driven by the error type
   (US1, US2.2, FR8, FR9).
4. The desktop app detects that the running agent is not its bundled binary,
   same-version rebuild included, prompts through the existing restart
   confirmation, and shows an outdated mark in its settings until restarted
   (US4, FR11, FR12).
5. Tests cover `pr_evidence` against an announcing agent without it and a
   legacy agent, assert the wording, cover the announcement at connect, and the
   forwarding between instances.
6. `CHANGELOG.md` has one `Fixed` line under `[Unreleased]` (FR13).

## Manual verification (not an acceptance criterion)

Restart the agent of the workstation that holds SFE-360, then replay its
`implemented` transition with the MR !97 `prUrl`; it should go through.

## Open requirements

None. The only detail the clarification left to the specification (how the
desktop identifies its bundled binary, D1 and the round 2 note) is settled in
[`plan.md`](plan.md), "Binary identity".
