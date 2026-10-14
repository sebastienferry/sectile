## ADDED Requirements

### Requirement: Respect Concurrent Pull-Request Link Detachment During Discovery
`applyDiscoveredPullRequests` SHALL re-read `pr_links_detached` inside its transaction on the locked task row and SHALL skip writing discovered pull requests when `pr_links_detached` is non-zero and the discovery is not forced.

#### Scenario: User detaches links while automatic discovery is in flight
- **GIVEN** a task eligible for automatic PR discovery (`force = false`)
- **WHEN** a user detaches the task's pull request links after the discovery gate passes and before `applyDiscoveredPullRequests` locks the task row
- **THEN** `applyDiscoveredPullRequests` writes nothing, attaches no links, and leaves `pr_links_detached` set

#### Scenario: Forced discovery still attaches links even when detached during discovery
- **GIVEN** a task on which forced PR discovery (`force = true` / `discoverAndApply`) is running
- **WHEN** the task's pull request links are detached while the discovery is in flight
- **THEN** `applyDiscoveredPullRequests` attaches the discovered links and resets `pr_links_detached` to 0
