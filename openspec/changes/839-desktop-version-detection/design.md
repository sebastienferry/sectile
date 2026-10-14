## Context

Accepted scope: clarification Round 2. Electron exposes installed Desktop version through `app.getVersion()` in `desktop/electron/main.cjs`; `desktop/electron/preload.cjs` mediates renderer access. `fillChangelogPanel` in `desktop/src/main.js` renders installed versions and embedded notes. Its inspected wording is English. Existing notifications in `desktop/src/notifications.mjs` concern task transitions and must not acquire release-specific run states.

## Goals / Non-Goals

Implement detection independent of agent/server connectivity, with durable availability and deduplication. Do not implement automatic installation, rolling channels, independent agent/server checks, or change packaging/version sources.

## Decisions

1. Add a main-process `desktop/electron/desktop-updates.cjs` service with injectable fetch, clock, platform/architecture, installed version, and storage. Use Node/Electron networking, not a shell or child process. Run once after app readiness, then schedule against a 24-hour deadline; check the deadline after resume, clear timers at quit, and coalesce manual triggers into the in-flight promise. Abort a check after 15 seconds; cap response size and pagination to avoid unbounded work. Failure or incomplete traversal is not a successful no-update result.
2. Query the public GitHub releases list for `sebastienferry/sectile` over HTTPS without workstation credentials, with GitHub JSON accept and application User-Agent headers. Follow pagination (100 releases/page, at most 10 pages); select highest numeric stable SemVer, not publication-date order. Accept optional leading `v`, reject invalid/draft/prerelease records and pre-release identifiers; ignore SemVer build metadata for precedence. Detect a newer incomplete stable release separately from a compatible candidate: a lower compatible newer candidate can still be offered with the incomplete-release explanation. If none is compatible, report platform unavailability. Invalid installed versions yield comparison-unavailable.
3. Match exact nonempty assets produced by `scripts/release/package-desktop.sh`: darwin arm64/x64 maps to `sectile-desktop-darwin-arm64.zip` / `sectile-desktop-darwin-amd64.zip`; linux x64 to `sectile-desktop-linux-amd64.tar.gz`; win32 x64 to `sectile-desktop-windows-amd64.zip`. Other targets are unavailable unless their exact supported archive exists under the same naming convention (win32 becomes windows, arm64 remains arm64, x64 becomes amd64). Do not offer an archive for another architecture or infer its URL from an incomplete release.
4. Persist an update-specific section through `desktop/electron/settings-file.cjs`, preserving unrelated settings. Store last successful result, checkedAt, and a set of normalized versions whose notifications were attempted. Validate cached data on load, tolerate corrupt/missing storage, and recompute eligibility against installed version. Persist the notification marker before attempting delivery; storage failure suppresses notification rather than risking repeats. Cache is displayed as last known information, never substituted for a successful network check. An unsuccessful refresh retains the last success and exposes a separate error. HTTP 403/429 report throttling; automatic retries respect Retry-After/rate-limit reset when supplied, without busy retry loops; manual checks during that deadline report throttling without a request.
5. Expose `desktopUpdates()`, `checkDesktopUpdates()`, and `onDesktopUpdates()` via explicit preload IPC. The main process owns `{status, installedVersion, checkedAt, lastAttemptAt, available, unavailableVersion, error}`; status is idle/checking/up-to-date/available/unavailable/uncomparable/error. `available` carries version, releaseUrl and downloadUrl; error uses bounded public codes (network, timeout, throttled, invalid-response, storage). Never expose tokens or raw response bodies. Broadcast state changes and provide a snapshot for a renderer attaching late. UI removes detached subscriptions.
6. Add a labeled update marker beside the existing Settings entry (independent of its agent mismatch marker), and update controls/status to Changelog settings. Render remote metadata as text, never HTML. Use explicit browser actions through the main process; validate HTTPS host/path against this repository's GitHub release page/download paths, reject credentials and arbitrary remote redirects/URLs. Requesting metadata is allowed; fetching archives is not. Use a separate update notification payload with the existing permission/failure conventions, delivered from the main process or one renderer under the service's durable claim.

## Target Files and Contracts

- New service and unit tests: `desktop/electron/desktop-updates.cjs`, `desktop/tests/desktop-updates.test.cjs`.
- Wiring/persistence: `desktop/electron/main.cjs`, `desktop/electron/preload.cjs`, `desktop/electron/settings-file.cjs` if needed.
- Presentation: `desktop/src/main.js`, existing Desktop stylesheet; focused UI tests alongside `desktop/tests/settings-version.ui.cjs`.
- User-facing changelog: `CHANGELOG.md` at implementation time only.

## Alternatives Rejected

- Electron auto-updater: installation and distribution/signing decisions exceed accepted scope.
- Agent/server-mediated checks: unnecessary connectivity dependency for Desktop updates.
- Lexical comparison or rolling GitLab metadata: wrong version precedence or unapproved channel.
- Renderer-only timers and in-memory deduplication: reload/restart would repeat checks and notifications.
- Clearing availability on errors: mistakes offline state for a successful no-update result.

## Risks / Validation

GitHub throttling and partially published archives are normal states. Bound requests and retain last known information. Validate parser/asset selection, injected scheduling, deduplication persistence and failure handling with deterministic fixtures rather than live network tests. UI tests cover offline settings, availability and external actions without downloading. No data migration; older settings without the section remain valid. No remaining owner decisions.
