## ADDED Requirements

### Requirement: Every user holds one of two roles
Every Sectile user SHALL hold exactly one role, `admin` or `member`. A user created
without an explicit role SHALL be `member`, except that the first user created while
no admin exists SHALL be `admin`. The interface SHALL show the signed-in user their
own role.

#### Scenario: First account becomes admin
- **GIVEN** a server with no admin user
- **WHEN** a person signs in for the first time
- **THEN** their account holds the role `admin`

#### Scenario: Later accounts are members
- **GIVEN** a server that already has an admin
- **WHEN** another person signs in for the first time
- **THEN** their account holds the role `member`

### Requirement: Admin-only actions
The server SHALL reserve to admins the global settings, the tracker credentials,
project creation, project settings and project deletion, the users list, role changes,
and the workstations of other users. A member attempting one SHALL receive a refusal that names the
missing role, distinct from the refusal given to an anonymous caller. Editing tasks,
recording workflow transitions, commenting and launching executions on one's own agent
SHALL remain available to every signed-in user.

#### Scenario: Member edits global settings
- **GIVEN** a signed-in member
- **WHEN** they save the global settings or the tracker credentials
- **THEN** the change is refused as not allowed for their role
- **AND** the stored values are unchanged

#### Scenario: Admin edits global settings
- **GIVEN** a signed-in admin
- **WHEN** they save the global settings
- **THEN** the change is stored

#### Scenario: Member works on tasks
- **GIVEN** a signed-in member
- **WHEN** they edit a task, transition its stage, comment on it or launch a skill on their own agent
- **THEN** each action succeeds as it did before roles existed

#### Scenario: Anonymous caller is told to sign in
- **GIVEN** a request with no session and no API key on a guarded route
- **WHEN** it reaches the server
- **THEN** it is refused as unauthenticated, not as lacking a role

### Requirement: Admins manage roles from the users view
An admin SHALL see every user with their e-mail, display name, role and last sign-in,
and SHALL be able to change a user's role. The last remaining admin SHALL NOT be
demoted. When the identity provider supplies roles, the view SHALL say that a manual
change lasts until that user's next sign-in.

#### Scenario: Promote a member
- **GIVEN** a signed-in admin and a member
- **WHEN** the admin sets the member's role to `admin`
- **THEN** the member's next request is treated as an admin's

#### Scenario: Last admin cannot demote themselves
- **GIVEN** a server whose only admin is signed in
- **WHEN** they set their own role to `member`
- **THEN** the change is refused with a message saying the board would have no admin
- **AND** their role is unchanged

### Requirement: The identity provider is the authority on roles when it supplies them
Each sign-in SHALL set the user's role from the provider's claim when an OpenID Connect
provider is configured together with a role claim name and an admin group value: `admin` when
one of its values equals the configured admin group, `member` otherwise, including
when the claim is absent. The claim MAY be a single string or a list of strings.
When no role claim is configured, the stored role SHALL remain the authority and the
first-admin rule applies.

#### Scenario: Group grants admin
- **GIVEN** a provider configured with a role claim and an admin group
- **WHEN** a person whose claim lists that group signs in
- **THEN** they are `admin` for the session that follows

#### Scenario: Missing group means member
- **GIVEN** the same provider
- **WHEN** a person whose claim lists other groups, or no claim at all, signs in
- **THEN** they are `member`, even if an admin had promoted them earlier

#### Scenario: Provider without a role claim
- **GIVEN** a provider configured without a role claim name
- **WHEN** people sign in
- **THEN** the first of them becomes admin and later role changes made by an admin persist

### Requirement: Machine callers hold the role of their user
An API key SHALL confer the role of the user it is bound to, on every machine surface.
The agent identity endpoint SHALL report that role.

#### Scenario: MCP client of a member
- **GIVEN** an API key bound to a member
- **WHEN** an MCP client or a desktop app uses it for an admin-only action
- **THEN** the action is refused as not allowed for the role

#### Scenario: Identity reports the role
- **GIVEN** an API key bound to an admin
- **WHEN** the agent reads its identity
- **THEN** the answer names the user, the device and the role `admin`
