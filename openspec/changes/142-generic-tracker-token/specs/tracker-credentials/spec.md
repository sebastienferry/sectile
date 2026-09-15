## ADDED Requirements

### Requirement: Server tracker credentials carry a tracker-agnostic name
The server SHALL accept `SECTILE_TRACKER_TOKEN` as its credential for every
supported tracker. For a given provider the server SHALL resolve the credential in
this order: the provider-specific variable (`SECTILE_GITHUB_TOKEN` for GitHub,
`SECTILE_LINEAR_API_KEY` for Linear), then `SECTILE_TRACKER_TOKEN`, then the
provider's environment-only convention fallback (`GH_TOKEN` then `GITHUB_TOKEN` for
GitHub, `LINEAR_API_KEY` for Linear). The first nonempty value SHALL win.

#### Scenario: Configure a single-tracker deployment with the generic name
- **GIVEN** a server started with only `SECTILE_TRACKER_TOKEN` exported
- **WHEN** a task on a GitHub project performs a tracker round-trip
- **THEN** the request is authenticated with the value of `SECTILE_TRACKER_TOKEN`.

#### Scenario: Keep an existing deployment working
- **GIVEN** a server started with only `SECTILE_GITHUB_TOKEN` exported
- **WHEN** a task on a GitHub project performs a tracker round-trip
- **THEN** the request is authenticated with the value of `SECTILE_GITHUB_TOKEN`.

#### Scenario: Serve two trackers without cross-provider leakage
- **GIVEN** a server started with `SECTILE_TRACKER_TOKEN` set to a GitHub credential and `SECTILE_LINEAR_API_KEY` set to a Linear credential
- **WHEN** the server addresses the Linear API
- **THEN** it uses the value of `SECTILE_LINEAR_API_KEY`
- **AND** the GitHub credential is never sent to the Linear endpoint.

### Requirement: Credential diagnostics name the tracker-agnostic variable
The server SHALL report `configure SECTILE_TRACKER_TOKEN on the server` when a tracker
request cannot proceed because no credential is configured. That message SHALL NOT name
a single tracker provider.

#### Scenario: Read a task with no credential configured
- **GIVEN** a server started with no tracker credential in its environment
- **WHEN** a client requests an operation that reaches the tracker
- **THEN** the reported error is `configure SECTILE_TRACKER_TOKEN on the server`.
