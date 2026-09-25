# #467: Implementation plan

References: [`spec.md`](spec.md), [`tasks.md`](tasks.md),
[ADR 0027](../../docs/adrs/0027-prometheus-metrics-and-active-users.md).

## Architecture

No server code changes. The deliverable is a static Grafana dashboard model, a
Go test that keeps it aligned with the metrics the server registers, and
documentation.

```
deploy/grafana/sectile.json          dashboard model (new)
internal/metrics/dashboard_test.go   drift and hygiene checks (new)
README.md                            "Grafana dashboard" subsection (edit)
CHANGELOG.md                         one line under [Unreleased] > Added (edit)
```

The test lives in `internal/metrics` because that package owns the registry: it
builds `New(...)` with a fake board, gathers the registered families and
compares them with the names the dashboard queries. It runs in `test:go` with
no new CI job.

## Target files

| File | Change |
| --- | --- |
| `deploy/grafana/sectile.json` | New. Grafana dashboard JSON, `schemaVersion` 39. |
| `internal/metrics/dashboard_test.go` | New. Reads `../../deploy/grafana/sectile.json`. |
| `README.md` | New subsection after "Prometheus metrics". |
| `CHANGELOG.md` | One `Added` line. |

## Dashboard model

### Identity

- `uid`: `sectile`; `title`: `Sectile`; `tags`: `["sectile"]`; top-level `id`:
  `null`. A fixed `uid` makes a re-import overwrite the previous one (FR8).
- `time`: last 6 hours; `refresh`: `30s`; `graphTooltip`: shared crosshair.
- No `__inputs` / `__requires` block: the datasource is a template variable, so
  the file imports as is through the interface or `POST /api/dashboards/db`.

### Variables (FR2, FR3, FR4)

| Name | Type | Definition | Notes |
| --- | --- | --- | --- |
| `datasource` | datasource | `query: prometheus` | Every panel and variable uses `{"type":"prometheus","uid":"${datasource}"}`. |
| `namespace` | query | `label_values(sectile_build_info, namespace)` | Single value, refresh on time range change, sorted alphabetically. `current` left empty in the file. |
| `pod` | query | `label_values(sectile_build_info{namespace="$namespace"}, pod)` | Multi-value, `includeAll`, `allValue: ".*"`. |

Selector used below: `S = namespace="$namespace", pod=~"$pod"`.

The default deployment (US4, "opens on dev") is **not** written into the file:
the namespace name is an internal identifier. It is set in the imported copy,
by selecting the dev namespace and saving the dashboard with "save current
variable values" (FR12). Anyone else importing the file lands on the first
namespace in the list.

### Overview row (US1)

| Panel | Type | Query |
| --- | --- | --- |
| Version | stat, text mode, one value per series | `count by (version) (sectile_build_info{namespace="$namespace"})`, legend `{{version}}` |
| Replicas | stat | `count(sectile_build_info{namespace="$namespace"})` |
| Active users | stat + sparkline | `max(sectile_active_users{namespace="$namespace"})` |
| Active runs | time series, stacked | `max by (status) (sectile_active_runs{namespace="$namespace"})`, legend `{{status}}` |

Board panels use `max`, never `sum` (FR5). Null values are left as gaps
(`spanNulls: false`), so a failed database read stays visible.

### HTTP row (US2)

`H = S, handler!="/metrics"` (FR6). Rates use `$__rate_interval`.

| Panel | Type | Query |
| --- | --- | --- |
| Requests per second by controller | time series | `sum by (handler) (rate(sectile_http_requests_total{H}[$__rate_interval]))` |
| Server error ratio by controller | time series, percent unit | `(sum by (handler) (rate(sectile_http_errors_total{H, class="server"}[$__rate_interval])) or 0 * sum by (handler) (rate(sectile_http_requests_total{H}[$__rate_interval]))) / sum by (handler) (rate(sectile_http_requests_total{H}[$__rate_interval]))` |
| Client errors per second by controller | time series | `sum by (handler) (rate(sectile_http_errors_total{H, class="client"}[$__rate_interval]))` |
| Latency p50 / p95 / p99 | time series, seconds, one query per quantile | `histogram_quantile(Q, sum by (le) (rate(sectile_http_request_duration_seconds_bucket{H}[$__rate_interval])))` |
| p95 latency by controller | time series, seconds | `histogram_quantile(0.95, sum by (handler, le) (rate(sectile_http_request_duration_seconds_bucket{H}[$__rate_interval])))` |
| Busiest controllers | table, instant | `topk(10, sum by (handler, method) (increase(sectile_http_requests_total{H}[$__range])))` |

