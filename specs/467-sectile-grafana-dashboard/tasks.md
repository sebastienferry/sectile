# #467: Implementation checklist

References: [`spec.md`](spec.md), [`plan.md`](plan.md).

## 1. Drift and hygiene test first (FR9, US5)

- [ ] T1.1 `internal/metrics/dashboard_test.go`: load `../../deploy/grafana/sectile.json`, walk panels, rows and `templating.list`, collect `expr` strings and `datasource` objects.
- [ ] T1.2 Checks of the plan: valid JSON; every datasource is `${datasource}` (built-in annotation source excepted); every `namespace=` matcher is `$namespace`; every `sectile_*` name (histogram suffixes stripped) is a family gathered from `New(fakeBoard{...})`; no `sum` over `sectile_active_*`.
- [ ] T1.3 Failure messages name the metric or the matcher and the panel title.

## 2. Dashboard model (FR1 to FR8, US1 to US4)

- [ ] T2.1 `deploy/grafana/sectile.json`: identity (`uid: sectile`, `id: null`, tags, time, refresh), variables `datasource`, `namespace`, `pod` as in the plan.
- [ ] T2.2 Overview row: version, replicas, active users, active runs by status, with `max` and gaps kept.
- [ ] T2.3 HTTP row: requests per second, server error ratio with the `or 0 *` term, client errors, p50/p95/p99, p95 by controller, busiest controllers; `/metrics` excluded.
- [ ] T2.4 Runtime row: goroutines, heap in use, resident memory, CPU, GC pause, file descriptors against the limit.
- [ ] T2.5 `go test ./internal/metrics/` passes; misspelling one `sectile_*` name makes it fail (checked by hand, reverted).
- [ ] T2.6 Review the file for internal identifiers: no datasource UID, cluster, node, pod IP, host name, Grafana URL or literal namespace.

## 3. Documentation (FR10, FR11)

- [ ] T3.1 `README.md`: "Grafana dashboard" subsection after "Prometheus metrics": file location, import through the interface or API, the three selectors, the expected `namespace` and `pod` labels, saving the default deployment after import.
- [ ] T3.2 `CHANGELOG.md`: one line under `[Unreleased]` > `Added`.

## 4. Verification (success criteria)

- [ ] T4.1 Run every panel query, variables substituted, against the dev Thanos datasource through the Grafana MCP (read only); list in the pull request which panels return data, without naming the datasource or namespace.
- [ ] T4.2 `go test ./...` and `make fmt-check` (or `gofmt -l`) pass.

## 5. Import into the company Grafana (FR12), after review

- [ ] T5.1 Ask the owner for approval and the target folder. Without approval, stop here and leave the step pending in the report.
- [ ] T5.2 Import the committed file (`overwrite: true`), select the dev datasource and namespace, save the variable values, and give the owner the dashboard link in chat only.

## Test plan

| Requirement | Covered by |
| --- | --- |
| FR1, FR8 | T2.1, T2.6, import in T5.2 |
| FR2, FR3 | T1.2 (datasource and namespace checks), T4.1 |
| FR4 | T2.1, T4.1 with one pod selected |
| FR5 | T1.2 (board aggregation check), T2.2 |
| FR6 | T2.3, T4.1 (no `/metrics` series in HTTP panels) |
| FR7 | T2.2 to T2.4, T4.1 |
| FR9 | T1.1 to T1.3, T2.5 |
| FR10, FR11 | T3.1, T3.2, review |
| FR12 | T5.1, T5.2 |
