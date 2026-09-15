## MODIFIED Requirements

### Requirement: Managed registrations converge on Sectile
Normal bootstrap SHALL create or migrate to exactly one managed `sectile` registration and remove the reserved `taskflow` registration for Codex, Claude, Antigravity, Gemini, Cursor and Vibe. It SHALL write that registration to the provider's **user-level** configuration location for every supported provider. It SHALL preserve unrelated settings and registrations and explicit permission restrictions. It SHALL NOT persist bearer credentials, and SHALL NOT write MCP configuration into a repository or worktree.

#### Scenario: Fresh or legacy-only configuration
- **GIVEN** a supported provider with no managed registration or only a legacy `taskflow` registration
- **WHEN** bootstrap succeeds
- **THEN** exactly one `sectile` registration uses the current managed connection settings and no `taskflow` registration remains
- **AND** unrelated content and the effective explicit restrictions on the corresponding renamed tools are preserved.

#### Scenario: Repeat bootstrap or refresh the gateway
- **GIVEN** an already migrated configuration
- **WHEN** bootstrap runs again, including with a changed executable or gateway
- **THEN** it refreshes managed connection settings without duplicates or permission changes
- **AND** Antigravity retains its shared user-level registration without a process-specific gateway.

#### Scenario: Both registrations already exist
- **GIVEN** existing `sectile` and `taskflow` registrations
- **WHEN** bootstrap can reconcile them without discarding Sectile settings or broadening either entry's explicit permissions
- **THEN** it retains the Sectile configuration, refreshes managed connection settings and removes the redundant legacy registration.

#### Scenario: Unsafe or malformed configuration
- **GIVEN** malformed configuration, invalid managed entries, conflicting restrictions, or a policy whose effective permissions cannot safely be preserved
- **WHEN** bootstrap attempts migration
- **THEN** it returns a clear actionable error and leaves the original file unchanged.

#### Scenario: Registration written outside the checkout
- **GIVEN** any supported provider
- **WHEN** bootstrap succeeds
- **THEN** the modified configuration file lies under the user's configuration home and not under the repository or worktree.
