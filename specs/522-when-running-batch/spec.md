# Specification #522 - Show every ticket of a running batch on the web board

- Ticket: https://github.com/sebastienferry/sectile/issues/522 (Bug)
- Branch: `feat/522`
- Clarification: `docs/clarifications/522.md` (rounds 1 and 2, confirmed by
  the owner), plus the decisions taken during specification listed under
  "Decisions taken during specification" (FR5, FR9, FR10, FR12, FR13)
- Framework: Spec Kit

## Summary

When a batch pickup is launched from the web, only the first ticket of the
batch shows that work is in progress. Every ticket of a running batch must show,
on the web board, in the web list and in the web task detail panel, that it
belongs to that batch and where it stands in it: waiting its turn, being
processed, or already done. While the batch runs, its tickets are busy: another
launch on any of them is refused with a message that names the batch. Every
batch indicator disappears when the batch run ends, whatever its outcome.

## Scope

In scope:

- Recording which tickets a batch launched from the web contains, and in which
  order.
- The state of each ticket within its running batch.
- The batch indicators on web board cards, web list rows and the web task
  detail panel.
- Refusing a launch on a ticket of a running batch.
- The agent's instructions for reporting which ticket of the batch it is
  working on.

Out of scope:

- How the agent processes the batch: order, single worktree, combined pull
  request.
- The batch selection and the batch launch dialog.
- The desktop app's display. The desktop app has no batch launch; it receives
  the refusal of FR8 like any other client, and shows it the way it shows any
  refused launch today.
- Batch runs launched before this change: they carry no recorded membership
  and keep today's display.

## Definitions

- **Batch**: the ordered list of tickets selected on the web and launched
  together with the batch pickup skill (`pickup_issues`).
- **Batch run**: the single run the launch records. It sits on the first ticket
  of the batch.
- **Lead ticket**: the first ticket of the batch, the one carrying the batch
  run.
- **Member**: any ticket of the batch, the lead ticket included. A member has a
  position (1 to N, in launch order).
- **Running batch**: a batch whose batch run is active (queued, pending or
  running, including while it waits for user input).
- **Member state**: one of *waiting*, *processing* or *done*, meaningful only
  while the batch runs.
- **Batch badge**: the small badge that reads `Lot <lead ticket key>`.

## User stories (prioritised)

### US1 - See every ticket of a running batch (P1)

As a web user who launched a batch, I see on each of its tickets that it belongs
to that batch, so the tickets waiting their turn do not look idle.

**Acceptance scenarios**

1. **Given** I launch a batch of #10, #11 and #12 from the web board, **when**
   the launch succeeds, **then** the cards of #10, #11 and #12 each show the
   batch badge `Lot #10`.
2. **Given** that batch is running, **when** I reload the page, or another user
   opens the same board, **then** the three cards show the same badges and
   states as before the reload.
3. **Given** that batch is running, **when** I switch to the list view, **then**
   the rows of #10, #11 and #12 show the same badge and state as their cards.
4. **Given** that batch is running, **when** I open the detail panel of #11,
   **then** it shows the same badge and state as the card of #11.
5. **Given** a ticket that is not in any running batch, **when** I look at its
   card, row or detail panel, **then** it shows no batch badge and its display
   is unchanged from today.

### US2 - See where each ticket stands in the batch (P1)

As a web user, I see which ticket the agent is working on, which ones wait
their turn and which ones are done.

**Acceptance scenarios**

1. **Given** a batch of #10, #11 and #12 was just launched, **when** I look at
   the board, **then** #10 shows `en cours` in the indigo running style, and
   #11 and #12 show `en attente dans le lot` in the amber queued style.
2. **Given** the agent reports that it starts working on #11, **when** the
   board updates, **then** #11 shows `en cours` (indigo), #10 shows only its
   batch badge, with neither the indigo nor the amber highlight, and #12 still
   shows `en attente dans le lot` (amber).
3. **Given** the agent reports that it starts working on #12, **when** the
   board updates, **then** #12 shows `en cours`, and #10 and #11 show only
   their batch badge.
4. **Given** the agent never reports which ticket it works on, **when** the
   batch runs, **then** #10 shows `en cours` and the other members show
   `en attente dans le lot` until the batch ends.

### US3 - Batch tickets are busy (P1)

As a web user, I cannot start a second agent on a ticket the batch is about to
process, is processing, or has processed.

**Acceptance scenarios**

1. **Given** a running batch of #10, #11 and #12, **when** I launch any skill
   on #11 from the web, **then** the launch is refused, nothing is recorded on
   #11, and the error message says that #11 belongs to the batch led by #10.
2. **Given** the same batch, **when** I launch a skill on #12 after the agent
   finished it within the batch (state *done*), **then** the launch is refused
   the same way.
3. **Given** a ticket #11 that already has an active run, **when** I launch a
   batch containing #11, **then** the batch launch is refused, nothing is
   recorded on any ticket, and the error message names #11 and its active run.
4. **Given** a ticket #11 in a running batch, **when** the batch run's owner or
   an admin uses "Launch anyway" on #11 from a client that offers it, **then**
   the launch is accepted as a concurrent run, as for any busy ticket today.

### US4 - Indicators end with the batch (P1)

**Acceptance scenarios**

1. **Given** a running batch of #10, #11 and #12, **when** the batch run ends
   completed, failed or canceled, **then** every batch badge and member state
   disappears from the cards, rows and detail panels of #10, #11 and #12, and
   each ticket shows what it would show without the batch.
2. **Given** that batch has ended, **when** I launch a skill on #11, **then**
   the launch is accepted.

## Functional requirements

### Membership

