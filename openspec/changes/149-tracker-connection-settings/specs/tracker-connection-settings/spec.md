## ADDED Requirements

### Requirement: GitHub and GitLab connection parameters are held in the user configuration
The user configuration SHALL carry the GitHub connection parameters `githubApiUrl`
and `githubToken`, and the GitLab connection parameters `gitlabUrl`,
`gitlabProject` and `gitlabToken`, alongside the existing Jira ones. These values
SHALL be settable from the interface without editing a file on the server and
without restarting it.

#### Scenario: Connect to GitHub from the interface
- **GIVEN** a server started with no GitHub variable in its environment
- **WHEN** the user saves a GitHub API URL and a token from the tracker setup screen
- **THEN** the values are persisted in the user configuration
- **AND** the next tracker request on a GitHub project authenticates with that token, with no server restart.

#### Scenario: Record a GitLab instance
- **GIVEN** a user configuration with no GitLab value
- **WHEN** the user saves a GitLab instance URL, a project slug and a token
- **THEN** the three values are persisted in the user configuration.

### Requirement: Tokens are write-only and reported through flags
The API SHALL NOT return `githubToken` or `gitlabToken` in any response. It SHALL
instead report `githubTokenSet` / `gitlabTokenSet`, true when a token is stored, and
`githubTokenFromEnv` / `gitlabTokenFromEnv`, true when no token is stored and the
server environment supplies one. An update carrying an empty token SHALL leave the
stored token unchanged; an update carrying the clear sentinel `__clear__` SHALL
delete it.

#### Scenario: Read the settings back
- **GIVEN** a stored GitHub token
- **WHEN** a client reads the settings
- **THEN** the response carries `githubTokenSet: true`
- **AND** the response carries no token value.

#### Scenario: Save an unrelated preference
- **GIVEN** a stored GitHub token
- **WHEN** a client saves the settings with an empty `githubToken`
- **THEN** the stored token is unchanged.

#### Scenario: Delete a stored token
- **GIVEN** a stored GitHub token
- **WHEN** a client saves the settings with `githubToken` set to `__clear__`
- **THEN** no GitHub token is stored
- **AND** a subsequent read reports `githubTokenSet: false`.

#### Scenario: Report a credential supplied by the environment
- **GIVEN** a server started with `SECTILE_GITHUB_TOKEN` exported and no stored GitHub token
- **WHEN** a client reads the settings
- **THEN** the response carries `githubTokenSet: false` and `githubTokenFromEnv: true`.

### Requirement: A project overrides the connection parameters it needs
A project SHALL be able to override the instance URL, the repository or project slug
and the token of its own tracker. An empty override field SHALL fall through to the
user configuration. Project tokens SHALL follow the same write-only rules as the
user configuration tokens.

#### Scenario: Drive two GitHub organisations with different tokens
- **GIVEN** a user configuration holding a GitHub token, and a project carrying its own GitHub token
- **WHEN** a tracker request runs on that project
- **THEN** it authenticates with the project token
- **AND** a request on a project with no override authenticates with the user configuration token.

#### Scenario: Leave an override empty
- **GIVEN** a project with no GitHub API URL of its own
- **WHEN** a tracker request runs on that project
- **THEN** it addresses the GitHub API URL held in the user configuration.

### Requirement: Stored configuration wins over the server environment
For each connection parameter the server SHALL resolve, in order: the project
override, then the user configuration, then the server environment variables. The
first nonempty value SHALL win. The existing environment variables SHALL keep
working when nothing is stored.

#### Scenario: A typed token supersedes an exported one
- **GIVEN** a server started with `SECTILE_GITHUB_TOKEN` exported
- **WHEN** the user saves a different GitHub token from the interface
- **AND** a tracker request runs on a GitHub project with no project override
- **THEN** the request authenticates with the saved token.

#### Scenario: Keep a headless deployment working
- **GIVEN** a server started with `SECTILE_GITHUB_TOKEN` exported and an empty user configuration
- **WHEN** a tracker request runs on a GitHub project
- **THEN** the request authenticates with the value of `SECTILE_GITHUB_TOKEN`
- **AND** the resolution falls back through `SECTILE_TRACKER_TOKEN`, `GH_TOKEN` and `GITHUB_TOKEN` as it does today.

### Requirement: The setup screen is tracker-aware and verifies before saving
The tracker setup screen SHALL present the fields of the tracker being configured
rather than the Jira fields for every tracker. For GitHub and GitLab the server
SHALL verify the submitted parameters against the instance before persisting them,
and SHALL persist nothing when the verification fails.

#### Scenario: Verify a GitHub token before saving
- **GIVEN** the setup screen with a GitHub API URL and a token entered
- **WHEN** the user asks for the check
- **THEN** the server calls the instance's `GET /user` with those parameters
- **AND** the answer names the authenticated account.

#### Scenario: Refuse a wrong credential
- **GIVEN** an invalid GitHub token entered in the setup screen
- **WHEN** the user submits the save
- **THEN** the response reports the failure returned by the instance
- **AND** the user configuration is unchanged.

#### Scenario: Show the GitLab fields
- **GIVEN** a project whose tracker is GitLab
- **WHEN** the user opens the tracker setup screen
- **THEN** it asks for an instance URL, a project slug and a token
- **AND** it does not ask for a Jira site and e-mail.

### Requirement: The agent configuration carries no credential
`~/.config/sectile/settings.json` SHALL NOT carry `githubToken`, `gitlabToken` or
any other tracker credential.

#### Scenario: Write the agent configuration with tokens stored
- **GIVEN** a user configuration holding a GitHub token and a GitLab token
- **WHEN** the agent configuration file is written
- **THEN** it contains neither token nor any other credential value.

### Requirement: Stored GitLab parameters do not register a GitLab tracker
Saving GitLab connection parameters SHALL NOT make GitLab selectable as a working
tracker. A project whose tracker is GitLab SHALL keep reporting the tracker
registry's existing "no remote tracker configured" error.

#### Scenario: Select GitLab as a project tracker
- **GIVEN** a user configuration holding valid GitLab parameters
- **WHEN** a project sets its tracker to GitLab and a synchronisation is requested
- **THEN** the request fails with the tracker registry's unconfigured-tracker error
- **AND** no partially working GitLab synchronisation is attempted.
