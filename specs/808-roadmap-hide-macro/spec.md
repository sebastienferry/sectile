# Specification #808 - Web UI: Roadmap, hide a macro

- Ticket: https://github.com/sebastienferry/sectile/issues/808
- Branch: `feat/808`
- Clarification: `docs/clarifications/808.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-09)
- Framework: Spec Kit

## Summary

In the web Roadmap, a user whose list is in the condensed row shape has no
way left to send a macro to the HIDDEN horizon: the condensed row offers NOW,
NEXT and LATER only, and the side panel shows the macro's horizon as a
read-only badge. The fix puts a NOW / NEXT / LATER / HIDDEN selector back in
the side panel header, in place of the badge, so a macro can be hidden, and
un-hidden, from the panel whatever the row shape and whatever the tab.

## Scope

In scope:

- the horizon shown in the header of the Roadmap side panel of the selected
  macro;
- a browser regression test of the Roadmap;
- the changelog.

Out of scope:

- the condensed row, which keeps NOW / NEXT / LATER only (#345 decision
  unchanged);
- the unfolded row, which keeps its four buttons unchanged;
- the meaning of the `hidden` horizon, its `roadmap:hidden` tracker label, how
  a horizon is saved and mirrored to the tracker;
- the board's "hidden project" notion;
- Sectile Desktop.

## User stories

### US1 - Hide a macro from the panel (P1)

As a Roadmap user working with the condensed list, I can hide the selected
macro from its side panel, so setting a macro aside does not require switching
the list back to the unfolded shape.

- **Given** a selected macro and the side panel shown, **when** I look at the
  panel header, **then** I see four horizon buttons labelled `NOW`, `NEXT`,
  `LATER` and `HIDDEN`, next to the macro key, and no separate read-only
  horizon badge.
- **Given** the list in the condensed shape, **when** I click `HIDDEN` in the
  panel, **then** the macro's horizon is saved as `hidden`, the Roadmap
  switches to the Hidden tab, the macro is listed there and stays selected.
- **Given** the list in the unfolded shape, **when** I click `HIDDEN` in the
  panel, **then** the outcome is the same as from the unfolded row's `HIDDEN`
  button.
- **Given** the save is refused (no current project, or the server answers an
  error), **then** the behaviour is today's for a row button: the tab does not
  change and the error is reported as today.

### US2 - Reclassify a macro from the panel, Hidden tab included (P1)

As a Roadmap user, I can move the selected macro to any horizon from its
panel, including out of the Hidden tab, so hiding stays reversible from the
same place.

- **Given** a selected macro in the Hidden tab, **when** I click `NOW`, `NEXT`
  or `LATER` in the panel, **then** the horizon is saved, the Roadmap switches
  to that tab and the macro stays selected there.
- **Given** a selected macro in any tab (NOW, NEXT, LATER, HIDDEN,
  unclassified), **then** the panel offers the four buttons.
- **Given** the button of the macro's current horizon, **when** I click it,
  **then** nothing visible changes beyond what a row's active button does
  today.

### US3 - The panel reads like the rows (P2)

- **Given** a classified macro, **then** the button of its horizon is filled
  with the horizon's colour and white text; the others are outlined, in their
  horizon's colour, as on the unfolded row.
- **Given** an unclassified macro whose sprints suggest a horizon, **then** the
  suggested button is highlighted with the horizon's tinted background and
  reads `<LABEL> ?`, and its tooltip is the existing "suggested from the
  sprints" text, as on the unfolded row.
- **Given** any other button, **then** its tooltip is the existing "classify
  as <horizon>" text.
- **Given** a screen reader, **then** the active button is announced as
  pressed and the others as not pressed.

## Functional requirements

- **FR-001** The side panel header of the selected macro shows a selector of
  the four horizons, in the order NOW, NEXT, LATER, HIDDEN, in place of the
  read-only horizon badge.
- **FR-002** Each button saves the macro's horizon through the same path as
  the row buttons, with the same side effects: the list switches to the target
  tab, the macro stays selected, the tracker label is mirrored as today.
- **FR-003** The selector is offered whatever the row shape (condensed or
  unfolded), whatever the tab, and whether the panel is expanded or not.
- **FR-004** The condensed row keeps NOW / NEXT / LATER only; the unfolded row
  keeps its four buttons; neither changes.
- **FR-005** The selector uses the existing horizon labels and tooltips; no
  new user-facing string is introduced.
- **FR-006** `CHANGELOG.md` carries one `Fixed` line under `## [Unreleased]`.

## Success criteria

- **SC-001** With the list condensed, a macro is hidden in one click from the
  panel and appears in the Hidden tab, without changing the row shape.
- **SC-002** A hidden macro is moved back to NOW, NEXT or LATER in one click
  from the panel.
- **SC-003** The existing Roadmap browser and unit tests pass unchanged.

## Acceptance

- [ ] US1, US2 and US3 behave as described in a browser.
- [ ] A browser test hides a macro from the panel while the list is condensed
      and checks it is listed in the Hidden tab, then un-hides it from the
      panel.
- [ ] The `CONDENSED_HORIZONS` comment in `web/src/lib/roadmapDisplayMode.ts`
      ("stays offered by the unfolded shape and by the panel") is true again
      and is left as is.
- [ ] `CHANGELOG.md` has its `Fixed` line under `## [Unreleased]`.

## Reversed prior decision

`3263a5a2` (2026-09-03) removed the panel selector as a duplicate of the
row's. It is partially reversed: the duplicate is accepted because the
condensed row does not carry HIDDEN. The framing/execution switcher removed by
the same commit stays removed.

## Open points

None. Both product questions were answered in clarification round 2.
