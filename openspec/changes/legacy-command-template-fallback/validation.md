# Validation

- Root cause reproduced against the running agent: `GET /desktop/project?id=…` answered 502 with `Sectile API returned HTTP 400: aiCommandTemplate must contain {prompt}` for both projects; the server database holds `agy` (global) and `claude` (project) as bare command templates.
- `go test ./internal/agentconfig`: passed, including the new `TestEffectiveCommandTemplate` table (bare names for named and legacy-default providers, `{prompt}` templates kept, `custom` unchanged).
- `go test ./internal/db`: passed, including `TestAgentConfigLegacyBareCommandTemplate` with the exact stored values (`agy`/`agy` inherited, `claude`/`claude` on the project, `{prompt}` template preserved, `custom` still rejected).
- `go test ./internal/handlers`: passed, including `TestAgentConfigServesLegacyBareTemplateAsEmpty` (200 with an empty template through `HandleAgentConfig`).
- `go test ./...`: 10 packages passed. `go vet ./...`: passed. `gofmt -l` on touched files: clean. `git diff --check`: passed.
- `npm test` in desktop: passed (the `api()` diagnostics change names method, route and status when a response body is empty).
- End to end: the fixed server built to a temporary directory, started on port 18090 with its own token against a snapshot copy of the live database, answered `200` with `aiProvider: claude, aiCommandTemplate: ""` for the affected project and `aiProvider: agy, aiCommandTemplate: ""` for the default project. The instance was stopped and the snapshot deleted. The credential-scrubbing step on the snapshot failed on a NOT NULL constraint and did not apply; the instance lived about two seconds and its log shows no synchronization activity.

Handler and agent suites bind loopback listeners and were run outside the restricted sandbox with a temporary Go build cache. The running server, the running agent and the installed desktop application were not replaced; the live database was only read.
