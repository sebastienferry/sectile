# Implementation checklist

## 1. Storage

- [x] 1.1 Migration: `orchestrator_arming` and `orchestrator_decisions`
      table, partial unique index for one queued/pending/running
      orchestrator-scoped activity per project (SQLite and PostgreSQL).
- [x] 1.2 Project-scoped orchestrator run: create, adopt, finish, list, modelled
      on `macroruns.go`, carrying the profile ID.

## 2. Server

- [x] 2.1 Skill `orchestrate_backlog` in `internal/skills/catalog.go`, scope
      `project`, autonomous, with its fragment: read state, read backlog, decide,
      launch or defer each candidate, finish with a summary.
- [x] 2.2 MCP tools `get_orchestration_state`, `launch_task_chain`,
      `defer_task`, restricted to the running orchestrator session.
- [x] 2.3 `launch_task_chain` guards: `admitOrchestratedActivity`, busy ticket,
      blocker labels, stage, project; `ChainStop = implemented`; new prompt.
- [x] 2.4 Remove the Go selection loop, the length filters and
      `orchestratorLaunchPrompt`'s eligibility text.
- [x] 2.5 `POST /orchestrate` arms and starts (202 + run ID);
      `POST /orchestrate/stop` disarms; `orchestrate_project` MCP tool follows.
      Keep 404 / 409 / 400-on-unknown-field.
- [x] 2.6 Wake-up hook in `handBackRun` once the ticket holds no active run,
      with a pending flag for chains that end during a session.
- [x] 2.7 Disarm on disable and on profile removal.

## 3. Agent and desktop

- [x] 3.1 Project-scoped dispatch in `docs/contracts/server-agent-v1.md` and the
      agent; the agent reads the armed profile's file at each launch.
- [x] 3.2 Desktop: Start / Stop per profile, armed indicator, session console in
      the overview, decisions list with reasons; offline-workstation state.
- [x] 3.3 Membership predicate of #871 recognises the orchestrator session.

## 4. Verify

- [x] 4.1 Go tests for every scenario of the spec, on SQLite and PostgreSQL,
      including concurrent wake-ups and concurrent starts across instances.
- [x] 4.2 Desktop unit and UI tests for Start / Stop, overview and decisions.
- [x] 4.3 Update `TestOnlyOrchestratedRunsTakeASlot` for the new prompt.

## 5. Document

- [x] 5.1 Mark ADR 0060 superseded in part by ADR 0064; accept ADR 0064.
- [x] 5.2 Update `docs/ARCHITECTURE.md`, `docs/CAPABILITIES.md`, `README.md` if
      it describes orchestration, and `.agents/MEMORY.md` (the preflight-gate
      entries become obsolete).
- [x] 5.3 One `Changed` line under `[Unreleased]` in `CHANGELOG.md`: the
      orchestrator now reviews the whole backlog, explains its choices, and keeps
      taking tickets while started.