- **FR1** - A batch launched from the web records its members and their order
  on the server, at the moment the batch run is recorded. A launch that is
  refused records no membership.
- **FR2** - Membership is attached to the batch run. It survives a page reload,
  a server restart and is the same for every viewer and every server replica.
- **FR3** - A batch launch names at least two tickets, each at most once, all
  of the same project, the first being the ticket the launch is made on. A
  launch that breaks one of these rules is refused as invalid, and nothing is
  recorded.

### Member state

- **FR4** - At launch, the lead ticket is *processing* and every other member
  is *waiting*.
- **FR5** - When the agent reports that it starts working on a member (FR6),
  that member becomes *processing*, and the member that was *processing* before
  becomes *done*. A member already *done* that the agent reports again becomes
  *processing* again. Reporting the member that is already *processing* changes
  nothing.
- **FR6** - The agent reports the member it starts working on by starting its
  run on that member with the batch run's identifier. This is accepted for any
  member of that batch while the batch run is running, returns the batch run
  itself, and creates no new run. The same call with a ticket that is not a
  member of that batch is refused as today.
- **FR7** - Stage transitions, comments and any other report on a member do not
  change its member state.

### Busy rule

- **FR8** - While a batch runs, every member is busy: a launch of any skill on a
  member is refused as a launch on a ticket with an active run is refused today,
  and the refusal message names the member and the lead ticket, for example
  `#11 is part of the batch led by #10, which is still running.`
- **FR9** - "Launch anyway" on a member follows the rule that applies to any
  busy ticket: it is reserved to the batch run's owner or an admin, and records
  a concurrent run without touching the batch.
- **FR10** - A batch launch is refused when any of its tickets already has an
  active run, or already belongs to a running batch. The refusal names the first
  such ticket and what makes it busy.
- **FR11** - The agent's own reports on the members of its batch (FR6, stage
  transitions, run start without a run identifier) are never refused by the
  busy rule.

### Display (web only)

- **FR12** - Every member of a running batch shows the batch badge `Lot <lead
  key>` on its board card, its list row and its detail panel. The badge's
  tooltip names the lead ticket and the member's position, for example
  `Lot mené par #10 · ticket 2 sur 3`.
- **FR13** - Next to the batch badge, a member shows its state:
  - *waiting*: the label `en attente dans le lot`, in the amber style the card
    uses today for a queued run;
  - *processing*: the label `en cours`, in the indigo style the card uses today
    for a running run;
  - *done*: no label and no highlight, the batch badge only.
- **FR14** - On the board card, the member state also drives the card border:
  amber for *waiting*, indigo for *processing*, none for *done*. This replaces,
  for the lead ticket, the indigo border its batch run would give it: the lead
  card's border follows its member state.
- **FR15** - The run badge that shows and controls the batch run on the lead
  ticket (state glyph, stop control) is unchanged.
- **FR16** - A member's own run, started outside the batch with "Launch anyway",
  keeps its run badge. The batch badge and member state are shown next to it.
- **FR17** - The indicators update without a page reload when the batch starts,
  when a member's state changes, and when the batch run ends, within the delay
  the board takes today to show a run starting or ending.
- **FR18** - All new labels exist in the web interface's French and English
  locales. English: `Batch <lead key>`, `waiting in batch`, `in progress`,
  `Batch led by #10 · ticket 2 of 3`.

### Agent contract

- **FR19** - The batch pickup skill instructs the agent to reuse the batch
  run's identifier for every ticket of the batch: start the run on each ticket,
  with that identifier, when it begins working on it; finish the run once, on
  the lead ticket, when the whole batch ends.

## Decisions taken during specification

These points were not settled by the clarification and follow from its decisions:

- **FR5**: the *processing* marker moves only on an explicit report from the
  agent. Stage transitions are not used as a signal (FR7): a batch records
  `reviewed` on every ticket at its end, which would move the marker back and
  forth.
- **FR9**: "Launch anyway" keeps working on a member, because the clarification
  asked for consistency with the one-active-run rule, and that rule has this
  escape. The web offers no "Launch anyway" today, so this only concerns the
  desktop and the API.
- **FR10**: a batch cannot take in a busy ticket. Without this rule a ticket
  would carry two active runs at once, which the busy rule exists to prevent.
- **FR12**, **FR13**: the tooltip and the English labels are wording choices
  made here; the French labels are the owner's.

## Edge cases

- A member deleted or moved to another project while the batch runs: it simply
  stops being displayed; the batch and the other members are unaffected.
- A server restart while a batch runs: membership and member states are kept;
  the batch run follows today's restart rules, and the indicators follow the
  batch run.
- The agent reports a member twice in a row: nothing changes (FR5).
- The agent stops on a blocked member: that member stays *processing* until the
  batch run ends.
- A batch run launched before this change carries no membership: only its lead
  ticket shows it, as today.

## Success criteria

- **SC1** - On a running batch of N tickets, the N cards, the N list rows and
  the N detail panels show the batch badge, from the launch to the end of the
  batch run, across page reloads.
- **SC2** - At every moment of a running batch, exactly one member shows
  `en cours`, the members before it show only the badge, and the members after
  it show `en attente dans le lot`, provided the agent reports the members in
  order.
- **SC3** - No launch on a member of a running batch is accepted without
  "Launch anyway".
- **SC4** - Within the usual refresh delay after the batch run ends, no badge
  or member state remains on any member.

## Changelog

The change is visible to users: `CHANGELOG.md` gains a `Fixed` line under
`## [Unreleased]` describing the batch indicators and the busy members (#522).

## Open points

None. The clarification left nothing open; the decisions above are the ones the
specification had to take, and they can be revisited at review.
