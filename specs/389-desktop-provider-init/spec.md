# Desktop provider initialization

## User story (P1)
As a desktop user, I select my native AI provider and initialize its server-managed skills and MCP connection without using a terminal.

## Requirements and acceptance scenarios
1. Given a mapped project, when Deployment opens, a labeled provider selector and Initialize button are available. Initialization requires an explicit click and targets exactly the selected provider.
2. Given a paired server serving current skills, when initialization succeeds, the selected provider receives its MCP configuration and managed skills. Both outcomes are visible. Other providers are not initialized.
3. Given a provider without a supported skills convention, when initialization succeeds, MCP succeeds and skills are explicitly skipped.
4. Given MCP failure, when initialization completes, MCP failure and skills not run are visible. Given skills failure after MCP success, both distinct outcomes remain visible.
5. Given any completed attempt, when the user retries, initialization runs again using fresh server configuration. Controls prevent duplicate submissions while running.
6. Given invalid provider input, an unmapped checkout, or active executions, initialization is refused without changing provider files.
7. CLI flag and positional provider selection, connection resolution, and project discovery remain compatible.

## Out of scope
Server web UI, automatic initialization on selection or startup, new provider conventions, execution provider preference changes, release or merge.
