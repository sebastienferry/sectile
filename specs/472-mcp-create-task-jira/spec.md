# #472: MCP create_task creates the work item on Jira

Ticket: https://github.com/sebastienferry/sectile/issues/472
Branch: `feat/472`.
Clarification: [`docs/clarifications/472.md`](../../docs/clarifications/472.md)
(confirmed in Round 2, addendum after #464).

## Context

An agent that files a follow-up ticket through the MCP tool `create_task` gets
`remote task creation is not supported for tracker "jira"` on every Jira-backed
project, and nothing is created anywhere. The same project accepts creations
from the web board. On GitHub-backed projects the tool works, but the issue
type and the parent it is given never reach the tracker. `get_task` on a Jira
ticket returns the task without its comments and reports
`configure the Jira account e-mail`, because it reads them under no one's name.

This file states behaviour and acceptance criteria only. Implementation choices
are in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

Out of scope: an `assignee` or a custom-field input on `create_task`; the
credential used by `add_comment`, `update_task`, `transition_stage` and the
other MCP tools; the web creation form; creating Jira projects or issue types.

## Terms

- **Caller**: the Sectile user an MCP call is authenticated as, resolved from
  the credential of the request that carried it. A transport that carries no
  credential names no caller.
- **Personal credential**: the tracker token a user stored in their own
  profile. It may be sealed behind a passphrase and locked.
- **Server credential**: the one credential per provider an admin configures
  (ADR 0028), used by work nobody asked for.
- **Remote creation required**: the contract of `create_task` and of the
  desktop quick-create: the ticket exists on the project's tracker, or the call
  fails and nothing is recorded.

## Decisions being specified

From the clarification, restated:

1. `create_task` acts as its caller: a Jira issue is created with the caller's
   personal credential, and the call fails when they have none or it is locked.
   The server credential is used only when no caller is named.
2. `get_task` reads a ticket's comments as its caller, like the web detail view.
3. `create_task` takes no custom-field input; a refusal over a mandatory field
   says which fields the project requires.
4. No assignee: Jira's project default applies.

## User stories

### US1 (P1): An agent files a ticket on a Jira project

An agent working for a signed-in user calls `create_task` on a Jira-backed
project, and the ticket appears on Jira under that user's name, attached to
the right epic, and on the local board.

- **Given** a Jira-backed project and a caller with a usable personal Jira
  credential, **when** `create_task` is called with a title, a Markdown
  description, an issue type, a priority, labels and a parent key, **then** one
  Jira issue is created in the project's Jira project, and the call returns the
  task with the Jira key and the issue's browse URL.
- **Given** that creation, **then** the Jira issue has the requested summary,
  issue type, priority and labels, its description is rendered from the
  Markdown (headings, lists, code, links), and its parent is the given key.
- **Given** that creation, **then** Jira attributes the issue to the caller's
  account, not to the server's.
- **Given** that creation, **then** the task is on the local board of that
  project, at the first workflow stage, carrying the single creation label and
  the caller's other labels.
- **Given** no issue type, **then** the project's default issue type is used,
  as on the web board.

### US2 (P1): A creation that cannot happen says why, and leaves nothing behind

- **Given** a caller with no personal Jira credential, **when** `create_task`
  targets a Jira-backed project, **then** the call fails with an error saying
  that a personal Jira token is needed in the profile, and neither Jira nor the
  local board gets a ticket.
- **Given** a caller whose personal Jira credential is sealed and locked,
  **then** the call fails with an error saying the credential is sealed and
  must be unlocked by its owner, and nothing is created.
- **Given** a project whose tracker Sectile cannot create on (no adapter for it,
  or an adapter without creation), **when** remote creation is required,
  **then** the call fails naming the tracker, and nothing is created locally.
- **Given** a project with a local board only, **then** `create_task` still
  creates the task locally.
- **Given** Jira refuses the creation (unknown issue type, invalid parent,
  missing mandatory field), **then** the error starts with
  `Jira issue creation failed:` and quotes Jira's reason, and nothing is
  created locally.
- None of these errors reads "not supported" for a Jira-backed project.

### US3 (P2): A project with mandatory fields is explained, not just refused

- **Given** a Jira project whose creation screen makes a custom field
  mandatory for the requested issue type, **when** `create_task` is refused for
  it, **then** the error lists the fields the project requires on creation for
  that issue type, by name and field id.
- **Given** the list of required fields cannot be read, **then** the error is
  still Jira's own refusal, unchanged.

### US4 (P1): An agent reads a Jira ticket's discussion

- **Given** a Jira ticket and a caller with a usable personal Jira credential,
  **when** `get_task` is called, **then** the result carries the ticket's Jira
  comments, read with the caller's credential.
- **Given** a caller with no personal Jira credential, or a locked one,
  **then** `get_task` still returns the task, and `commentsError` says why the
  comments could not be read, as the web detail view does for that user.

### US5 (P2): GitHub creation keeps working, as its caller

- **Given** a GitHub-backed project, **when** `create_task` is called, **then**
  the issue is created as before, with the caller's personal GitHub token when
  they stored one and the server credential otherwise.

### US6 (P3): Other entry points gain the same creation

- **Given** a Jira-backed project, **when** the desktop quick-create files a
  ticket, **then** it is created on Jira under the signed-in user, with the
  same errors as US2.
- **Given** the web board creates a ticket with an issue type or a parent,
  **then** the issue type and the parent reach the tracker, not only the local
  row.

## Functional requirements

- **FR1** When remote creation is required and the project's tracker is not the
  local board, the ticket is created on the tracker through its adapter,
  whichever tracker it is, provided the adapter can create; Jira is no longer
  excluded.
- **FR2** When remote creation is required and the tracker cannot be resolved,
  is not the one requested, or cannot create, the call fails naming the tracker
  and no task row is written.
- **FR3** The issue type and the parent key given at creation are sent to the
  tracker, for every creation path that goes through the board's task creation.
- **FR4** `create_task` runs on behalf of its caller when one is named, and with
  no acting user otherwise.
- **FR5** `get_task` reads comments on behalf of its caller when one is named,
  and with no acting user otherwise.
- **FR6** A tracker refusal is reported as `<Tracker> issue creation failed:
  <cause>`, with the cause kept inspectable (a locked credential is still
  recognisable as such by callers).
- **FR7** A Jira refusal on creation is completed, when the site can list them,
  with the fields the project requires on creation for that issue type that the
  request did not provide.
- **FR8** A successful creation costs no request beyond what it costs today.
- **FR9** `CHANGELOG.md` carries one `Fixed` line under `[Unreleased]`.

## Success criteria

- The repro of the ticket (Jira project, `issueType: Task`, `parentKey`) creates
  one Jira issue under the caller, linked to its parent, and one local task.
- Every refusal of US2 leaves the tasks table unchanged.
- The existing GitHub `create_task` test passes unchanged in its assertions.
- No test reaches a real tracker.

## Open requirements

None. Every product question was settled in the clarification.
