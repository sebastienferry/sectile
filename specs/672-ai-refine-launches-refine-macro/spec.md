# Specification #672 - Roadmap: "AI refine" launches the refine-macro skill

- Ticket: https://github.com/sebastienferry/sectile/issues/672
- Branch: `feat/672`
- Clarification: `docs/clarifications/672.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-06)
- Framework: Spec Kit

## Summary

The **AI refine** button of a macro's Framing panel stops calling the
deterministic line splitter and launches the interactive `refine_macro` skill
on the user's local agent, as **Realign the spec** launches `realign_macro`.
The deterministic generator, its routes and its preview modal are removed.

## Scope

In scope:

- the Framing header of the Roadmap panel: both skill buttons, the active run,
  the stop action and the refresh of the macro when a run ends;
- the removal of the deterministic generator, server and web;
- the strings, the tests and the changelog.

Out of scope:

- the `refine-macro` skill itself;
- the server's `run-skill`, `runs` and `cancel-run` routes;
- how Sectile Desktop shows an interactive run;
- a launch path for users without a local agent.

## User stories

### US1 - Refine a macro with the skill (P1)

As a macro owner, I click **AI refine** and the refine-macro skill starts on my
local agent, so the framing is refined with judgment rather than split line by
line.

- **Given** a macro with a framing text and a connected local agent serving
  the project, **when** I click **AI refine**, **then** a `refine_macro` run is
  launched on the macro and a success toast names the macro.
- **Given** the run is active, **then** the **AI refine** button shows it as
  running, or as waiting for my answer while the skill waits on me, and a stop
  button stops it, as for Realign.
- **Given** a dirty framing draft, **when** I click **AI refine**, **then** the
  draft is saved before the launch.
- **Given** an empty framing text, **when** I click **AI refine**, **then** the
  "framing required" warning is shown and nothing is launched.

### US2 - The button says why it cannot launch (P1)

- **Given** no connected agent of mine serves the project, **then** **AI
  refine** is disabled and its tooltip says to connect the local agent, as
  Realign does.

### US3 - One skill at a time on a macro (P1)

- **Given** a realign run is active on the macro, **then** **AI refine** is
  disabled with "another skill is running on this macro", and the run is shown
  and stoppable on **Realign the spec** only.
- **Given** a refine run is active, **then** the reverse holds.
- **Given** a run of another macro skill (started by hand through MCP) is
  active, **then** both buttons are disabled, and the run is shown with its
  skill name and stoppable on **Realign the spec**, the button that showed
  every macro run until now.
- A run recorded under an alias (`refine`, `refine-macro`, `sectile:refine-macro`)
  is matched to its button like the catalog ID.

### US4 - The panel shows what the run saved (P1)

- **Given** a run of either skill is active, **when** it leaves the running
  state, **then** the panel re-reads the project's macros, so TODOs saved by
  the run show up without a manual reload.

### US5 - No label claims AI where none runs (P2)

- The deterministic generator is removed: no route, no function, no preview
  modal, no string is left of it. Stories are still created from TODOs with the
  panel's existing **Create the stories** action.

## Functional requirements

- FR1 `POST /api/projects/{id}/macros/{key}/refine` and `POST /api/macros/{key}/refine`
  no longer exist; `/api/macros/` is no longer routed.
- FR2 The macro's runs are read once per selected macro and shared by both
  buttons (no double polling).
- FR3 A run belongs to a button when its skill name, normalised through the
  catalog aliases, equals the button's skill ID.
- FR4 The launch blocker gives, in order: no agent; already running (own
  skill); another skill running (any other active run).
- FR5 The strings exist in French and English. The label stays "AI refine" /
  "Raffiner AI"; its tooltip says it launches refine-macro on the local agent.
- FR6 `CHANGELOG.md` gains a `Changed` and a `Removed` line under
  `## [Unreleased]`.

## Open points

None. Every decision was settled in the clarification.
