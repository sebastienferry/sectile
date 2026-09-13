## ADDED Requirements

### Requirement: Current product presentation uses Sectile
The application SHALL identify itself as Sectile in current UI branding, browser title, CLI usage and startup messages, API landing-page presentation, terminal banners, and generated default workflow prose. Existing visual design, icons, layout, and functional behavior SHALL remain unchanged.

#### Scenario: Open the application
- **GIVEN** a supported UI locale and a fresh or upgraded installation
- **WHEN** the user opens the application and views normal product-identifying screens
- **THEN** the product name is displayed as Sectile in the browser title and visible headers
- **AND** navigation, icons, colors, and layout retain their existing behavior.

#### Scenario: Use server and command-line entry points
- **GIVEN** the Sectile executable binary
- **WHEN** the user starts the server, requests CLI help or usage, or views the API landing page
- **THEN** human-readable product identification and current command examples use Sectile and `sectile`
- **AND** existing command flags, arguments, and outcomes remain supported.

### Requirement: Distribution uses the selected technical name
Builds SHALL produce `bin/sectile` and release artifacts named `dist/sectile-<os>-<arch>` with `.exe` for Windows. The existing five platform targets (`darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`, `windows/amd64`) and embedded frontend assets SHALL remain supported. Frontend package metadata SHALL identify the package as `sectile-web`.

#### Scenario: Build and run locally
- **GIVEN** the documented build prerequisites
- **WHEN** the user executes `make build` and `make run`
- **THEN** the generated Sectile executable (`bin/sectile`) runs and serves its embedded frontend without requiring separate frontend files.

#### Scenario: Generate release artifacts
- **GIVEN** a release build invocation via `make release`
- **WHEN** release generation completes
- **THEN** Sectile artifacts exist for darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, and windows/amd64 (with `.exe`)
- **AND** current documentation refers to these exact artifact names and the real source repository coordinates.

### Requirement: Existing installations retain their state
The rename SHALL preserve existing database selection precedence, environment configuration loading, credentials, browser preferences, project filters, workflow settings, and supported Taskacao fallbacks. It SHALL NOT automatically relocate, merge, delete, or reset existing state or introduce competing storage namespaces.

#### Scenario: Database selection on upgrade
- **GIVEN** databases with distinguishable tasks at multiple supported locations
- **WHEN** the renamed application starts
- **THEN** an explicit `DB_PATH` wins, followed by an existing working-directory database, the TaskFlow user database, then the Taskacao user database
- **AND** non-selected databases are untouched.

#### Scenario: Fresh installation or unavailable data directory
- **GIVEN** no explicit database path and no existing supported database
- **WHEN** Sectile starts
- **THEN** it uses the existing TaskFlow data-directory default or the working-directory fallback when that directory is unavailable
- **AND** no separate Sectile data store is created.

#### Scenario: Credentials and preferences survive
- **GIVEN** an installation with saved credentials, custom settings, project-specific filters, and browser preferences
- **WHEN** the user upgrades and reloads the application
- **THEN** the same settings and credentials remain effective under existing precedence rules
- **AND** preferences already supported through Taskacao fallback remain available
- **AND** new preference changes remain visible after another reload.

### Requirement: Automation and generated context remain compatible
Existing environment variables, machine-facing health checks, terminal protocols, task result contracts, and project-context ownership markers SHALL remain supported without a namespace migration. Generated product prose SHALL use Sectile while preserving user-owned configuration, instructions, and script content.

#### Scenario: Regenerate project instructions
- **GIVEN** project configuration with unknown keys, an existing owned workflow block, and user-authored instructions outside it
- **WHEN** project instructions are generated twice
- **THEN** one owned workflow block identifies the product as Sectile
- **AND** unknown keys and user instructions outside the owned block are preserved
- **AND** existing configuration paths (`.taskflow/`) and repository references remain valid.

#### Scenario: Existing custom automation
- **GIVEN** a custom command consuming existing task/project environment variables and a supported terminal run
- **WHEN** Sectile launches and observes the command
- **THEN** the existing variables carry the same values
- **AND** output and completion status are recognized under the existing protocol (`__TASKFLOW_*`)
- **AND** stored custom scripts are not rewritten.

#### Scenario: Existing service detection
- **GIVEN** a supported TaskFlow or Taskacao service already occupying the configured port
- **WHEN** the Sectile executable starts
- **THEN** it recognizes the service as before via health check `taskflow-api`
- **AND** an unrelated service on the port is rejected under existing detection rules.

### Requirement: Documentation explains the rename and retained identifiers
Current installation and technical documentation SHALL use Sectile and explain intentional legacy compatibility identifiers and how to update hard-coded executable launchers. Real remote URLs, historical reports, and user-owned content SHALL not be rewritten merely to remove the previous name.

#### Scenario: Follow upgrade instructions
- **GIVEN** a user with an existing TaskFlow installation or a launcher referencing its executable
- **WHEN** the user follows the Sectile upgrade instructions
- **THEN** the instructions identify the new executable and release names, preserved data/configuration locations, and environment names
- **AND** explain updating the launcher or supplying a local compatibility alias
- **AND** source links still target `sebastienferry/taskflow` until an actual external repository rename occurs.
