# Specification #711 - A launch from the web follows the Desktop console view

- Ticket: https://github.com/sebastienferry/sectile/issues/711
- Branch: `feat/711`
- Clarification: `docs/clarifications/711.md` (round 2, the owner chose the
  recommended scope: every interactive dispatch)
- Framework: Spec Kit

## Summary

With **Settings → Appearance → Claude consoles** set to **Conversation** in
Sectile Desktop, a skill launched interactively from Desktop opens in the
conversation view, but the same launch started from the web app opens a
terminal on the workstation. After this change, every interactive launch the
server dispatches to this workstation's agent follows the Desktop setting,
whichever client asked for it.

## Scope

In scope: every interactive dispatch that reaches this workstation's agent,
whatever started it:

- the **Clarify** follow-up of the web app's new task dialog;
- the skill buttons of a web task card;
- **Run skill** and **Discussion** in the web task detail;
- the next stage of a chain the server continues on its own;
- every launch Desktop already makes, which keeps working as today.

Out of scope:

- autonomous (headless) launches, which keep their read-only trace;
- engines other than Claude, and Claude engines the conversation view cannot
  honour, which keep the terminal;
- **Discussion in native terminal** and the `open_terminal` action;
- Desktop's free console (**New console**), which is not a dispatch;
- a per-launch "terminal or conversation" choice in the web app;
- moving the setting to the server or to the project configuration;
- any server, web, database or migration change.

## Vocabulary

- **Console view setting**: **Settings → Appearance → Claude consoles** in
  Desktop, **Terminal** (default) or **Conversation**.
- **Workstation console view**: the copy of that setting the local agent
  holds, so that it applies when the launch did not come through Desktop.
- **Interactive dispatch**: a skill or discussion launch the server sends to
  this workstation's agent in interactive mode, whichever client asked for it.
- **Eligible dispatch**: an interactive dispatch that is not `open_terminal`
  and whose engine `conversationDiscussionEngine` accepts (a Claude provider).

## User stories

### US1 (P1) - Clarify a task created on the web

As a user who chose **Conversation** in Desktop, I create a task in the web
app and choose **Clarify**, and the clarification opens in Desktop's
conversation view on my workstation, as it would had I created the task in
Desktop.

### US2 (P1) - Every web launch follows the setting

As the same user, a skill launched from a web task card, **Run skill** or
**Discussion** in the web task detail, and the next stage a chain starts on
its own, all open as a conversation, so the setting means the same thing
whatever I click.

### US3 (P2) - The setting survives an agent restart and a closed Desktop

As the same user, after the agent restarts, or while Desktop is closed, a web
launch still opens as a conversation; I see it as soon as Desktop opens.

### US4 (P2) - Terminal users see no change

As a user who kept **Terminal**, or who runs an agent older than Desktop,
nothing changes: every launch opens a terminal as today.

## Functional requirements

- **FR1** The agent holds a workstation console view, `terminal` or
  `conversation`. An agent that never received one holds `terminal`.
- **FR2** Desktop gives the agent its console view setting each time it
  connects to the agent, and each time the user changes the setting while the
  agent is connected.
- **FR3** The agent keeps the workstation console view across its own
  restart.
- **FR4** An eligible dispatch opens as a conversation when Desktop marked
  that launch for the conversation view (today's behaviour), or when no mark
  exists and the workstation console view is `conversation`. Otherwise it opens
  a terminal, as today.
- **FR5** An autonomous dispatch, an `open_terminal` dispatch, **Discussion in
  native terminal**, and a dispatch whose engine the conversation view cannot
  honour behave exactly as today, whatever the workstation console view.
- **FR6** A change of the setting applies to the next dispatch; a session
  already open keeps its view.
- **FR7** Desktop gives the setting only to an agent announcing the capability
  for it in `/desktop/status`. Desktop connected to an older agent behaves as
  today; an older Desktop connected to a new agent leaves the agent at
  `terminal`, which is today's behaviour.
- **FR8** A failure to give the setting to the agent never blocks saving it in
  Desktop, nor any launch; Desktop gives it again at the next connection.
- **FR9** The agent refuses a console view other than `terminal` or
  `conversation` and keeps the one it held.

## Acceptance scenarios

1. **Given** Desktop set to **Conversation** and connected to its agent,
   **when** the user creates a task in the web app and chooses **Clarify**,
   **then** the clarify run appears in Desktop as a conversation in the task's
   worktree, started on the clarify command.
2. **Given** the same setting, **when** the user clicks **Discussion** in the
   web task detail, **then** Desktop shows a conversation waiting for the first
   message.
3. **Given** the same setting, **when** a web task card launches a skill
   interactively, or the server continues a chain on an interactive stage,
   **then** the run opens as a conversation.
4. **Given** the same setting, **when** a web launch is autonomous, **then** it
   runs headless with its read-only trace, as today.
5. **Given** the same setting and a project whose engine is not Claude,
   **when** the web launches a skill on it, **then** a terminal opens, as
   today.
6. **Given** Desktop set to **Terminal**, **when** the web launches a skill,
   **then** a terminal opens, as today.
7. **Given** Desktop set to **Conversation**, **when** the user switches it to
   **Terminal** and the web launches a skill, **then** a terminal opens.
8. **Given** Desktop set to **Conversation**, **when** the agent restarts and,
   before Desktop reconnects, the web launches a skill, **then** it opens as a
   conversation.
9. **Given** Desktop set to **Conversation** and then closed, **when** the web
   launches a skill, **then** it opens as a conversation and Desktop lists it
   when it opens.
10. **Given** an agent that never received a console view, **when** the web
    launches a skill, **then** a terminal opens.
11. **Given** Desktop set to **Conversation** and an agent without the new
    capability, **when** the user changes the setting, **then** the setting is
    saved, no error is shown, and web launches open a terminal, as today.
12. **Given** Desktop set to **Conversation**, **when** a skill is launched
    from Desktop, **then** it opens as a conversation, as today.
13. **Given** Desktop set to **Conversation**, **when** the user picks
    **Discussion in native terminal**, **then** the native terminal opens, as
    today.

## Edge cases

- The setting is changed while the agent is stopped: Desktop saves it, and
  gives it to the agent when it reconnects; a dispatch reaching the agent in
  between uses the value the agent held.
- Two Desktop launches and a web launch on the same task at once: the
  explicit mark of a Desktop launch is consumed by its dispatch as today; the
  web one falls back to the workstation console view, which gives the same
  result.
- An agent settings file written before this change has no console view: the
  agent reads `terminal`.

## Changelog

No new line: the conversation feature is still under `[Unreleased]`. Its
existing `Added` line, **A task's interactive launches open as a Claude
conversation.**, is amended to say that the setting applies to interactive
launches started from Desktop or from the web app, including a chain the
server continues.

## Open points

None. The clarification settled the only product question (Q1: every
interactive dispatch follows the setting).
