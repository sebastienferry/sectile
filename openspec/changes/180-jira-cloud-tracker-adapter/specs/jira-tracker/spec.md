# jira-tracker

## Purpose
A project whose issue tracker is Jira Cloud is driven from Sectile through the
Jira REST API: its work items are imported and kept current, created, updated,
commented, assigned, closed, moved between sprints, attached to a team and to
an epic, with credentials configured and verified from the interface.

## ADDED Requirements

### Requirement: A Jira Cloud project is a first-class tracker
The registry SHALL resolve a project whose `issueTracker` is `jira`, and a task
whose `source` is `jira`, to a Jira adapter that supports create, get, update,
delete, sync, comment, labels, assign, transition, sprint, team, epic and board
reads. The adapter SHALL reach the site over HTTPS only, authenticating with
`Basic base64(email:token)`, and SHALL never execute a workstation command.

#### Scenario: Resolve the adapter for a Jira project
- **GIVEN** a project with `issueTracker: jira`
- **WHEN** the server resolves the project's tracker
- **THEN** it obtains an adapter named `jira`
- **AND** `Supports` answers true for every capability listed above.

#### Scenario: Refuse to work without credentials
- **GIVEN** a Jira project with no e-mail or no token resolved
- **WHEN** any Jira operation runs
- **THEN** it fails before any network call with a message naming the missing Jira credential.

### Requirement: Credentials are resolved per request, stored values first
The server SHALL resolve the Jira site URL from the project `trackerUrl`, then
the user configuration `jiraUrl`, then `SECTILE_JIRA_URL`; the e-mail from the
user configuration `jiraEmail`, then `SECTILE_JIRA_EMAIL`; the token from the
user configuration `jiraApiToken`, then `SECTILE_JIRA_TOKEN`,
`SECTILE_TRACKER_TOKEN`, `JIRA_API_TOKEN`. The API SHALL report
`jiraApiTokenFromEnv: true` when no token is stored and the environment supplies
one. Tokens SHALL stay write-only under the existing empty-means-unchanged and
`__clear__` rules.

#### Scenario: A typed token supersedes an exported one
- **GIVEN** a server started with `JIRA_API_TOKEN` exported
- **WHEN** the user saves a different Jira token from the interface
- **AND** a Jira request runs
- **THEN** the request authenticates with the saved token.

#### Scenario: A project reaches its own site
- **GIVEN** a user configuration pointing at `https://acme.atlassian.net`
- **AND** a project whose `trackerUrl` is `https://other.atlassian.net`
- **WHEN** a Jira request runs on that project
- **THEN** it addresses `other.atlassian.net`
- **AND** a request on a project with no `trackerUrl` addresses `acme.atlassian.net`.

#### Scenario: Report a token supplied by the environment
- **GIVEN** `SECTILE_JIRA_TOKEN` exported and no stored Jira token
- **WHEN** a client reads the settings
- **THEN** the response carries `jiraApiTokenSet: false` and `jiraApiTokenFromEnv: true`.

### Requirement: Credentials are verified before they are saved
The tracker setup SHALL check a Jira site URL, e-mail and token against the
site's own identity endpoint and SHALL answer with the account's display name.
A failed check SHALL persist nothing. A successful save SHALL store the site
URL, the e-mail, the project key upper-cased and the token in the user
configuration. The setup screen SHALL NOT offer to store the token in a file.

#### Scenario: Check a valid token
- **GIVEN** a site, an e-mail and a token the site accepts
- **WHEN** the user checks them from the setup screen
- **THEN** the answer names the account the token belongs to.

#### Scenario: Refuse a stale token
- **GIVEN** a token the site rejects
- **WHEN** the user saves it
- **THEN** the save is refused with the site's error
- **AND** the previously stored Jira configuration is unchanged.

### Requirement: Work items are imported with their Sectile identity
Synchronising a Jira project SHALL read every work item of the project, limited
to the project's configured issue types when set, ordered by last update, page
by page until the site reports the last page. Each imported task SHALL carry
`source: jira`, the Jira key as `key`, `jira-<projectID>-<KEY>` as `id`, the
site's `/browse/<KEY>` URL, its labels, its Jira status name as tracker status,
its priority mapped by name (`Highest`/`High` → high, `Medium` → medium,
`Low`/`Lowest` → low), its assignee's account id, its parent key, title and
type, its sprint and its team when the site exposes those fields.

