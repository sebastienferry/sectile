# #249 — Editing a project drops its autonomous CLI command

Ticket: https://github.com/sebastienferry/sectile/issues/249
Type: Bug — "Only interactive CLI commands are saved, not autonomous ones. In Desktop app and Web Ux."
Branch: `feat/249`.
Clarification: [`docs/clarifications/249.md`](../../docs/clarifications/249.md).

## Context

A project carries two CLI commands: the interactive one and the autonomous
(headless) one. Both are stored when a project is created, but saving the
project settings afterwards keeps whatever autonomous command was stored before,
whatever the form sent. The desktop app shows the command inherited from the
server, so it shows the same stale value.

The web form also cannot clear either command: an empty field is not sent, so the
stored value silently stays.

Out of scope: how the two commands are resolved at launch (`{mode:...}` marker,
provider defaults, workstation overrides), the deployment-wide settings screen
(it already saves both), any desktop or agent change, and any new field.

This file states behaviour and acceptance criteria only. Implementation choices
are in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

## Decisions being specified

Settled by the owner in clarification round 2:

1. **Server fix only.** The desktop inherits the saved server value and needs no
   change. A lost desktop *local* override, if still observed after this fix,
   becomes its own ticket.
2. **An empty field clears.** Web project settings always send both commands,
   empty included. An empty autonomous command means the interactive one serves
   headless launches; both empty means the inherited defaults apply. Turning
   "custom agent" off clears both.

## User stories

### P1 — Saving an autonomous command on an existing project

As a project owner, I set the autonomous CLI command in the project settings and
save, so that headless launches use it.

- **Given** an existing project,
  **When** the owner saves the project with an autonomous command,
  **Then** reopening the project settings shows that command,
  **And** the agent configuration served for the project carries it.
- **Given** an existing project with both commands stored,
  **When** the owner saves the project changing only the interactive command,
  **Then** both stored commands reflect what the form sent.

### P2 — Clearing a command

As a project owner, I empty a command field and save, so the project falls back
to the inherited command.

- **Given** a project with an autonomous command stored,
  **When** the owner empties the autonomous field and saves,
  **Then** the project stores no autonomous command.
- **Given** a project with an interactive command stored,
  **When** the owner empties the interactive field and saves,
  **Then** the project stores no interactive command.
- **Given** a project with both commands stored,
  **When** the owner turns "custom agent" off and saves,
  **Then** the project stores neither command.

### P3 — The form shows what is stored

- **Given** a project that stores only an autonomous command,
  **When** the owner opens its settings,
  **Then** "custom agent" is on and the autonomous field shows the command,
  so saving without touching anything does not clear it.

## Functional requirements

- **FR-1** Updating a project applies the autonomous command it receives, exactly
  as it applies the interactive one.
- **FR-2** An update that does not mention a command leaves the stored value
  unchanged, so API clients other than the web form keep today's behaviour.
- **FR-3** An update carrying an empty command stores an empty command.
- **FR-4** The web project form always sends both commands, empty when the field
  is empty or "custom agent" is off.
- **FR-5** The web project form considers "custom agent" on when the project
  stores a provider, an interactive command or an autonomous command.
- **FR-6** Project creation, deployment-wide settings and launch-time resolution
  behave as before.

## Acceptance criteria

1. Editing a project in the web UI and saving an autonomous command persists it;
   reopening shows it; the agent config carries it.
2. Emptying either command and saving clears the stored value.
3. Turning "custom agent" off and saving clears both commands.
4. Interactive-command behaviour and deployment-wide settings are unchanged.

## Open requirements

None. Values lost by earlier edits cannot be recovered; users re-enter them once.
