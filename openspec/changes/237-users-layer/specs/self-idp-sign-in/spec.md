## ADDED Requirements

### Requirement: One sign-in mode at a time
The server SHALL run in exactly one sign-in mode: `oidc` when an OpenID Connect
provider is configured, otherwise `local`. The current-user endpoint SHALL report the
mode, and the interface SHALL show it. The development header switch that let a caller
name its own identity SHALL no longer exist.

#### Scenario: Provider configured
- **GIVEN** a server started with a provider
- **WHEN** a person opens the interface
- **THEN** sign-in goes through the provider and the local e-mail sign-in is not offered

#### Scenario: No provider
- **GIVEN** a server started without a provider
- **WHEN** a person opens the interface
- **THEN** the local e-mail sign-in is offered and no provider redirect happens

### Requirement: Local sign-in from an e-mail address alone
In `local` mode, a person SHALL sign in by giving an e-mail address and nothing else.
An unknown address SHALL create an account; a known address SHALL sign into the
existing one, regardless of letter case or surrounding spaces. The sign-in screen SHALL
state that this mode identifies people without authenticating them and is meant for a
trusted network until a provider is connected.

#### Scenario: First sign-in creates the account
- **GIVEN** a server in local mode where the address is unknown
- **WHEN** a person submits it
- **THEN** an account exists for that address, a session is opened and the board is shown

#### Scenario: Same address, same account
- **GIVEN** an account created for an address
- **WHEN** the address is submitted again with different casing
- **THEN** the same account is signed into, with the same role

#### Scenario: Local sign-in unavailable with a provider
- **GIVEN** a server in `oidc` mode
- **WHEN** a client posts an e-mail to the local sign-in
- **THEN** it is refused as not found

### Requirement: The implicit user lasts until the first account
Without a provider, the interface SHALL keep its single implicit user, with the admin
role, only while no local account exists. Once one exists, an anonymous request on a
guarded route SHALL be refused as unauthenticated and the interface SHALL lead to the
sign-in screen.

#### Scenario: Fresh server opens the board
- **GIVEN** a server without a provider and no account
- **WHEN** a person opens the interface
- **THEN** the board opens as the implicit user, who may do everything an admin may

#### Scenario: First account closes the implicit mode
- **GIVEN** the same server
- **WHEN** the first account is created through the e-mail screen
- **THEN** that account is admin
- **AND** a later anonymous request is refused and sent to the sign-in screen

### Requirement: The interface leads to sign-in and back
When an interface call is refused as unauthenticated, the interface SHALL show the
sign-in screen instead of failing silently, and after a successful sign-in SHALL return
the person to the page they were on. The return target SHALL only ever be a path inside
the interface.

#### Scenario: Expired session on a task page
- **GIVEN** a person on a task page whose session has expired
- **WHEN** the interface next calls the server
- **THEN** the sign-in screen is shown
- **AND** after signing in the same task page is shown again

#### Scenario: Foreign redirect is ignored
- **GIVEN** a sign-in link whose return target points outside the interface
- **WHEN** the person signs in
- **THEN** they land on the board, not outside

### Requirement: The profile shows who the person is
The profile SHALL show the sign-in mode, the identity (e-mail and display name), the
role, and the projects on which the person has a registered agent. Once a person is
signed in as an account, the legacy free-text name and e-mail settings SHALL no longer
be offered.

#### Scenario: Member's profile
- **GIVEN** a signed-in member with an agent registered on two projects
- **WHEN** they open the profile
- **THEN** it shows the mode, their e-mail, the role `member` and the two projects
- **AND** it does not offer the free-text name and e-mail fields