#### Scenario: Import a paginated project
- **GIVEN** a Jira project of 250 work items
- **WHEN** a sync runs
- **THEN** three pages are read
- **AND** 250 tasks are stored with `jira-<projectID>-<KEY>` identifiers.

#### Scenario: Restrict the import to the configured types
- **GIVEN** a project whose `issueTypes` is `["Story", "Bug"]`
- **WHEN** a sync runs
- **THEN** the search query restricts the issue type to Story and Bug.

#### Scenario: Sync a site without the sprint and team fields
- **GIVEN** a site exposing neither a Sprint nor a Team custom field
- **WHEN** a sync runs
- **THEN** the tasks are imported with empty sprint and team
- **AND** the sync succeeds.

### Requirement: The Sectile status follows the workflow labels, then the Jira status category
The import SHALL give a work item carrying a workflow label (`new`,
`clarified`, `specified`, `implemented`, `reviewed`, `finished`) the Sectile
status that label denotes. Without such a label, a work item whose Jira status
category is `done` SHALL be `finished`, and any other SHALL be `to_clarify`.

#### Scenario: A labelled work item
- **GIVEN** a work item labelled `specified` in status "In Progress"
- **WHEN** it is imported
- **THEN** its Sectile status is `to_implement`.

#### Scenario: A resolved work item without label
- **GIVEN** a work item with no workflow label in a status of category `done`
- **WHEN** it is imported
- **THEN** its Sectile status is `finished`.

### Requirement: Finishing a task moves the work item to a done status
When a task is finished or deleted from Sectile, the adapter SHALL run the first
transition available on the work item whose target status category is `done`.
When the workflow offers none, the operation SHALL fail with a message naming
the work item and the transitions that were available. A stage change that only
changes a label SHALL NOT transition the work item. An update naming a target
status SHALL run the transition whose target status has that name,
case-insensitively.

#### Scenario: Close a work item
- **GIVEN** a work item whose available transitions lead to "In Review" and "Done" (category `done`)
- **WHEN** its task is finished in Sectile
- **THEN** the "Done" transition is executed.

#### Scenario: No closing transition exists
- **GIVEN** a work item whose only available transition leads to "In Progress"
- **WHEN** its task is finished in Sectile
- **THEN** the write fails with a message naming the work item and "In Progress"
- **AND** the local task keeps its finished status.

#### Scenario: Move a card without touching the Jira status
- **GIVEN** a work item labelled `clarified`
- **WHEN** its task advances to `specified` in Sectile
- **THEN** the work item's labels become `specified` without `clarified`
- **AND** no transition is executed.

### Requirement: Text travels as Atlassian Document Format
Descriptions and comments written from Sectile SHALL be converted from Markdown
to ADF, covering paragraphs, headings 1 to 3, bullet and ordered lists with one
level of nesting, fenced code blocks, inline code, bold, italics and links.
Descriptions and comments read from Jira SHALL be converted from ADF to
Markdown for the same constructs, mentions rendered as `@name`, and unknown
nodes flattened to their text.

#### Scenario: Post a clarification report
- **GIVEN** a Markdown comment with a heading, a bullet list and a code block
- **WHEN** it is added to a work item
- **THEN** the work item shows a heading, a bullet list and a code block.

#### Scenario: Read a description containing a mention and a table
- **GIVEN** a description with a mention of "Ada" and a table
- **WHEN** the task is imported
- **THEN** its description contains `@Ada`
- **AND** the table's cell texts, in reading order.

### Requirement: Work items are created with the fields the site requires
Creating a work item SHALL send the project key, the summary, the ADF
description, the labels, the issue type from the request, else the first
configured project issue type, else `Task`, the parent key when given, and any
extra fields the caller supplies for instance-mandatory fields. The created
task SHALL be returned with its key and URL. A refusal by the site SHALL be
reported with the site's own message.

