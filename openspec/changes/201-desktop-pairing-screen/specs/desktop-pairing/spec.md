## ADDED Requirements

### Requirement: The desktop connect screen accepts a pairing code
The desktop application SHALL offer, on its connect screen, a field for the pairing code generated
by the web interface, and SHALL make that field the primary way of connecting. It SHALL NOT require
an API key to connect. It SHALL keep an API key reachable as a secondary option, presented as such.
The screen SHALL tell the user where the code is generated and that a code is single use.

#### Scenario: Connecting with a pairing code
- **GIVEN** a desktop application that holds no credential
- **AND** a valid, unspent pairing code generated in the web interface
- **WHEN** the user enters the server address and the code and confirms
- **THEN** the application exchanges the code with the server for a device credential
- **AND** it uses that credential, never the code, to authenticate its subsequent calls
- **AND** the connect screen gives way to the workspace

#### Scenario: Connecting with an API key instead
- **GIVEN** a user who already holds an API key from their web profile
- **WHEN** they open the secondary credential option, enter the key and confirm
- **THEN** the application connects with that key and exchanges nothing

#### Scenario: Neither credential is given
- **WHEN** the user confirms with an empty pairing code and an empty API key
- **THEN** the application refuses to connect
- **AND** the message names both ways in

### Requirement: The pairing code wins over an API key
When the connect form carries both a pairing code and an API key, the application SHALL exchange the
pairing code and SHALL ignore the API key.

#### Scenario: A code typed while an older key is still present
- **GIVEN** a connect form whose secondary field still holds a previously used API key
- **WHEN** the user pastes a fresh pairing code and confirms
- **THEN** the application exchanges the code
- **AND** the credential it keeps is the one the exchange returned, not the older key

### Requirement: A single-use code is not spent on an unusable server address
The application SHALL establish that the server address is usable — an HTTP or HTTPS URL carrying no
credentials — before it sends the pairing code to that address.

#### Scenario: A malformed server address
- **GIVEN** a connect form carrying a pairing code and a server address that is not an HTTP or HTTPS
  URL, or that embeds a user name or password
- **WHEN** the user confirms
- **THEN** the application refuses before any pairing request leaves the machine
- **AND** the code remains unspent and usable on a corrected address

### Requirement: The obtained credential is always persisted
Having spent a pairing code, the application SHALL persist the credential it received, so that a
later launch connects without a new code. It SHALL encrypt the credential with the operating
system's secure store when one is available, and SHALL otherwise write it unencrypted in the same
settings file. Either way the file SHALL be readable and writable by its owner only, in a directory
closed to others. The application SHALL persist the device identifier the exchange returned
alongside the credential.

#### Scenario: A host with a secure store
- **GIVEN** a machine whose operating system offers an encryption store
- **WHEN** a pairing code is exchanged successfully
- **THEN** the credential is written encrypted
- **AND** no clear copy of it remains in the settings file

#### Scenario: A host without a secure store
- **GIVEN** a machine offering no encryption store
- **WHEN** a pairing code is exchanged successfully
- **THEN** the credential is still written, unencrypted, with owner-only permissions
- **AND** a later launch connects with it and does not ask for a pairing code

#### Scenario: Reading back a persisted credential
- **GIVEN** a settings file written by either of the two paths above
- **WHEN** the application reads its settings to populate the connect screen
- **THEN** it returns the credential in usable form
- **AND** it does not expose its stored encrypted representation

### Requirement: Pairing failures are reported in the user's terms
The application SHALL report a failed pairing with a message stating what happened and what to do
next, and SHALL distinguish an unreachable server, a rejected code and a server that is not a
Sectile server.

#### Scenario: An expired or already used code
- **GIVEN** a pairing code that has expired or has already been exchanged
- **WHEN** the user confirms
- **THEN** the application reports that the code is invalid or expired and that a new one is needed
- **AND** the connect screen stays open with the server address preserved

#### Scenario: An unreachable server
- **GIVEN** a server address that answers nothing
- **WHEN** the user confirms with a pairing code
- **THEN** the application reports that the server could not be reached and points at the address

#### Scenario: An address that is not a Sectile server
- **GIVEN** an address that answers, but not with a device credential
- **WHEN** the user confirms with a pairing code
- **THEN** the application reports that this URL does not expose the Sectile agent API
