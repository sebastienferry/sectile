## ADDED Requirements

### Requirement: A task holds an ordered set of pull request links
The system SHALL store the pull requests of a task as an ordered set of links, each keeping its
URL and the branch it was opened from. The system SHALL keep the task's single current pull
request URL equal to the last link of that set, and SHALL NOT let the two diverge.

#### Scenario: A follow-up pull request is added to the set
- **GIVEN** a task whose set holds one pull request on its branch
- **WHEN** a later stage records a different pull request on the same branch
- **THEN** the set holds both links in the order they were recorded
- **AND** the task's current pull request URL is the newly recorded one

#### Scenario: Recording the same pull request twice
- **GIVEN** a task whose set already holds a pull request
- **WHEN** the same URL is recorded again
- **THEN** the set is unchanged and holds that link once

#### Scenario: A task with no pull request
- **GIVEN** a task that has never had a pull request recorded
- **WHEN** it is read
- **THEN** its set of links is empty and it carries no current pull request URL

#### Scenario: Existing single links are migrated
- **GIVEN** a stored task that carries a pull request URL recorded before this capability existed
- **WHEN** the database is opened by the current version
- **THEN** the task's set holds exactly that one link, carrying the task's branch
- **AND** its current pull request URL is unchanged
- **AND** re-opening the database does not add a second copy of that link

### Requirement: Stage validation reasons over the recorded set
The system SHALL validate the pull request evidence of a stage against the task's recorded set
rather than against a single recorded URL. A recorded pull request SHALL NOT veto a pull request
that shares a branch with any recorded link. The system SHALL refuse a pull request whose branch
matches none of the recorded links.

#### Scenario: A newer pull request on a branch whose recorded one was merged
- **GIVEN** a task whose recorded pull request on its branch has been merged
- **AND** a newer pull request on that same branch, whose head commit is the agent checkout commit
- **WHEN** an implemented or reviewed transition is recorded
- **THEN** the transition succeeds
- **AND** the newer pull request is appended to the set and becomes the current one

#### Scenario: A pull request swapped for an unrelated one
- **GIVEN** a task whose recorded links are all on one branch
- **WHEN** a transition presents a pull request on a branch that matches no recorded link
- **THEN** the transition is refused and names the recorded branch it expected
- **AND** the set is left untouched

#### Scenario: The first pull request of a task
- **GIVEN** a task with an empty set of links
- **WHEN** a stage records a valid pull request for its branch
- **THEN** the transition succeeds and the link becomes the first of the set

#### Scenario: The pull request evidence is otherwise invalid
- **GIVEN** a pull request that is closed without having been merged, or whose head commit is not
  the agent checkout commit
- **WHEN** a transition is evaluated
- **THEN** it is refused as before, whatever the recorded set contains

### Requirement: Forge evidence over a branch carrying several pull requests
The system SHALL resolve the pull request evidence of a branch that carries several pull requests.
When the branch has exactly one open pull request the system SHALL select it. When the branch has
no open pull request and one or more merged ones the system SHALL select the most recently merged.
The system SHALL refuse to choose between several open pull requests on one branch.

#### Scenario: Two merged pull requests and none open
- **GIVEN** a branch whose only pull requests are two merged ones
- **WHEN** the stage evidence is looked up
- **THEN** the most recently merged pull request is returned as the evidence

#### Scenario: An open pull request outranks merged ones
- **GIVEN** a branch carrying merged pull requests and one open one
- **WHEN** the stage evidence is looked up
- **THEN** the open pull request is returned and the usual readiness gate applies to it

#### Scenario: Several open pull requests
- **GIVEN** a branch carrying more than one open pull request
- **WHEN** the stage evidence is looked up
- **THEN** the lookup fails and reports how many open pull requests it found
- **AND** no transition is recorded on that evidence

#### Scenario: Only abandoned pull requests
- **GIVEN** a branch whose pull requests were all closed without being merged
- **WHEN** the stage evidence is looked up
- **THEN** the lookup fails and no pull request is treated as evidence

### Requirement: The pull request links of a task are editable from the web UI
The web UI SHALL let a human add a pull request link to a task, correct the URL of an existing
link, and detach a link, without any direct database access. The system SHALL keep the task's
current pull request URL consistent with the edited set.

#### Scenario: Adding a link
- **GIVEN** a task open in the task detail modal
- **WHEN** the user adds a pull request URL and saves
- **THEN** the link is appended to the task's set with the task's branch
- **AND** it becomes the task's current pull request

#### Scenario: Correcting a wrong link
- **GIVEN** a task whose recorded pull request URL is wrong
- **WHEN** the user edits that URL in place and saves
- **THEN** the corrected URL replaces it at the same position in the set

#### Scenario: Detaching a link
- **GIVEN** a task carrying two pull request links
- **WHEN** the user detaches the last one and saves
- **THEN** the set holds only the remaining link
- **AND** the task's current pull request URL is that remaining link

#### Scenario: Detaching every link
- **GIVEN** a task carrying a single pull request link
- **WHEN** the user detaches it and saves
- **THEN** the task carries no current pull request URL
- **AND** the next stage that records a pull request treats it as the first of the set

#### Scenario: Leaving the links untouched
- **GIVEN** a task open in the modal with no edit made to its links
- **WHEN** the modal is closed
- **THEN** no unsaved-changes prompt is raised on account of the links
- **AND** the stored set is unchanged