#### Scenario: Create a story under an epic
- **GIVEN** a project whose first issue type is Story
- **WHEN** a task is created with a parent key `PE-10`
- **THEN** a Story is created in the project with parent `PE-10`
- **AND** the task returned carries the new key.

#### Scenario: The site requires an extra field
- **GIVEN** an issue type whose creation requires "Epic Type"
- **WHEN** a task is created without it
- **THEN** the creation fails with a message containing "Epic Type".

### Requirement: Comments are read from and written to the work item
Adding a comment SHALL post it to the work item. Reading comments SHALL return
them oldest first with their author display name, creation time and `source:
jira`.

#### Scenario: Round trip a comment
- **GIVEN** a work item with no comment
- **WHEN** a comment is added, then the comments are read
- **THEN** one comment is returned with the posted text, an author and a creation time.

### Requirement: Assignment uses account identifiers
Assigning a work item SHALL send the assignee's account id, or clear the
assignee when the id is empty. Searching assignable people SHALL query the
site for the work item and return account id, display name, e-mail, avatar and
active state.

#### Scenario: Assign by account id
- **GIVEN** an account id returned by an assignable search
- **WHEN** the work item is assigned to it
- **THEN** the work item's assignee is that account.

### Requirement: Sprints, boards and columns come from the site's Agile boards
The adapter SHALL list the boards attached to a project, the sprints of a board
with their state and dates, and the columns of a board with the status names
they group. Moving work items into a sprint SHALL address the sprint by id in
batches of at most 50 keys; an empty sprint id SHALL move them back to the
backlog. After a sync of a project with a selected board, the project's columns
SHALL be refreshed preserving hand-assigned statuses and hidden columns, and
its sprint list SHALL be refreshed.

#### Scenario: Move sixty work items into a sprint
- **GIVEN** sixty selected tasks and a sprint id
- **WHEN** they are moved to the sprint
- **THEN** two requests are sent, of 50 and 10 keys.

#### Scenario: Import the board columns
- **GIVEN** a project with a selected board of columns "To Do", "Doing", "Done"
- **WHEN** a sync completes
- **THEN** the project's tracker columns are those three, each with its status names.

### Requirement: Teams are found, read and written
Searching teams SHALL return the site's teams matching a name. Reading a team's
members SHALL return the full members with their account id, display name,
e-mail, avatar and active state. Setting a work item's team SHALL write the
team id to the site's Team field, or clear it. A failure to read a team's
members SHALL NOT fail the operation that triggered it; the team is kept with
its members unknown.

#### Scenario: Refresh a team's members
- **GIVEN** a team id met on the project's work items
- **WHEN** its members are refreshed
- **THEN** the members are stored with their account ids and display names.

#### Scenario: The members endpoint is unavailable
- **GIVEN** a site whose team members endpoint answers an error
- **WHEN** a sync runs
- **THEN** the work items are imported
- **AND** the sync reports the teams whose members could not be read.

### Requirement: Epics are read and attached
Listing the epics of a project SHALL return its work items of type Epic.
Attaching a work item to an epic SHALL set its parent, and an empty epic key
SHALL detach it.

#### Scenario: Attach and detach
- **GIVEN** a work item with no parent
- **WHEN** it is attached to `PE-10`, then detached
- **THEN** its parent is `PE-10` after the first write and empty after the second.

### Requirement: The interface no longer claims Jira is unsupported
The Jira sync endpoint SHALL enqueue a sync like the GitHub one. Project status
listing SHALL answer the project's Jira statuses. The sync view SHALL describe
the REST synchronisation. The documentation SHALL describe Jira as supported
over REST.

#### Scenario: Trigger a Jira sync
- **GIVEN** a Jira project with a saved token
- **WHEN** the user triggers the Jira sync
- **THEN** a sync activity is queued and completes with the number of imported work items.

### Requirement: Rate limiting is respected
A `429` answer from the site SHALL surface as a rate-limit error that the
auto-sync loop recognises to back off.

#### Scenario: The site asks to slow down
- **GIVEN** a site answering 429 to a sync
- **WHEN** the auto-sync loop runs
- **THEN** the loop records a back-off and does not retry before it expires.
