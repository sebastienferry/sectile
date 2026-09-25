# My Tasks finds the tickets assigned to me

Scope restated from the clarification (`docs/clarifications/468.md`, two
rounds, confirmed by the owner). Implementation choices are in `plan.md`.

## User stories

### P1: My Tasks shows my tickets on the tracker I work in

As someone signed in to Sectile on a GitHub or Jira project, I want the
**My Tasks** button of the sidebar to narrow the board to the tickets assigned
to me on that tracker, so that I see my work without knowing how the tracker
spells my name.

### P2: My Tasks works across trackers

As someone looking at "All projects" or a saved view that mixes GitHub, Jira
and local projects, I want **My Tasks** to keep my tickets from each of them at
once, so that one click answers "what is on my plate" everywhere.

### P3: I am told when Sectile cannot know who I am on a tracker

As someone without a personal credential for a tracker, I want **My Tasks** to
still do its best and to tell me which trackers it had to guess on and how to
fix it, so that an empty or partial board is never a silent failure.

## Functional requirements

1. **"Me" per tracker.** A ticket is mine when its assignee equals one of my
   identities for the ticket's tracker:
   - **GitHub:** the login of the account my personal GitHub credential belongs
     to.
   - **Jira:** the display name of the account my personal Jira credential
     belongs to.
   - **Any other tracker with a personal credential** (GitLab): the account name
     that tracker reports for it.
   - **Local tickets:** my account name and my account e-mail.
2. **Fallback.** On a tracker for which my identity is not known, my identities
   are my account name and my account e-mail, as for local tickets. My identity
   is not known when I hold no personal credential for that tracker, or when I
   hold one whose account was never confirmed by the tracker (see 4).
3. **Comparison.** An assignee equals an identity when they are the same text
   once leading and trailing spaces are removed and case is ignored. Nothing
   else is folded: no accent folding, no partial match.
4. **When my identity becomes known.** My identity on a tracker is learnt when
   the tracker confirms my personal credential: when I save it, or when I
   verify the stored one from my profile. It is forgotten when I delete that
   credential, and replaced when I save another one. Loading or filtering the
   board never contacts a tracker.
5. **Locked credentials.** A sealed personal credential whose account was
   confirmed keeps its identity while it is locked: the identity is the
   tracker's account name, not the secret the seal protects.
6. **Scope.** My Tasks combines with the project, "All projects", a saved view
   and every other filter of the board (search, status, priority, label,
   sprint, team, macro, tracker status, issue type, pinned).
7. **The button.** Clicking **My Tasks** turns the filter on; clicking it again
   turns it off. While it is on, the button shows as active, whatever my
   account name is or becomes.
8. **The person picker is unchanged.** Choosing a person in the filter panel
   still shows the tickets assigned to exactly that name. Choosing a person
   turns My Tasks off, and turning My Tasks on clears the chosen person. A
   status shortcut of the sidebar, which clears the chosen person today, turns
   My Tasks off as well, and is not shown as active while My Tasks is on.
9. **The active filter chip.** While My Tasks is on, the header's active-filter
   chips and the roadmap's show one chip labelled with the button's label
   ("Mes tâches" / "My Tasks"), not a name. Removing that chip with its × turns
   My Tasks off and leaves the other chips as they are.
10. **Remembered per scope.** My Tasks is remembered with the other filters of
    the project or view it was set on, and restored when that project or view
    is opened again. It is never dropped because no ticket in scope is
    assigned to me: an empty board is a correct answer.
11. **Old remembered values.** A person filter remembered by an earlier version
    of Sectile, holding a name, keeps working as a person filter, as in 8.
12. **The button's tooltip** names the button and:
    - when every tracker in the current scope knows my identity: nothing more;
    - otherwise: lists the trackers in scope on which my name and e-mail are
      used instead, and says that saving (or verifying) a personal credential
      for them in the profile fixes it.
    "Trackers in scope" are the trackers of the tickets the current project or
    view selects, before any other filter.