The `or 0 * ...` term turns "no error series yet" into a 0 ratio wherever
requests exist (US2, last criterion). Streams are absent from the latency
panels by construction: the server does not observe them.

### Runtime row (US3)

Per pod, legend `{{pod}}`, selector `S`.

| Panel | Query |
| --- | --- |
| Goroutines | `go_goroutines{S}` |
| Heap in use | `go_memstats_heap_inuse_bytes{S}` |
| Resident memory | `process_resident_memory_bytes{S}` |
| CPU (cores) | `rate(process_cpu_seconds_total{S}[$__rate_interval])` |
| GC pause time per second | `rate(go_gc_duration_seconds_sum{S}[$__rate_interval])` |
| Open file descriptors | `process_open_fds{S}` and `process_max_fds{S}` (dashed) |

`go_*` and `process_*` come from the collectors `metrics.New` registers.

## Drift and hygiene test (FR9)

`internal/metrics/dashboard_test.go`, package `metrics`:

1. Read and `json.Unmarshal` the model into `map[string]any`; fail on error.
2. Walk the whole tree (rows nest `panels`), collecting every `expr` string and
   every `datasource` object, including those of `templating.list`.
3. **Datasource**: every `datasource` object has `uid == "${datasource}"`,
   except the built-in annotation source (`uid == "-- Grafana --"`).
4. **Namespace**: every `namespace=` matcher in an `expr` or variable query is
   `namespace="$namespace"`; a literal namespace fails.
5. **Metric names**: extract `\bsectile_[a-z0-9_]+\b` from each `expr`; strip a
   `_bucket`, `_sum` or `_count` suffix when the base is a registered histogram;
   each name must be a family returned by `New(fakeBoard{users: 1, runs:
   map[string]int{"running": 1}}, time.Minute, Build{}).registry.Gather()`.
   The failure names the metric and the panel title.
6. **Board aggregation**: an `expr` containing `sectile_active_users` or
   `sectile_active_runs` must not contain `sum(` or `sum by`.

`go_*` and `process_*` names are not checked: several process metrics are not
produced on macOS, where contributors run the suite, and the collectors are
upstream code, not ours.

## Verification before review

Read-only, not committed: substitute the variables and run each panel query
against the dev Thanos datasource of the company Grafana (Grafana MCP
`query_prometheus`), and record in the pull request which panels returned data.
No datasource UID or namespace goes into the pull request text either.

## Import (FR12)

Outward-facing, so gated on the owner's explicit approval at the time. After
review: import the committed file into the company Grafana (interface, or the
Grafana MCP `update_dashboard` with `overwrite: true`), in the folder the owner
names, select the dev datasource and namespace, and save the variable values.
The saved defaults exist only in that Grafana, never in the repository.

## Decisions

- **Datasource as a template variable rather than `__inputs`.** `__inputs` only
  works through the interactive import screen; a variable works through both the
  interface and the API, and lets one dashboard switch between Thanos and a
  per-cluster Prometheus.
- **Namespace as the deployment key.** It is present on every series the
  platform scrapes and needs no server change; `job` and `service` depend on the
  scrape configuration.
- **Test in `internal/metrics`.** The registry is the source of truth for the
  names; a test elsewhere would have to duplicate the list.

## Rejected alternatives

- **Hard-coding the dev namespace as the default.** It would publish an
  internal identifier in a public repository.
- **Provisioning through the deployment repository (ConfigMap and Grafana
  sidecar).** Settled out of scope in the clarification.
- **A dashboard generator (Grafonnet, Jsonnet, Go SDK).** A new toolchain for
  one dashboard of about twenty panels; the JSON stays reviewable as is.
- **Checking `go_*` and `process_*` names.** Platform-dependent, see above.

## Risks

- Grafana versions older than 10 may render some panel options differently;
  the model uses no feature newer than `schemaVersion` 39.
- A scrape configuration that drops the `namespace` or `pod` label would empty
  the selectors; the README says both labels are expected.
