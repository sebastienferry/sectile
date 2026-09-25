# ADR 0030: GitLab maps onto labels, board lists and two sprint kinds

Status: Accepted

## Context

Sectile drove GitHub and Jira projects; GitLab parameters and credentials could
be stored (#464), but no adapter was registered, so no project could use GitLab.
Taskativ, Sectile's predecessor, had a GitLab tracker whose mapping leaned on
GitLab's own scoped labels: `workflow::<stage>` for the stage,
`priority::<level>` for the priority, group epics for the macros, and a sprint
kind guessed on write by probing milestones first, then iterations.

#398 brings GitLab back at the level of the Jira adapter. Its clarification is
in `docs/clarifications/398.md`, and its specification is in
`specs/398-recreate-gitlab-tracker-support/`.

GitLab differs from both existing trackers in ways that force choices:

- It has labels, like GitHub, and scoped labels (`scope::value`), which are
  mutually exclusive on Premium and a naming convention on Free.
- A board column is a list, and a list groups the issues carrying its label.
  There is no status field: an issue is opened or closed.
- Two things can be a sprint: a project milestone, on every tier, and a group
  iteration, on Premium only. Iterations are written through GraphQL; REST
  cannot create or assign them.
- Many instances run the Free tier, where iterations do not exist.

## Decision

**The stage is a `#<stage>` label**, exactly as on GitHub and Jira. A closed
issue is finished whatever its labels; finishing a task closes its issue and
moving it out of finished reopens it. GitHub and GitLab share one reading of the
stage labels (`statusFromStageLabels`).

**The macro is a pair of labels**, `macro:<title>` and `parent:<key>`, on every
tier. GitLab group epics are neither read nor written, and a milestone is never
read as a macro, since on GitLab a milestone is a sprint. A GitLab macro gets a
local `M-<n>` key.

**The team is a `team::<name>` scoped label**, the only scoped label Sectile
writes. A team write replaces every other `team::` label of the issue; a GitLab
team has no id of its own, so its name serves as one. Its members are the
project's members.

**A board column is a board list**, plus the implicit Open and Closed. Moving a
card into a list swaps its list labels; into Closed closes the issue; out of
Closed reopens it; into Open drops every list label. A column move never touches
the stage label.

**A sprint names its kind in its id**: `milestone:<id>` or `iteration:<id>`.
Moving a work item into a sprint uses that kind and never probes; a work item
has one sprint, so moving it into a milestone clears its iteration and the
reverse. An id without a kind is refused rather than guessed.

**Premium features degrade on Free.** Whether a group has iterations is asked
once per instance and group, through GraphQL, and remembered for an hour; a
transport failure or a server error is not an answer and is not remembered.
Without iterations the sprint list carries the milestones only and the
synchronisation succeeds; a write that needs iterations is refused as
unsupported, never sent as a milestone. The iterations of an automatic cadence
are GitLab's to schedule: renaming, re-dating or deleting one is refused with a
message saying so. An iteration's state follows its dates and is not set by
hand. Creating sprints from Sectile creates milestones (spec open point O1).

**Credentials follow ADR 0029**, as on GitHub: reads use the acting person's
token where they stored one and the server credential otherwise, writes a person
causes use their own token or are refused, and unattended work uses the server
credential. Every token is a personal access token with the `api` scope, sent as
a bearer token to the configured instance, so gitlab.com and a self-managed
instance are reached the same way.

## Consequences

- GitHub and GitLab read a stage the same way, and a project moved from one to
  the other keeps its stage labels meaningful.
- The ticket panel and the triage table decide what they offer (assignee
  picker, sprint, team) from one table of tracker features, which lists Jira and
  GitLab, instead of a `source === 'jira'` test per component.
- A card's column on GitLab is read from the list labels of every board of the
  project, one extra read per synchronisation rather than one per issue.
- A board that uses a `#<stage>` label as one of its lists makes a column move
  change the stage too. That is the owner's configuration, not a rule.
- Group milestones are listed (`include_ancestors`) and can be assigned, but
  renaming or deleting one goes through the project's milestone endpoint, which
  GitLab refuses for a group milestone.
- A team using milestones for releases and iterations for sprints loses the
  milestone of an issue moved into an iteration from Sectile.

## Rejected alternatives

- **`workflow::` scoped labels for the stage**, as Taskativ did. They would give
  GitLab a mapping of its own, while `#<stage>` already works on GitHub and
  Jira, and a project moved between trackers would lose its stages.
- **Group epics as macros.** Premium only, so a Free project would have no
  macro at all, and a second macro mechanism next to the labels.
- **Probing the sprint kind on write** (milestones first, then iterations).
  A milestone and an iteration can share a number, so the probe could move an
  issue into the wrong sprint; and it costs a read per write.
- **Milestones as macros, as on GitHub.** On GitLab a milestone is the sprint of
  every Free instance; reading it as a macro would leave Free projects without
  sprints.
