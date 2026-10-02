# Specification #703 - Keep settings controls clear of the macOS overlay scrollbar

- Ticket: https://github.com/sebastienferry/sectile/issues/703
- Branch: `feat/703`
- Clarification: `docs/clarifications/703.md` (round 1, no product question)
- Framework: Spec Kit
- Type: bug

## Summary

In Sectile Desktop settings, a panel that scrolls keeps its controls far
enough from its right edge that an overlay scrollbar, such as the one macOS
draws with "Show scroll bars: Automatic", never covers them. Every control of
such a panel, the reset buttons of Execution defaults first, is clickable
across its whole width once the panel is scrolled, whatever the platform's
scrollbar style.

## Scope

In scope: the scrolling panels of Desktop's workstation settings (Settings,
reached from the sidebar's settings button), the project settings view, the UI
test that fails on macOS, and the changelog.

Out of scope:

- Restyling the scrollbar itself.
- The web interface's settings.
- Scrolling areas of Desktop outside settings (sidebar, task lists,
  conversation, changes panel) and the log reader of the Logs panel, which
  scrolls inside its own bordered box with no control on its right edge.
- The size, order or layout of the reset buttons and other controls.

## Vocabulary

- **Overlay scrollbar**: a scrollbar that takes no layout space and is drawn
  over the content's right edge while the area scrolls (macOS with a trackpad
  and "Automatic", or "When scrolling"). About 15 px wide.
- **Classic scrollbar**: a scrollbar that takes its own layout space beside
  the content (Windows, Linux, macOS "Always").
- **Scrolling settings panel**: the element of a settings view that scrolls
  when its content is taller than the view: each category panel of the
  workstation settings except Logs, and the content area of the project
  settings.
- **Gutter**: the empty band between a scrolling panel's right edge and the
  right edge of its content.

## User stories

### US1 (P1) - Reset an Execution defaults setting on macOS

As an owner on macOS with overlay scrollbars, I scroll Execution defaults and
click a reset button, and the setting resets, wherever on the button I click.

1. Given Execution defaults scrolled down with an overlay scrollbar showing,
   when I click the right half of "Reset custom project skills win to
   default", then the setting resets to its default.
2. Given the same panel, when I point at any pixel of any reset button, then
   the button, not the panel, is the element under the pointer.
3. Given a classic scrollbar, then the controls sit left of the scrollbar as
   today, with the gutter added.

### US2 (P1) - Every scrolling workstation settings panel keeps the gutter

1. Given any workstation settings category other than Logs, when its panel is
   taller than the view, then its controls stay at least one overlay scrollbar
   width away from its right edge.
2. Given a panel shorter than the view, then the gutter is still there, so the
   controls do not shift when the content grows into a scroll.
3. Given the narrow layout (window 720 px wide or less), then the same gutter
   applies.

### US3 (P2) - The project settings view keeps its clearance

1. Given the project settings view scrolled down, then its controls stay at
   least one overlay scrollbar width away from its right edge, as they already
   do today.

## Functional requirements

- **FR1** Every scrolling workstation settings panel (every category panel but
  Logs) keeps a right gutter of at least 16 px between its right edge and its
  content, whether it currently scrolls or not.
- **FR2** The gutter belongs to the element that scrolls, so an overlay
  scrollbar is drawn over the gutter and never over a control.
- **FR3** The gutters do not stack: the element around the panels, which does
  not scroll in the workstation settings, adds none on top of the panel's.
- **FR4** The project settings view, whose content area scrolls itself, keeps
  a right clearance of at least 16 px (24 px today); the change does not
  reduce it.
- **FR5** The Logs panel and its log reader are unchanged.
- **FR6** The narrow layout keeps the same gutter as the wide one.
- **FR7** No control changes size, order or alignment within its row.
- **FR8** `CHANGELOG.md` gains one `Fixed` line under `[Unreleased]`, written
  for users.

## Acceptance criteria

- `desktop/tests/workstation-settings.ui.cjs` passes on macOS with overlay
  scrollbars, including "the skill settings save through the agent and reset
  to their defaults", with its plain centre clicks.
- A platform-independent check, run on every OS: with Execution defaults
  scrolled, each reset button's right edge lies at least 15 px left of the
  scrolling panel's content right edge (its left plus `clientWidth`), and the
  element under the button's rightmost pixel is the button.
- The other Desktop UI suites that open settings pass unchanged.
- US1 to US3 scenarios hold.

## Edge cases

- Classic scrollbars: the scrollbar already takes its own space; the gutter
  adds 16 px of padding inside it. Accepted: the panels are capped at 900 px
  and the extra space is not visible as a defect.
- macOS "Always" scrollbars behave as classic ones.
- A panel whose content fits without scrolling keeps the gutter (FR1), so its
  layout is the same before and after it starts to scroll.
- A wide window: panels are capped at 900 px; the scrollbar sits on the
  panel's own right edge, so the gutter is needed there too.

## Open points

None. The clarification left no product question, and its technical choices
are applied in `plan.md`, with one factual correction recorded there: the
project settings view already has 24 px of clearance, so it needs no change.
