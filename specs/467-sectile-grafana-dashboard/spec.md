# #467: A Grafana dashboard for the Sectile server

Ticket: https://github.com/sebastienferry/sectile/issues/467
Branch: `claude/clarify-issue-gh-11a4f59c-51d927`.
Clarification: [`docs/clarifications/467.md`](../../docs/clarifications/467.md) (confirmed in Round 2).

## Context

The server exposes Prometheus metrics at `/metrics` (ADR 0027), and the dev
deployment is already scraped, but nobody can look at them without writing
PromQL by hand: there is no dashboard. Operating Sectile still means reading
logs to know whether the server is slow, failing, or used at all.

This file states behaviour and acceptance criteria only. Implementation choices
are in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

Out of scope: new server-side metrics, alerting rules, provisioning the
dashboard from the deployment repository, and any change to how the server is
scraped.

## Terms

- **Dashboard model**: the Grafana dashboard JSON committed to this repository.
- **Deployment**: one Sectile server and its replicas, identified by the
  Kubernetes namespace its series carry (dev, a `testenv-*` board, prod later).
- **HTTP series**: `sectile_http_requests_total`, `sectile_http_errors_total`,
  `sectile_http_request_duration_seconds`. Each replica counts its own.
- **Board series**: `sectile_active_users`, `sectile_active_runs`. Every replica
  reports the same value, read from the shared database.
- **Internal identifier**: anything that names the company's infrastructure: a
  datasource UID, a cluster or node name, a pod IP, an internal host name, a
  Grafana URL, a namespace name.

## Decisions being specified

From the clarification, restated:

1. The dashboard model is committed at `deploy/grafana/sectile.json`, carries no
   internal identifier, and is imported once into the company Grafana.
2. The datasource and the deployment are chosen on the dashboard, not written
   into it; the deployment selector lists every namespace reporting Sectile
   series.
3. Only metrics the server already exposes are charted.

## User stories

### US1 (P1): An operator sees at a glance whether a deployment is healthy

As whoever operates a Sectile deployment, I want one page that shows which
build runs, how many replicas answer, who uses the board and how many runs are
in flight, so that I know in seconds whether the server is up and busy.

**Acceptance**

- **Given** the dashboard open on a deployment, **then** an overview shows the
  running version, the number of replicas reporting, the active users and the
  active runs broken down by status (`running`, `queued`, `pending`).
- **Given** a deployment with two replicas, **when** three people are active,
  **then** the dashboard shows 3 active users, not 6: the board series are never
  added up across replicas.
- **Given** a scrape where the database could not be read, **then** the active
  users and runs panels show a gap for that moment, not a drop to zero.
- **Given** a rolling deployment where two versions run at once, **then** both
  versions are visible.

### US2 (P1): An operator sees which controllers are slow or failing

As whoever operates a Sectile deployment, I want the traffic, the error rate and
the latency of each controller, so that I can tell which part of the API
misbehaves and since when.

**Acceptance**

- **Given** the dashboard open on a deployment, **then** it shows, per
  controller (`handler`), the request rate, the server error ratio (5xx over all
  requests), the client error rate (4xx), and the 50th, 95th and 99th
  percentile latencies.
- **Given** several replicas, **then** the HTTP figures are the sum of what
  every replica served.
- **Given** the scraper polling `/metrics`, **then** those scrapes do not appear
  in the HTTP panels.
- **Given** event streams and WebSockets, **then** they appear in the request
  rate but not in the latency panels (the server does not time them).
- **Given** a period without any server error, **then** the error ratio reads 0,
  not "no data", as long as requests were served.

### US3 (P2): An operator sees the process behind the figures

As whoever operates a Sectile deployment, I want the Go runtime and process
figures per replica, so that I can tell a slow controller from a starved or
leaking process.

**Acceptance**

- **Given** the dashboard open on a deployment, **then** a runtime section shows,
  per replica, goroutines, heap in use, resident memory, CPU usage, garbage
  collection pause time and open file descriptors against their limit.
- **Given** a deployment with several replicas, **when** the operator selects
  one or more replicas, **then** the runtime and HTTP panels narrow to them.

### US4 (P1): The dashboard works in any Grafana, for any deployment

As whoever imports the dashboard, I want to choose the datasource and the
deployment on the dashboard itself, so that the same file serves dev, the test
environments, prod, and anyone running Sectile elsewhere.

**Acceptance**

- **Given** a Grafana with a Prometheus-compatible datasource holding Sectile
  series, **when** the dashboard model is imported, **then** it opens without
  editing the file, and the operator picks the datasource from a selector.
- **Given** several deployments scraped into one datasource, **then** the
  deployment selector lists each of them, taken from the series themselves.
- **Given** the dashboard imported into the company Grafana, **then** it opens on
  the dev deployment by default.
- **Given** the committed dashboard model, **then** it contains no internal
  identifier.
- **Given** the dashboard imported a second time, **then** it replaces the first
  import instead of creating a copy.

### US5 (P2): The dashboard does not drift from the server

As a contributor, I want the build to fail when the dashboard charts a Sectile
metric the server no longer exposes, so that renaming a series cannot silently
empty a panel.

**Acceptance**

- **Given** a panel querying a `sectile_*` metric that the server does not
  register, **then** the Go test suite fails and names the metric.
- **Given** a panel that adds up a board series across replicas, **then** the Go
  test suite fails.
- **Given** a dashboard model that is not valid JSON, or that names a datasource
  other than the selected one, **then** the Go test suite fails.

## Functional requirements

- **FR1** The dashboard model lives at `deploy/grafana/sectile.json` and is a
  Grafana dashboard JSON importable through the Grafana interface or API.
- **FR2** Every panel queries the datasource chosen in the dashboard's
  datasource selector; none names a fixed datasource.
- **FR3** A deployment selector lists the namespaces reporting
  `sectile_build_info` in the selected datasource; every panel is restricted to
  the selected deployment.
- **FR4** A replica selector, defaulting to all replicas, narrows the HTTP and
  runtime panels.
- **FR5** HTTP series are summed across replicas; board series are taken as the
  maximum across replicas (ADR 0027).
- **FR6** The `/metrics` controller is excluded from the HTTP panels.
- **FR7** The panels of US1, US2 and US3 are present, grouped in three sections:
  Overview, HTTP, Runtime.
- **FR8** The dashboard model contains no internal identifier and has a stable
  identity, so a re-import replaces the previous one.
- **FR9** A Go test fails when the dashboard model is invalid JSON, names a
  fixed datasource, queries an unregistered `sectile_*` metric, or sums a board
  series.
- **FR10** The README explains where the dashboard model is, how to import it,
  and what the selectors do.
- **FR11** `CHANGELOG.md` gains one line under `[Unreleased]` › `Added`.
- **FR12** The dashboard is imported into the company Grafana, with the dev
  deployment saved as the default, once the owner approves that step.

## Success criteria

- Every panel returns data on the dev deployment in the company Grafana.
- `go test ./internal/metrics/` passes, and fails when a `sectile_*` name in the
  dashboard model is misspelled.

## Open requirements

None. The clarification settled every product question.
