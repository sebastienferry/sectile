## ADDED Requirements

### Requirement: On-demand guided setup
Desktop SHALL expose the assistant in Settings > General for catalogue engines using Claude, Codex and Antigravity, disclose provider-shared configuration, and keep custom engines outside guided setup.

#### Scenario: Select supported engines
- **GIVEN** two catalogue engines share a supported provider
- **WHEN** the user opens the assistant and selects either engine
- **THEN** it shows their shared provider setup state and explains the shared effects.

#### Scenario: No supported engine
- **GIVEN** no supported provider is configured
- **WHEN** the user opens the assistant
- **THEN** it explains the prerequisite without changing custom engine configuration.

### Requirement: Actionable startup prompt
Desktop SHALL show a dismissible in-window setup toast at most once per launch when readable configuration establishes incomplete setup for a configured supported provider. It SHALL NOT automatically open the assistant or require OS notification permission.

#### Scenario: Incomplete startup
- **GIVEN** a configured supported provider has an unfinished required step or unresolved optional decision
- **WHEN** Desktop reads its setup state after startup
- **THEN** it offers a toast action to open the assistant and identifies incomplete providers.

#### Scenario: Dismiss and relaunch
- **GIVEN** the user dismissed the toast while setup remains incomplete
- **WHEN** state refreshes in the same launch and Desktop is later relaunched
- **THEN** no duplicate appears in the first launch and a new toast may appear in the next launch.

#### Scenario: Unknown or complete startup
- **GIVEN** all readable supported providers are complete or unavailable reads are the only unresolved evidence
- **WHEN** startup state is evaluated
- **THEN** no incomplete-setup toast is inferred from unavailable state, and on-demand setup remains accessible.

### Requirement: Optional skills and explicit source
The assistant SHALL allow standalone Sectile skills installation from an explicitly selected configured project or an explicit skip, preserve plugin-managed installations on skip, and explain user-wide scope.

#### Scenario: Install with chosen source
- **GIVEN** multiple projects exist
- **WHEN** the user chooses a configured source and requests skills installation
- **THEN** only the selected provider's skills are deployed from that source and the individual outcome is shown.

#### Scenario: Skip or unavailable source
- **GIVEN** the user uses a plugin or lacks a configured source folder
- **WHEN** they skip skills or attempt installation respectively
- **THEN** skip is a valid resolved decision without changing plugins, or installation is blocked with prerequisite guidance without false success.

### Requirement: Required MCP configuration
The assistant SHALL offer existing remote HTTP and local HTTP proxy choices, retain advanced STDIO access, explain credentials and agent-lifetime implications, and verify saved Sectile configuration without claiming live connectivity.

#### Scenario: Configure MCP
- **GIVEN** a supported paired provider and unrelated MCP servers
- **WHEN** the user applies a connection choice successfully
- **THEN** the saved Sectile entry is confirmed, unrelated entries and permissions remain intact, previews contain no pairing secret, and the affected engine restart is requested.

#### Scenario: Existing valid or stale entry
- **GIVEN** existing Sectile MCP configuration
- **WHEN** the assistant reads it
- **THEN** valid configuration can satisfy the step without rewriting it, while missing, invalid or stale configuration remains incomplete and Claude repair requires explicit action.

### Requirement: Explicit provider permission review
The assistant SHALL present supported provider-specific tool permission controls and require an explicit reviewed/retained decision. It SHALL preserve current decisions unless the user saves a change and SHALL NOT escalate automatically.

#### Scenario: Retain current controls
- **GIVEN** existing Claude, Codex or Antigravity permissions
- **WHEN** the user reviews and retains them
- **THEN** the current controls are acknowledged without a write and their applicable scope is explained.

#### Scenario: Unsupported control
- **GIVEN** a provider or platform lacks a permission capability, including Claude sandbox on Windows
- **WHEN** the permission step is opened
- **THEN** it explains the limitation, offers supported controls, and does not pretend an unavailable control was applied.

### Requirement: Optional project SDD
The assistant SHALL offer installation of the explicitly selected project's configured SDD framework or an explicit skip. It SHALL keep project-local SDD separate from provider-wide setup.

#### Scenario: Install or skip SDD
- **GIVEN** a selected configured project with a supported framework
- **WHEN** the user installs it or explicitly skips SDD
- **THEN** only that project's framework is deployed and its result verified, or no project content changes and the optional decision is resolved.

#### Scenario: Missing prerequisite or busy project
- **GIVEN** no configured local folder, no usable framework, or deployment blocked by active execution
- **WHEN** the user requests SDD installation
- **THEN** the step reports the prerequisite or conflict and remains retryable; explicit skip remains available.

### Requirement: Durable truthful completion
The assistant SHALL retain successful outcomes and explicit optional skips across retries/restarts, re-read relevant current configuration, and require valid MCP plus explicit current permission review and resolved skills/SDD decisions for completion.

#### Scenario: Retry partial failure
- **GIVEN** skills succeeded, MCP failed, and SDD was skipped
- **WHEN** the user retries MCP or reopens the assistant
- **THEN** skills success and SDD skip are retained, only requested actions run, and failure cannot become completion from an HTTP-success wrapper.

#### Scenario: Configuration changed externally
- **GIVEN** previously completed setup
- **WHEN** relevant MCP, permissions, installed assets, pairing identity or the selected SDD project's configuration changes
- **THEN** affected outcomes are re-evaluated and stale completion is not trusted; unrelated successful outcomes are retained.

#### Scenario: Interrupt or switch selection
- **GIVEN** a setup action is in flight
- **WHEN** the assistant closes, reconnects, or changes selection
- **THEN** accepted writes remain available for readback, pending operations are not assumed successful, and late results are never attributed to another provider or project.

### Requirement: Accessible results and bounded scope
The assistant SHALL provide keyboard-accessible actions, labelled controls, announced step outcomes, safe actionable errors, and a summary distinguishing configured, skipped and failed steps. New runtime text SHALL follow the accepted French surface requirement. Implementation SHALL add an English user-facing changelog entry under Unreleased.

#### Scenario: Finish configuration
- **GIVEN** all required outcomes are verified and optional decisions resolved
- **WHEN** the user reaches the summary
- **THEN** it identifies configured and skipped steps, reminds them to restart affected engines, and makes no claim of live skill/MCP verification.

#### Scenario: Inspect execution boundaries
- **GIVEN** any assistant action
- **WHEN** it is executed
- **THEN** it does not install AI CLIs, perform provider login, install plugins automatically, start an AI verification session, or introduce additional provider support.
