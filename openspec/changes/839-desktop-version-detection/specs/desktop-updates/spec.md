## ADDED Requirements

### Requirement: Stable Desktop release discovery
Sectile Desktop SHALL check published stable tagged GitHub releases for Sectile Desktop, compare numeric SemVer precedence against the installed Desktop version, and offer only a strictly newer release with an archive matching the running OS and architecture. Drafts, prereleases, invalid version tags, and rolling main builds SHALL be excluded. Checks SHALL require no tracker credentials. Unknown installed versions SHALL be reported as uncomparable rather than treated as zero.

#### Scenario: Numeric version comparison
- **GIVEN** installed Desktop version 0.9.0 and a stable release 0.10.0 with a matching archive
- **WHEN** a check succeeds
- **THEN** 0.10.0 is offered as an update
- **AND** releases equal to or older than the installed version are never offered

#### Scenario: Excluded or incomplete releases
- **GIVEN** metadata includes a draft, prerelease, invalid tag, or newer release missing this platform's archive
- **WHEN** a check succeeds
- **THEN** none of those releases is offered as an installable update
- **AND** if a newer stable version has no matching archive, settings explain that no download is available for this platform rather than saying the application is up to date

#### Scenario: Installed version cannot be compared
- **GIVEN** the installed Desktop version is unavailable or invalid
- **WHEN** release metadata is checked
- **THEN** settings explain that comparison is unavailable
- **AND** no downgrade or update claim is made

### Requirement: Automatic and manual checks
Desktop SHALL check at launch and every 24 hours while it remains open, with a manual Check for updates control in Changelog settings. Checks SHALL be bounded and coalesced so concurrent triggers do not start overlapping network requests. Checks SHALL not depend on the local agent running or on pairing with a server.

#### Scenario: Startup and daily schedule
- **GIVEN** Desktop launches and remains open
- **WHEN** startup completes and subsequently 24 hours elapse
- **THEN** a background check is initiated at each point without blocking normal use
- **AND** resume after a missed interval initiates one due check without replaying missed intervals

#### Scenario: Manual check while checking
- **GIVEN** a check is in progress
- **WHEN** the owner activates Check for updates or another scheduled trigger occurs
- **THEN** the current check is shared and its progress is visible
- **AND** at most one network check remains active

### Requirement: Visible and actionable availability
Desktop SHALL expose a persistent availability indicator outside the Changelog panel while a newer compatible release is known, and Changelog settings SHALL show installed version, available version, last check outcome, and an explicit release/download action. Opening that action SHALL navigate through the normal external browser. No archive SHALL be fetched or installed by Desktop. Touched runtime wording SHALL preserve its existing surface language.

#### Scenario: Available release
- **GIVEN** a check finds a newer compatible stable release
- **WHEN** the owner closes and reopens settings or restarts Desktop
- **THEN** availability remains visible until a successful check supersedes it or the installed version catches up
- **AND** settings offer the matching archive and a release page link
- **AND** activating a link does not restart the agent or install anything

### Requirement: Notification deduplication
Desktop SHALL attempt at most one availability notification for each detected newer compatible version on this workstation, including across renderer reloads and application restarts. Denied or unavailable notifications SHALL not prevent the persistent indicator or normal use.

#### Scenario: Repeated detection
- **GIVEN** version 0.10.0 already caused a notification attempt
- **WHEN** subsequent automatic/manual checks or an application restart detect 0.10.0 again
- **THEN** no further notification is attempted for 0.10.0
- **AND** a subsequently detected 0.11.0 may cause one new attempt

### Requirement: Failure and offline behavior
Desktop SHALL distinguish checking, successful checks with no newer compatible release, available updates, comparison/platform limitations, and failed or throttled checks. An unsuccessful check SHALL retain previously known availability with its last successful check time and indicate that it was not refreshed. Installed versions and embedded release notes SHALL remain usable offline. Published Desktop availability SHALL remain independent of bundled-agent mismatch and restart controls.

#### Scenario: Offline or throttled check
- **GIVEN** Desktop has cached availability and the network is offline or GitHub throttles requests
- **WHEN** a check fails
- **THEN** settings show the unsuccessful outcome and retained availability as last known information
- **AND** installed version and embedded notes remain readable
- **AND** normal tasks and consoles remain usable

#### Scenario: Successful no-update result
- **GIVEN** the newest compatible stable release is equal to or older than the installed version
- **WHEN** a check succeeds
- **THEN** settings show a successful no-update outcome and its check time
- **AND** any superseded availability indicator is cleared

#### Scenario: Agent mismatch remains independent
- **GIVEN** the running agent differs from the agent bundled with Desktop
- **WHEN** a Desktop release check completes
- **THEN** the existing mismatch indicator and restart behavior remain available independently

### Requirement: User-facing changelog
Implementation SHALL add an English entry under Added in CHANGELOG.md's Unreleased section describing Desktop update detection and release/download navigation.

#### Scenario: Implementation is delivered
- **GIVEN** the feature is implemented
- **WHEN** the Unreleased changelog is read
- **THEN** it describes the user-visible update detection and link behavior without claiming automatic installation
