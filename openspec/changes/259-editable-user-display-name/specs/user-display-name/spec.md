## ADDED Requirements

### Requirement: A person chooses the name the board shows for them
A signed-in person SHALL be able to set and change the display name of their own account
from the account section of their profile. That name SHALL be shown wherever the account is
named: the account section, the admin users panel, and the owner of an execution in the
activity log. The change SHALL be visible without reloading the interface.

#### Scenario: Replace the e-mail address with a name
- **GIVEN** an account created by the local e-mail sign-in, whose name is its address
- **WHEN** its owner types `Sébastien Ferry` in the display name field and saves
- **THEN** the account section says they are signed in as `Sébastien Ferry`
- **AND** a following `GET /api/me` carries `Sébastien Ferry`.

#### Scenario: The admin users panel shows the chosen name
- **GIVEN** a colleague who chose a display name
- **WHEN** an admin opens the users panel
- **THEN** the colleague's row shows that name, with their e-mail underneath
- **AND** the role control behaves exactly as before.

### Requirement: A chosen name survives the next sign-in
The server SHALL preserve a chosen display name across sign-ins. A name supplied by the
local sign-in or by the identity provider SHALL NOT replace a name its owner chose.

#### Scenario: Sign in again on a local deployment
- **GIVEN** a person who chose a display name and signed out
- **WHEN** they sign in again with the local e-mail sign-in
- **THEN** their chosen name is still shown, not their e-mail address.

#### Scenario: Sign in again behind an identity provider
- **GIVEN** a person who chose a display name on a deployment with an identity provider
- **WHEN** they sign in and the provider sends its own name claim
- **THEN** their chosen name stands
- **AND** their e-mail keeps following the provider.

### Requirement: Clearing the name falls back rather than blanking the board
The server SHALL accept an empty display name and treat it as "no choice". An account with
no chosen name SHALL be named by the value the sign-in supplied, and failing that by the
existing `displayName || email || id` fallback, so no row ever shows an empty owner.

#### Scenario: Clear a chosen name
- **GIVEN** a person whose account carries a chosen name
- **WHEN** they clear the field and save
- **THEN** the call succeeds
- **AND** everywhere they are named the board shows their e-mail address.

### Requirement: A display name is validated as a name
The server SHALL trim the surrounding whitespace of a display name, SHALL refuse a value of
more than 80 characters with `400` and the message `A display name is at most 80 characters`,
and SHALL refuse a value containing a line break or a control character with `400` and the
message `A display name cannot contain line breaks`. The length SHALL be counted in runes,
so an accented name is not shorter than an unaccented one. A refused value SHALL leave the
stored name unchanged. A display name SHALL NOT be unique: the account is identified by its
e-mail, never by its name.

#### Scenario: Refuse an over-long name
- **GIVEN** a signed-in person
- **WHEN** they save a display name of 81 characters
- **THEN** the call answers `400` with `A display name is at most 80 characters`
- **AND** the stored name is unchanged.

#### Scenario: Refuse a line break
- **GIVEN** a signed-in person
- **WHEN** they save a display name containing a line break
- **THEN** the call answers `400` with `A display name cannot contain line breaks`.

#### Scenario: Accept a name a colleague already uses
- **GIVEN** an account already named `Sébastien Ferry`
- **WHEN** another person saves the same display name
- **THEN** the call succeeds.

### Requirement: A person renames only their own account
The server SHALL apply a rename to the calling account only, and SHALL expose no route that
renames another account. An unauthenticated rename SHALL be refused with `401` and the
message `Sign in to use this interface`, as every interface route is since ADR 0015.

#### Scenario: Rename without a session
- **GIVEN** no session cookie and no workstation API key
- **WHEN** the rename call is made
- **THEN** it answers `401` with `Sign in to use this interface`.

#### Scenario: An admin has no route to rename someone else
- **GIVEN** a signed-in admin
- **WHEN** they look for a way to rename another account through the API
- **THEN** no route accepts it, and the users panel offers only the role control.
