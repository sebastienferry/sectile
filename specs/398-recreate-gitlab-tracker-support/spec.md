# #398: Recreate GitLab tracker support

Ticket: https://github.com/sebastienferry/sectile/issues/398
Branch: `feat/398`.
Parent macro: M-8 "Core features".
Clarification: [`docs/clarifications/398.md`](../../docs/clarifications/398.md)
(three rounds, owner-confirmed on 2026-09-25).

## Context

Sectile drives GitHub and Jira projects through tracker adapters. GitLab
settings (instance URL, default project, server token, personal token) can
already be stored, but no GitLab adapter is registered: no project can choose
GitLab, and the web interface hides it. Taskativ, Sectile's predecessor, had a
GitLab tracker; this ticket brings it back, at the level of the Jira adapter.

Out of scope: GitLab group epics, issue weights, the GitLab CI mirror
(`.gitlab-ci.yml`), merge-request stage evidence (delivered by #364), and any
change in behaviour of the GitHub and Jira trackers.

This file states behaviour and acceptance criteria only. Implementation choices
are in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

---

## Decisions being specified

Settled by the owner during the clarification.

- **D1: Jira parity.** Create, read, update, delete, full and incremental
  synchronisation, comments, labels, assignee, transitions (close, reopen,
  board-list moves), sprints read and written, teams, boards, statuses, issue
  types, and merge-request discovery. Epics are not offered (round 2, Q1;
  round 3, Q4).
- **D2: The stage is a `#<stage>` label**, exactly as on GitHub and Jira
  (`#new`, `#clarified`, `#specified`, `#implemented`, `#reviewed`,
  `#finished`). GitLab scoped `workflow::` labels are not used (round 2, Q2).
- **D3: The macro is carried by `macro:<title>` / `parent:<key>` labels**, on
  every GitLab tier. GitLab group epics are neither read nor written
  (round 2, Q3; round 3, Q4).
- **D4: The team is a `team::<name>` scoped label.** It is the only scoped
  label Sectile writes (round 3, Q5).
- **D5: Two kinds of sprint.** Project milestones (every tier) and group
  iterations (Premium) are both sprints; a sprint always says which kind it
  is, so moving a work item never guesses (round 2, reopened point 1).
- **D6: Premium features degrade on Free.** What the instance does not offer
  (iterations) is absent from the lists instead of failing the
  synchronisation; a write that needs it is refused with a clear message
  (round 2, reopened point 2).
- **D7: A board column is a board list.** Moving a work item between columns
  swaps its list label; the Closed list closes it; the `#<stage>` label is not
  touched by a column move (round 2, reopened point 3).

Settled from the code and conventions during the clarification (round 1).

- **D8: gitlab.com and self-managed instances**, addressed by the configured
  API URL; authentication by personal access token (scope `api`).
- **D9: Credentials as on GitHub (#464, ADR 0028).** The background
  synchronisation uses the server GitLab credential; a request somebody makes
  uses their personal GitLab token when they stored one, the server credential
  otherwise; a personal token that is stored but cannot be unlocked refuses the
  request.
- **D10: Finished means closed.** Finishing a work item closes the issue;
  moving it out of `finished` reopens it; a closed issue imports as finished,
  whatever its labels.
- **D11: No new configuration.** The GitLab project comes from the project's
  GitLab project setting, else from the default in the settings; the instance
  URL likewise. No schema change.
- **D12: Keys look like GitLab's own references** (`#12`), and two GitLab
  projects, or a GitLab and a GitHub project, never share a task identity.
- **D13: Comments are the issue's user notes**; GitLab system notes ("changed
  the label", "closed") are not shown as comments.
- **D14: User-visible strings stay in French**, like the rest of the
  interface.

---

## User stories

### US1: Put a project on GitLab (P1)

As a project owner whose issues live on GitLab, I want to choose GitLab as the
project's tracker and name its GitLab project, so that Sectile works on those
issues as it does on GitHub or Jira ones.

**Acceptance**

1. **Given** the project form in the web app, **when** I open the tracker
   choice, **then** GitLab is offered next to Sectile (Local), GitHub Issues
   and Jira.
2. **Given** I choose GitLab, **then** the form asks for the GitLab project
   path (`group/sub/project`) and optionally an instance URL, prefilled with
   the defaults from the settings, and describes GitLab's model in French
   (stage labels, board lists as columns, milestones and iterations as
   sprints).
3. **Given** I save a GitLab project and neither I nor the server has a GitLab
   token, **then** the form warns that the synchronisation will bring nothing
   back, as it does for GitHub.
4. **Given** the personal tracker access screen, **then** GitLab is offered
   and a personal GitLab token can be checked and saved; the check names the
   GitLab account behind the token.
5. **Given** a GitLab project whose instance is self-managed
   (`https://gitlab.example.org/api/v4`), **then** every call goes to that
   instance, never to gitlab.com.

### US2: Synchronise a GitLab project (P1)

As a member of a GitLab project, I want its issues imported and kept up to
date on the board, so that the board reflects GitLab.

**Acceptance**

1. **Given** a GitLab project with 250 open and closed issues, **when** a
   synchronisation runs, **then** all 250 are imported, each with its title,
   description, labels, assignee, author, web link, sprint and macro.
2. **Given** an open issue labelled `#specified`, **then** it is imported at
   the specified stage. **Given** an open issue without a stage label, **then**
   it is imported at the new stage. **Given** a closed issue labelled
   `#clarified`, **then** it is imported as finished.
3. **Given** an issue labelled `macro:Core features` and `parent:M-8`, **then**
   its macro is "Core features" with key M-8.
4. **Given** an issue labelled `team::platform`, **then** its team is
   "platform".
5. **Given** an issue in milestone "Sprint 12" or in iteration "Iteration 7",
   **then** its sprint is that milestone or that iteration; a milestone is a
   sprint whatever its title.
6. **Given** the background synchronisation loop, **when** it runs on a GitLab
   project, **then** it reads only the issues GitLab updated inside its window,
   as it does on GitHub and Jira.
7. **Given** the background synchronisation, **then** it reads with the server
   GitLab credential, never with somebody's personal token.
8. **Given** an issue has merge requests related to it on GitLab, **then**
   they are attached to the task as its pull requests, oldest first, the
   latest being the current one, as on GitHub.
9. **Given** a GitLab project `acme/app` and a GitHub repository both have an
   issue `#12` in Sectile, **then** they are two distinct tasks, and so are
   `#12` of two GitLab projects.
10. **Given** the token is refused, the project does not exist, or GitLab
    answers an error, **then** the synchronisation activity fails with a
    French message naming the problem (GitLab's own error text when it gives
    one), and no token appears in the message or the log.

### US3: Work on GitLab issues from Sectile (P1)

As a member, I want what I do on a GitLab task to reach GitLab, attributed to
me when I stored my own token.

**Acceptance**

1. **Given** I create a task in a GitLab project, **then** a GitLab issue is
   created with its title, description, labels, stage label and, when chosen,
   issue type (`issue`, `incident`, `task`, `test_case`), assignee, team and
   sprint; the task gets the issue's key and web link.
2. **Given** I edit a task's title, description or labels, **then** the issue
   changes accordingly, labels others added on GitLab being kept.
3. **Given** a task moves from one stage to another, **then** the issue's
   `#<stage>` label is replaced by the new one and no other stage label
   remains.
4. **Given** a task reaches finished, **then** the issue is closed; **given**
   a finished task moves back to another stage, **then** the issue is
   reopened.
5. **Given** I delete a task, **then** the issue is closed, or deleted when
   the deletion asks for it and the token is allowed to delete.
6. **Given** I comment on a task, **then** a note is posted on the issue;
   **given** I open the task, **then** its comments are the issue's user notes,
   oldest first, without system notes.
7. **Given** I assign a task, **then** the assignee picker lists the GitLab
   project's members matching what I type, and the chosen member becomes the
   issue's assignee; clearing it unassigns the issue.
8. **Given** I stored a personal GitLab token, **then** the issue, note or
   change is made under my GitLab account; **given** I stored none, **then** it
   is made under the server credential; **given** mine is sealed and locked,
   **then** the action is refused with a message asking me to unlock it.
9. **Given** a task is attached to a macro, or created under one, **then** the
   issue carries the `macro:` and `parent:` labels of that macro, replacing
   previous ones.

### US4: Boards, columns and statuses (P2)

As a project owner, I want to pick one of the GitLab project's boards and map
its lists to Sectile's columns, so that the board mirrors GitLab's.

**Acceptance**

1. **Given** a GitLab project with two boards, **when** I choose the board in
   the project options, **then** both are listed by name.
2. **Given** a board with lists "To Do" and "Doing", **then** its columns are
   Open, To Do, Doing and Closed, in board order, each list grouping its label.
3. **Given** the status list used to map columns, **then** it holds `opened`,
   `closed` and every list label of the project's boards.
4. **Given** I drag a task from the "To Do" column to the "Doing" column,
   **then** the issue loses the "To Do" label and gains the "Doing" label; its
   `#<stage>` label is unchanged.
5. **Given** I drag a task into the Closed column, **then** the issue is
   closed; **given** I drag it out of Closed into a list, **then** it is
   reopened and gains that list's label; **given** I drag it into Open,
   **then** every list label is removed.
6. **Given** a project with no board, **then** the board choice says so and
   the default columns are used.

### US5: Sprints on GitLab (P2)

As a project owner, I want GitLab milestones and iterations to be Sectile
sprints, readable and writable.

**Acceptance**

1. **Given** a Premium group with iterations and a project with milestones,
   **then** the sprint list shows both, each with its name, state (active,
   future, closed) and dates, and says for each whether it is a milestone or
   an iteration.
2. **Given** a Free instance, **then** the sprint list shows the milestones
   only, and the synchronisation succeeds; it does not report an error for the
   missing iterations.
3. **Given** I move tasks into a milestone sprint, **then** each issue's
   milestone is set; into an iteration sprint, **then** each issue's iteration
   is set; out of any sprint, **then** both are cleared.
4. **Given** I create sprints from Sectile, **then** they are created on GitLab
   as milestones with their dates (see open point O1 for iterations).
5. **Given** I rename, re-date, close or reopen a milestone or an iteration of
   a manual cadence, **then** GitLab reflects it; **given** I delete one,
   **then** it is deleted, and one GitLab no longer knows counts as deleted.
6. **Given** an iteration of an automatically scheduled cadence, **when** I
   try to change or delete it, **then** Sectile refuses with a French message
   saying the cadence schedules it and it must be changed on GitLab.
7. **Given** a Free instance, **when** a write needs iterations, **then** it is
   refused as not supported by this GitLab instance, never sent as a milestone.

### US6: Teams on GitLab (P3)

As a project owner, I want teams on GitLab, carried by `team::` labels.

**Acceptance**

1. **Given** the project has labels `team::platform` and `team::data`, **when**
   I search teams, **then** "platform" and "data" are found; a search narrows
   them by name.
2. **Given** I set the team of one or several GitLab tasks, **then** each issue
   gets the `team::<name>` label and loses any other `team::` label; clearing
   the team removes it.
3. **Given** I open a team's members, **then** they are the GitLab project's
   members.
4. **Given** a GitHub task, **then** setting its team is still refused, as
   today.

### US7: GitLab is documented (P3)

As a Sectile user or operator, I want the documentation and the release notes
to say GitLab is supported and how it maps.

**Acceptance**

1. **Given** the README tracker section, **then** it describes GitLab setup
   (API URL, project, token and scope) and no longer says no adapter exists.
2. **Given** `docs/CAPABILITIES.md`, **then** it lists GitLab's capabilities.
3. **Given** `CHANGELOG.md`, **then** `[Unreleased]` / `Added` has one line
   announcing GitLab as a tracker.
4. **Given** `docs/adrs/`, **then** an ADR records the GitLab mapping: labels
   for stage and macro, scoped label for team, board lists as columns, two
   sprint kinds, Premium degradation.

---

## Functional requirements

- **FR-1** A project whose tracker is `gitlab` is synchronised, read, created
  in, updated, commented and closed through the GitLab REST API v4 of the
  configured instance (US1, US2, US3).
- **FR-2** GitLab declares the capabilities `create`, `get`, `update`,
  `delete`, `sync`, `incremental_sync`, `comment`, `labels`, `assign`,
  `transition`, `sprint`, `sprint_manage`, `team`, `board` and
  `pull_requests`, and not `epic`. Every interface that shows or hides a
  feature from the capabilities does so for GitLab with no GitLab-specific
  rule (D1).
- **FR-3** The stage mapping is the GitHub one: `#new` → new, `#clarified` →
  clarified, `#specified` → specified, `#implemented` → implemented,
  `#reviewed` → reviewed, `#finished` → finished; closed → finished; a stage
  write keeps exactly one stage label (D2, D10).
- **FR-4** The macro is read from and written to `macro:` / `parent:` labels
  only; GitLab milestones are never read as macros (D3, D5).
- **FR-5** The team is read from and written to one `team::<name>` label (D4).
- **FR-6** A sprint identifier names its kind; moving a work item into a
  sprint uses that kind and never probes (D5).
- **FR-7** Iterations are detected once per instance and group and cached for
  a bounded time; their absence (Free tier, no group, no permission) removes
  them from reads and makes iteration writes answer "not supported" (D6).
- **FR-8** Board columns are the board's lists plus Open and Closed; a column
  move swaps list labels or closes/reopens, and leaves the stage label (D7).
- **FR-9** `ListStatuses` returns `opened`, `closed` and the list labels;
  `ListIssueTypes` returns `issue`, `incident`, `task`, `test_case`;
  creation requires no extra field.
- **FR-10** Credentials follow D9; the GitLab project and URL follow D11;
  nothing is read from or derived out of the Git remote.
- **FR-11** Task identities are namespaced per GitLab project; keys are
  `#<iid>`; a GitLab task's web link is the issue's `web_url` (D12).
- **FR-12** Every GitLab error reaching an activity or the interface is in
  French, quotes GitLab's own message when it has one, and never carries a
  token.
- **FR-13** GitHub and Jira behaviour is unchanged; their existing tests pass
  unmodified.

---

## Edge cases

- A project path with subgroups (`acme/platform/app`) is URL-encoded as one
  path segment; the group for iterations is `acme/platform`. A project in a
  personal namespace (`alice/app`) has no iterations, since iterations belong
  to groups; its milestones still work. A numeric project id is accepted; its
  group is the namespace GitLab reports for the project.
- More than 100 issues, notes, labels, members or milestones: every page is
  read.
- An issue carries two `#<stage>` labels (edited by hand on GitLab): the
  import picks one as GitHub does today, and the next stage write leaves one.
- An issue carries two `team::` labels despite GitLab's exclusivity (Free
  tier does not enforce it): the first is read, the next team write leaves one.
- A column list whose label was deleted on GitLab: the column is still listed,
  and a move into it fails with GitLab's message.
- A sprint identifier without a kind (a value stored before this ticket, or
  typed by hand) is refused with a message, not guessed.
- GitLab rate limits (HTTP 429): reported as a rate limit, like GitHub's, so
  the background loop backs off.
- A self-managed instance with a self-signed certificate is out of scope; the
  server's trust store applies.

---

## Open points

- **O1: Which kind of sprint does "create sprints" create on GitLab?** The
  clarification says sprint creation works on milestones and on iterations of
  a manual cadence, but not how Sectile picks one when the owner creates
  sprints: the project has one board and no cadence setting. **Blocks**: the
  iteration branch of `CreateSprint` only (tasks T5.4). Until decided, US5-4
  creates milestones, on every tier; rename, re-date, close and delete of an
  existing iteration are not blocked. Candidate answers: always milestones;
  iterations on the group's only manual cadence when there is exactly one,
  milestones otherwise; a cadence choice in the project options.
- **O2: May a GitLab task join a macro of another GitLab project?** The
  labels would allow it on the same instance, but the clarification did not
  address it. The plan follows GitHub's rule (same instance and same project),
  which is the conservative choice and easy to widen. **Blocks**: nothing.
