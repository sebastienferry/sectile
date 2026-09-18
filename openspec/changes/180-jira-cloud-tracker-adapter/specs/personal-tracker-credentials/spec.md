# personal-tracker-credentials

## Purpose
A tracker credential belongs to the person who uses it, not to the server. On a
tracker that attributes a write to the account behind the token, this is what
puts somebody's own name on what they wrote. The credential is stored so that
obtaining the database does not hand over anybody else's, and its owner may seal
it behind a passphrase only they know.

## ADDED Requirements

### Requirement: A tracker credential belongs to one person
A person SHALL be able to store, replace and forget their own credential for one
tracker, from their own profile, without an administrator. The credential SHALL
carry the site URL, the account e-mail and the token together, because an
account belongs to an instance. A tracker that attributes its writes to an
account SHALL offer no server-wide credential in the interface.

#### Scenario: Store a personal credential
- **GIVEN** a signed-in person with no credential for Jira
- **WHEN** they save a site, an e-mail and a token from their profile
- **THEN** the three are stored against their identity
- **AND** no other person's view of their own credentials changes.

#### Scenario: A project inherits the instance of whoever creates it
- **GIVEN** a person whose Jira credential names `acme.atlassian.net`
- **WHEN** they select Jira on a project with no tracker URL
- **THEN** the project's tracker URL is prefilled with that instance
- **AND** a URL already typed is left untouched.

### Requirement: The API never returns a token
No response SHALL carry a stored token. A listing SHALL report the tracker, the
site, the e-mail, whether the credential is sealed and whether it is unlocked.
The routes SHALL act on the caller's own credentials only, taking the identity
from the session and never from the payload, and SHALL require a session.

#### Scenario: Read back what is stored
- **GIVEN** a stored Jira token
- **WHEN** the person lists their credentials
- **THEN** the answer carries the site, the e-mail and the sealed state
- **AND** carries no token.

#### Scenario: Refuse an anonymous caller
- **GIVEN** a deployment with an identity provider configured
- **WHEN** an unauthenticated call reaches the personal credential routes
- **THEN** it is refused, and the routes do not bypass the session guard.

### Requirement: A stored credential is encrypted and bound to its owner
A token SHALL be stored encrypted with an authenticated cipher, with the owner's
identity and the tracker name as additional authenticated data, and with the key
held outside the database. Moving a stored record from one owner to another, or
from one tracker to another, SHALL make it fail to open rather than serve the
thief.

#### Scenario: A row moved between users opens for nobody
- **GIVEN** two people each holding a Jira credential
- **WHEN** one person's encrypted record is written into the other's row directly in the database
- **THEN** opening it fails
- **AND** the original owner's credential keeps working.

#### Scenario: A copy of the database alone is useless
- **GIVEN** a stored credential encrypted with the server key
- **WHEN** the database is opened with a different key
- **THEN** no token can be read from it.

### Requirement: An owner may seal their credential
A person SHALL be able to seal their credential with a passphrase, which SHALL
NOT be stored in any form. A sealed credential SHALL be openable only while its
owner has supplied that passphrase in the running server, and SHALL require it
again after a restart. The interface SHALL state that consequence at the moment
of the choice. Replacing the credential SHALL retire the key it was sealed with.

#### Scenario: Unseal for a session
- **GIVEN** a sealed credential and a restarted server
- **WHEN** an operation needs it before the passphrase is supplied
- **THEN** it fails saying the credential is sealed
- **AND** it succeeds once the owner has supplied the passphrase.

#### Scenario: A wrong passphrase teaches nothing
- **GIVEN** a sealed credential
- **WHEN** a wrong passphrase is supplied
- **THEN** it is refused with the same answer a missing credential gives
- **AND** the credential stays locked.

### Requirement: An operation carries the credential of whoever asked for it
An operation a person asked for SHALL use their own credential, whether it runs
in their request or in the background queue that outlives it. On a tracker whose
credential is personal, an operation naming a person who has none SHALL be
refused rather than run under the server credential. Work nobody asked for, such
as a timer, SHALL name nobody and use the server credential.

#### Scenario: A queued write carries its author
- **GIVEN** a person with their own Jira token
- **WHEN** they advance a task, edit it, or post a comment
- **THEN** the write reaching Jira authenticates with their token
- **AND** the work item shows them as the author.

#### Scenario: Refuse rather than write under another name
- **GIVEN** a person with no Jira token of their own
- **WHEN** they ask for an operation on a Jira project
- **THEN** it is refused, naming the missing personal token
- **AND** nothing is written under the server account.

### Requirement: The server serves without a key
The absence of the encryption key SHALL NOT prevent the server from starting.
Storing or opening an unsealed personal credential SHALL then be refused, naming
the environment variable that supplies the key, while a sealed credential SHALL
keep working since it derives its own key. Nothing SHALL be encrypted under a
default or empty key.

#### Scenario: A read-only data directory
- **GIVEN** a server whose data directory cannot be written and no key in the environment
- **WHEN** it starts
- **THEN** it serves, and reports that the key is unavailable
- **AND** storing an unsealed personal credential is refused with the variable named.
