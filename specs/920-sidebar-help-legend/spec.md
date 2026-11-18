# #920 — Sidebar help legend

Status: specified. Accepted baseline: docs/clarifications/920.md, round 2.
Task: https://github.com/sebastienferry/sectile/issues/920
Branch: feat/920. Framework: Spec Kit.

## Scope

Explain the existing Tauri Desktop sidebar execution and skill status symbols through persistent help. Preserve the current English runtime language. No new skill identity glyphs, status meanings, workflow behavior, Electron changes, provider marks, ticket stage color catalogue, or unrelated navigation/action icon catalogue.

## Prioritized user stories

### P1 — Understand status and required action

As a Desktop user, I can open a concise legend and understand what a symbol means and whether I must respond or inspect a result.

- Given a visible project sidebar, including an empty project/conversation list, when I scroll its list, then a discreet identifiable question-mark help button remains available at the bottom.
- Given the help button, when I click it or activate it with Enter or Space, then a compact legend opens with a title, labeled symbols, concise explanations and an explicit close action.
- Given the legend, when I read a ticket skill entry, then I can distinguish the skill's identity conveyed by text/tooltips from its state or outcome. A successful skill does not mean the entire ticket is finished.

### P1 — Dismiss help without affecting work

As a keyboard user, I can read and dismiss help and return to my previous context.

- Given the button, when I navigate by keyboard, then it has a descriptive accessible name and a visible focus indication.
- Given an open legend, when it opens, then focus moves inside; Tab and Shift+Tab remain inside; its title and dialog semantics are exposed to assistive technology.
- Given an open legend, when I activate Close or press Escape, then it closes and focus returns to its trigger if still present. If the sidebar disappears, focus returns to a sensible remaining control rather than a detached element.
- Given a selected conversation and an active execution, when I open and close help, then selection, transcript, draft and execution remain intact; help neither navigates nor sends input, starts, cancels or interrupts work. Live updates may continue normally.

### P2 — Read help in constrained layouts

- Given a narrow window or a long legend, when help opens, then text wraps, all entries and Close remain reachable, and scrolling stays within a viewport-bounded dialog without horizontal clipping.
- Given reduced-motion preferences, when the legend displays normally animated symbols, then pulse is disabled consistently with the sidebar and labels still convey their meaning.

## Functional requirements

FR-01: Help is available whenever the project sidebar is shown, outside its scrolling list. No help entry is required when the sidebar is hidden or in Settings.
FR-02: Group ticket skill states and conversation/fallback indicators so visually identical symbols are explained in context. Every entry uses the existing shape, color and relevant animation together with text; color alone is insufficient.
FR-03: Cover this complete inventory, verified against the current sidebar on 2026-11-17:

| Group / state | Existing appearance | Meaning and action to communicate |
| --- | --- | --- |
| Skill / working | Pulsing blue circle | Work is queued, preparing or running; no answer is implied. |
| Skill / waiting | Amber circle | Needs your answer: respond to the console question/approval or choose the required repository. |
| Skill / done | Green circle | The skill outcome is recorded; a workflow skill has attained its expected stage. The whole ticket is not necessarily finished. |
| Skill / verify | Yellow diamond | Needs verification: completion is unconfirmed, expected stage was not reached, or the session ended without reporting a result. Check the result and ticket. |
| Skill / failed | Red square | Could not finish. Inspect the console and recorded reason before deciding whether to relaunch. |
| Skill / stopping | Pulsing muted circle | Stop requested; waiting for the session to exit. |
| Skill / stopped | Filled muted circle | Execution stopped; completion is not implied. The recorded reason may be unavailable. |
| Conversation/fallback / live | Pulsing blue circle | Conversation busy or fallback execution active. |
| Conversation / session | Pulsing blue circle | Session active, possibly ready between turns; the pulse alone does not prove work is running. |
| Conversation/fallback / needs-input | Amber circle | A question, approval or other input needs a response. |
| Conversation/fallback / idle | Outlined muted circle | Inactive conversation/execution; this dot alone does not certify success. |

FR-04: Explain that row labels/tooltips provide context and details; the legend is explanatory and does not diagnose or alter a particular execution. Do not infer a stop reason or successful outcome from an idle dot.
FR-05: Opening help requires no selected conversation, active run, network request or agent connection.
FR-06: Preserve existing sidebar symbols, reconciliation rules and English wording elsewhere.
FR-07: Implementation updates docs/USER_GUIDE.md and adds one user-facing Added entry under CHANGELOG.md [Unreleased]. Specification alone adds no changelog entry.

## Acceptance and success criteria

All Given/When/Then cases above pass. UI verification covers all eleven inventory entries, correct visual correspondence, keyboard opening, focus containment/return, Close and Escape, empty and scrolling lists, continuing execution, retained selection/draft, narrow viewport, overflow and reduced motion. No backend or native permission changes are needed.

## Open questions

None. Owner accepted round 2. Recheck symbol inventory if rendering changes before implementation; do not invent semantics for new states.