13. **Not signed in.** When the board is used without an account, "me" is the
    name and e-mail of the local profile, on every tracker, and the tooltip
    says so.
14. `CHANGELOG.md` carries one `Fixed` line under `[Unreleased]`: My Tasks now
    finds the tickets assigned to you on GitHub and Jira.

## Out of scope

- The person picker's own behaviour and its list of names.
- How a ticket gets assigned, and writing an assignment back to the tracker.
- The account display name, settled by #348.
- Matching Jira on the account id rather than the display name (two Jira users
  with the same display name are both "me"; raise separately if it matters).
- Learning the identity of credentials saved before this change without the
  user acting: they are used as soon as they are verified or saved again.

## Acceptance scenarios

- **Given** I hold a personal GitHub credential whose account is
  `sebastienferry`, my account name is `Sébastien F.`, and a GitHub project has
  a ticket assigned to `sebastienferry` and one assigned to `alice`, **when** I
  click My Tasks, **then** only the first ticket is on the board.
- **Given** a GitHub ticket assigned to `SebastienFerry `, **when** My Tasks is
  on with the identity `sebastienferry`, **then** the ticket is shown.
- **Given** I hold a personal Jira credential whose account display name is
  `Sébastien Ferry`, **when** My Tasks is on over a Jira project, **then** the
  tickets assigned to `Sébastien Ferry` are shown and the ones assigned to
  `Sébastien F.` (my account name) are not.
- **Given** "All projects" covers a GitHub project, a Jira project and a local
  project, and I hold both personal credentials, **when** My Tasks is on,
  **then** my GitHub, Jira and local tickets are all shown together, and a
  local ticket assigned to `sebastienferry` is not.
- **Given** a local ticket assigned to my e-mail `SFerry@example.com` typed in
  another case, **when** My Tasks is on, **then** it is shown.
- **Given** I hold no personal Jira credential, **when** My Tasks is on over a
  Jira project, **then** the tickets assigned to my account name or e-mail are
  shown, and the tooltip names Jira and says a personal credential fixes it.
- **Given** a personal Jira credential saved before this change, **when** I
  press "Vérifier" on it in my profile, **then** its identity is learnt and the
  tooltip no longer names Jira.
- **Given** I delete my personal GitHub credential, **when** My Tasks is on
  over a GitHub project, **then** matching falls back to my name and e-mail and
  the tooltip names GitHub.
- **Given** my personal Jira credential is sealed and locked but was confirmed
  when I saved it, **when** My Tasks is on over a Jira project, **then** its
  display name is used and the tooltip does not name Jira.
- **Given** no ticket in scope is assigned to me, **when** I click My Tasks,
  **then** the board is empty, the button stays active, and switching to
  another project and back finds My Tasks still on.
- **Given** My Tasks is on, **when** I rename my account, **then** the button
  stays active and the board still uses my tracker identities.
- **Given** My Tasks is on, **when** I choose `alice` in the person picker,
  **then** My Tasks turns off and the board shows `alice`'s tickets.
- **Given** a saved view whose remembered filters hold `assignee: "Alice"`
  from an earlier version, **when** I open it, **then** it shows `Alice`'s
  tickets and My Tasks is not active.
- **Given** My Tasks is on, **when** I look at the header, **then** one chip
  reads "Mes tâches"; removing it turns the button off.
- **Given** My Tasks and a priority filter are on, **when** I click the
  `#implemented` status shortcut of the sidebar, **then** My Tasks and the
  priority filter turn off and the board shows the implemented tickets.
- **Given** a saved view mixing a GitHub project and a Jira project, with only
  a personal GitHub credential, **when** I hover My Tasks, **then** the tooltip
  names Jira only.
- **Given** loading the board with My Tasks on, **then** no request reaches
  GitHub or Jira.
