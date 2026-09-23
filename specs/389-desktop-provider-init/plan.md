# Implementation plan

Extract a typed initialization result and shared provider initialization routine from `internal/agent/init.go`. CLI retains argument/connection/discovery handling and formats the result. The desktop project handler already fetches fresh project configuration and guards mapped paths and active executions; add an initialize action calling the same routine with the selected provider and daemon connection.

Result contract: provider, MCP and skills objects with status (success, failed, skipped, not_run) and message, plus overall success and a human-readable summary. An attempted initialization returns the structured result even on partial failure; transport/configuration/guard errors remain HTTP errors. Fail fast after MCP errors; preserve a successful MCP result if skills fail.

Thread the provider through Electron preload and IPC. Replace the Deployment skills-only button with a labeled provider selector and Initialize button; retain SDD framework deployment. The existing desktop panel uses English. Use runtime statuses in a dedicated live result area and leave controls available for retries.

Tests: CLI regression suite; shared routine partial failure and provider isolation; real desktop HTTP handler success, invalid provider and busy guards; desktop UI selected provider, status and retry behavior. Run Go agent/config tests, Go vet/build, desktop unit/UI tests and Vite build. Update README and changelog.
