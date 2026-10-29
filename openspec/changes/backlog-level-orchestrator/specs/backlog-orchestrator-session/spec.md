## ADDED Requirements

### Requirement: One orchestrator session decides over the whole backlog

Starting orchestration SHALL run one agent session for the project, with the
selected profile's Markdown file as its policy. The session SHALL be able to
read every candidate ticket and the previous orchestrator decisions before
launching anything. At most one orchestrator session SHALL be queued or running
per project.

#### Scenario: Start runs one session

- **GIVEN** an enabled project and a valid orchestration profile
- **WHEN** the user starts orchestration with that profile
- **THEN** one `orchestrate_backlog` session is queued for the project with no
  task, and the endpoint answers 202 with its run ID

#### Scenario: Second start while a session runs

- **GIVEN** an orchestrator session running for the project
- **WHEN** another start is requested, from any client
- **THEN** no second session is created and the answer names the running one

#### Scenario: Another profile is armed

- **GIVEN** the project armed with profile A
- **WHEN** the user starts profile B
- **THEN** the request is refused with the name of profile A

### Requirement: The server enforces the hard guards

`launch_task_chain` SHALL refuse, without launching, a ticket that would exceed
the project's cap, that already has an active run, that carries a blocker label,
that is at or past `implemented`, or that belongs to another project. It SHALL
accept calls only from the project's running orchestrator session. An accepted
launch SHALL start a chain that stops at `implemented`.

#### Scenario: Cap reached

- **GIVEN** a cap of 2 and 2 orchestrated runs active
- **WHEN** the session calls `launch_task_chain` on an eligible ticket
- **THEN** the call fails with a capacity reason and no run is queued

#### Scenario: Blocked ticket

- **GIVEN** a ticket labelled `blocked`
- **WHEN** the session calls `launch_task_chain` on it
- **THEN** the call fails naming the label

#### Scenario: Another agent calls the tool

- **GIVEN** an ordinary ticket run on the same project
- **WHEN** it calls `launch_task_chain`
- **THEN** the call is refused

### Requirement: Ticket chains carry context, not a gate

A chain launched by the orchestrator SHALL carry the `Backlog orchestration:`
prefix and the orchestrator's rationale on every step, and SHALL NOT instruct
the step to re-evaluate the ticket's eligibility or repeat the profile's policy.

#### Scenario: Chained step prompt

- **GIVEN** a chain launched with the rationale "unblocks PE-12"
- **WHEN** its specify step is enqueued
- **THEN** its prompt opens with `Backlog orchestration:`, contains the
  rationale, and contains neither the eligibility instruction nor the profile
  file's text

### Requirement: A freed slot wakes an armed orchestrator

While a project is armed, the end of an orchestrated chain, completed, failed,
stopped or canceled, SHALL start a new orchestrator session when none is queued
or running, and otherwise SHALL start one after the running session ends. A
session that launches nothing SHALL NOT cause another session. Stopping,
disabling orchestration, or the starting workstation saving its agents without
the armed profile SHALL disarm the project. A session woken while the armed
workstation is offline SHALL wait, queued, for that workstation to connect.

#### Scenario: Chain reaches implemented

- **GIVEN** an armed project and one orchestrated chain running
- **WHEN** that chain reaches `implemented`
- **THEN** one orchestrator session is queued on the armed workstation

#### Scenario: Chains end during a session

- **GIVEN** an armed project, an orchestrator session running, and two
  orchestrated chains
- **WHEN** both chains end before the session does
- **THEN** exactly one orchestrator session starts, after the running one ends

#### Scenario: Workstation offline

- **GIVEN** an armed project whose workstation is offline
- **WHEN** an orchestrated chain ends
- **THEN** the woken session stays queued, no other workstation takes it, and
  it is sent when the armed workstation connects again

#### Scenario: Nothing left to take

- **GIVEN** an armed project whose session deferred every candidate
- **WHEN** the session finishes
- **THEN** no further session is queued until a slot frees or the user starts
  orchestration again

#### Scenario: Stop

- **GIVEN** an armed project
- **WHEN** the user stops orchestration
- **THEN** the project is disarmed, running chains continue, and their end
  wakes nothing

### Requirement: Decisions are recorded and reused

Each launch and deferral SHALL be stored with its reason, its session and the
ticket's `updated_at`. `get_orchestration_state` SHALL return the latest
decision per ticket and flag deferred tickets unchanged since.

#### Scenario: Unchanged deferred ticket

- **GIVEN** a ticket deferred by a previous session and not edited since
- **WHEN** the next session reads the orchestration state
- **THEN** the ticket is flagged unchanged since its deferral, with its reason

#### Scenario: Desktop shows the reasons

- **GIVEN** a finished orchestrator session
- **WHEN** the user opens the profile's overview
- **THEN** each ticket the session launched or deferred appears with its reason
