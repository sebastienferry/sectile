# Plan #428 - Package Desktop binaries

Implementation plan for `spec.md`. Behaviour lives there; this file holds the
technical choices.

## Stack and existing pieces

- `desktop/electron/package.cjs` calls `@electron/packager` 20.3 for the host
  platform only, copying `desktop/bin/sectile-agent[.exe]` as an extra
  resource, output in `desktop/release/`. `make desktop-package` drives it.
- `desktop/electron/runtime.cjs` resolves the bundled agent at
  `process.resourcesPath/<agentName()>`; `agentName()` takes a platform
  argument (`win32` gives `sectile-agent.exe`).
- `.gitlab-ci.yml`: `build:web` → `build:binaries` (5 targets × server/agent,
  `SHA256SUMS`) → `publish:binaries` (curl upload to
  `packages/generic/sectile/<tag>`), all tag-only, gated on `test:go`,
  `test:web`, `test:desktop`. `ELECTRON_SKIP_BINARY_DOWNLOAD=1` is global.
- `@electron/packager` 20.3 facts that shape the plan (read in
  `node_modules/@electron/packager/dist`):
  - Windows resources are edited with `resedit` (pure JavaScript): packaging
    `win32` from Linux needs no Wine.
  - Packaging `darwin` off macOS skips the asar integrity digest with a
    warning, precisely so that the Electron Framework's shipped ad-hoc
    signature stays valid and Apple Silicon keeps launching the app. On macOS
    it writes the digest and re-signs the framework ad-hoc with `codesign`.
  - It downloads the Electron zip of each target itself through
    `@electron/get`, which `ELECTRON_SKIP_BINARY_DOWNLOAD` (an `electron`
    postinstall switch) does not affect. To confirm in T1.4.
- `desktop/package.json` version is bumped to the release version by the
  release procedure (`AGENTS.md` step 4) and is what `app.getVersion()` and
  packager's `appVersion` read.

## Decisions

1. **One packaging entry point, parameterised by target.** `package.cjs`
   accepts `--platform <darwin|linux|win32>`, `--arch <arm64|x64>`,
   `--agent <path>` and `--out <dir>`; with no argument it behaves exactly as
   today (host platform and arch, `desktop/bin/<agentName()>`,
   `desktop/release/`). Option building moves to a pure function in
   `desktop/electron/package-options.cjs` so it is unit-tested without
   running packager.
2. **The agent is staged under its runtime name.** CI agents are named
   `sectile-agent-<os>-<arch>[.exe]`, the runtime looks for
   `sectile-agent[.exe]`. `package.cjs` copies `--agent` into a temporary
   directory as `agentName(targetPlatform)` (the target's, not the host's),
   mode `0755`, and passes that file as `extraResource`. The temporary
   directory is removed afterwards.
3. **A shell script per release step, shared by both streams.** Under
   `scripts/release/`, POSIX `sh`, no internal name:
   - `build-binaries.sh <version> <commit> <outdir>`: the Go matrix loop now
     inline in `build:binaries`, moved verbatim (same targets, flags, names).
     GitLab's `build:binaries` calls it, so both streams cannot drift.
   - `check-desktop-version.sh <tag>`: fails unless
     `v$(node -p "require('./desktop/package.json').version")` equals the tag.
   - `package-desktop.sh <tag> <os> <arch> <agent> <outdir>`: maps Go tokens to
     Electron ones (`amd64`→`x64`, `windows`→`win32`), runs
     `node electron/package.cjs` with decision 1's arguments, then archives
     the packager directory as `sectile-desktop-<os>-<arch>.<ext>` (decision
     4). For `linux`, it also runs the bundled agent's `--version` and greps
     the tag when the runner is linux/amd64 (spec FR7).
   - `changelog-section.mjs <X.Y.Z>`: prints the body of `## [X.Y.Z]` from
     `CHANGELOG.md` (up to the next `## [` heading or the link definitions),
     trimmed; exits non-zero with `CHANGELOG.md has no section for X.Y.Z`
     when absent or empty.
4. **Archiving.** `zip -qry` for `darwin` and `win32` (`-y` stores the
   framework symlinks as links), `tar -czf` for `linux` (keeps modes). The
   archive root is the packager directory `Sectile-<platform>-<arch>/`. `zip`
   is installed with `apt-get` in the GitLab Node job; GitHub's
   `ubuntu-latest` and `macos-latest` images ship it.
5. **GitLab: one new job, SHA256SUMS moves to publication.**
   - `build:desktop` (stage `build`, image `${NODE_IMAGE}`, tag-only, needs
     `build:binaries` artifacts, `.desktop-cache` plus an `ELECTRON_CACHE`
     directory keyed on `desktop/package-lock.json`): checks the desktop
     version, `npm ci`, `npm run build`, then `package-desktop.sh` for the
     four targets from `dist/sectile-agent-*`, all cross-packaged on the
     Linux runner. Artifacts: `dist-desktop/`, one week.
   - `build:binaries` stops writing `SHA256SUMS` (it keeps its `--version`
     checks). `publish:binaries` also needs `build:desktop`, moves
     `dist-desktop/*` into `dist/`, writes `SHA256SUMS` over `dist/*`
     (busybox `sha256sum` in the curl image), then uploads as today.
   - Only `build:desktop` downloads Electron; the global
     `ELECTRON_SKIP_BINARY_DOWNLOAD=1` stays, since `npm ci` does not need the
     host binary to package (confirmed in T1.4, else overridden in that job
     only).
6. **GitLab macOS archives are cross-packaged and ad-hoc signed as Electron
   ships them.** Packager keeps the framework's shipped signature valid off
   macOS (see *Stack*). The GitLab darwin archives therefore carry no asar
   integrity digest, unlike the GitHub ones. Accepted: the digest is a
   tamper check that an unsigned app cannot enforce against a determined
   attacker anyway. Fallback, only if T5.3's manual launch on Apple Silicon
   fails: add an ad-hoc `rcodesign sign` step (apple-codesign, pinned
   release, checksum-verified download) to `package-desktop.sh` for darwin
   on non-macOS hosts.
7. **GitHub workflow `.github/workflows/release.yml`.** Trigger
   `push: tags: ['v[0-9]+.[0-9]+.[0-9]+']`; top-level
   `permissions: contents: read`; `concurrency: release-${{ github.ref }}`.
   Actions pinned by full commit SHA with the version in a comment. Jobs:
   - `binaries` (`ubuntu-latest`): checkout, `setup-node` 22,
     `setup-go` with `go-version-file: go.mod`; `check-desktop-version.sh`;
     `web` `npm ci` + `npm run build`; `desktop` `npm ci` + `npm test`;
     `build-binaries.sh "$GITHUB_REF_NAME" "$GITHUB_SHA" dist`; the same
     `--version` checks as GitLab; upload `dist/` as artifact `binaries`.
   - `desktop` (needs `binaries`), a two-entry matrix: on `ubuntu-latest` it
     packages `linux amd64` and `windows amd64`; on `macos-latest` it
     packages `darwin arm64` and `darwin amd64` natively, so packager writes
     the integrity digest and re-signs ad-hoc with `codesign`.
   - Both desktop jobs `chmod +x` the downloaded agents first
     (`actions/download-artifact` drops file modes), run `npm ci` and
     `npm run build` in `desktop/`, and upload their archives as artifacts.
   - `release` (`ubuntu-latest`, needs all three, `permissions:
     contents: write`): downloads every artifact into one directory, writes
     `SHA256SUMS`, writes the notes with `changelog-section.mjs`, then
     `gh release create "$TAG" --verify-tag --title "$TAG" --notes-file
     notes.md --latest <files>`; when `gh release view "$TAG"` succeeds, it
     runs `gh release upload "$TAG" --clobber <files>` and
     `gh release edit "$TAG" --notes-file notes.md` instead (spec FR13).
     `GH_TOKEN: ${{ github.token }}`.
8. **Electron download caching.** GitLab caches `ELECTRON_CACHE`; GitHub
   uses `actions/cache` on the same path keyed on
   `desktop/package-lock.json`. About 100 MB per target, four targets.
9. **GitHub gate (spec FR10).** Desktop unit tests, web build and binary
   version checks run on GitHub; the Go suite (`-race`, about 8 minutes on
   `internal/db` alone) and PostgreSQL tests do not. The tag is cut from a
   `main` whose GitLab pipeline already ran them, and GitLab's tag pipeline
   gates its own stream on them. Reversible: adding a `go test` job to
   `needs:` of `release` is one block.
10. **ADR 0034 "A release is published on both forges".** Context: the
    public repository has no release page; the pipeline lives on the GitLab
    mirror. Decision: GitHub Actions builds and publishes the GitHub
    Release from the tag with `GITHUB_TOKEN`; GitLab keeps its package.
    Rejected: GitLab only; a GitLab job pushing to GitHub with a stored
    GitHub token; a local `make` target. Consequences: two builds, two
    `SHA256SUMS`, no cross-forge credential, macOS runners for free on
    GitHub. ADR 0018 gains a consequence line pointing to it.

## Target files

| File | Change |
| --- | --- |
| `desktop/electron/package-options.cjs` | New: pure option builder (decisions 1, 2) |
| `desktop/electron/package.cjs` | Parse arguments, stage the agent, call packager |
| `desktop/tests/package-options.test.cjs` | New: unit tests of the option builder |
| `scripts/release/build-binaries.sh` | New: Go matrix loop moved from `.gitlab-ci.yml` |
| `scripts/release/check-desktop-version.sh` | New |
| `scripts/release/package-desktop.sh` | New |
| `scripts/release/changelog-section.mjs` | New |
| `desktop/tests/changelog-section.test.mjs` | New: runs in `test:desktop` |
| `.gitlab-ci.yml` | `build:binaries` calls the script; new `build:desktop`; `publish:binaries` needs it and writes `SHA256SUMS`; header comment |
| `.github/workflows/release.yml` | New (decision 7) |
| `desktop/README.md` | "Install a release" section; "Package and verify" mentions the release archives |
| `README.md` | Releases section points to the desktop archives and the install guide |
| `AGENTS.md` | Step 6 describes both streams |
| `docs/adrs/0003-*.md` | Unsigned statement covers release packages |
| `docs/adrs/0018-semver-tags-and-changelog.md` | Consequence line, link to ADR 0034 |
| `docs/adrs/0034-a-release-is-published-on-both-forges.md` | New (decision 10) |
| `CHANGELOG.md` | One `Added` line (spec FR18) |

## Data contracts

- `package.cjs` CLI: `node electron/package.cjs [--platform P] [--arch A]
  [--agent PATH] [--out DIR]`; prints the packager output directory; exit
  code 1 on any error, including a missing agent file.
- `package-desktop.sh <tag> <os> <arch> <agent> <outdir>` output: exactly one file
  `<outdir>/sectile-desktop-<os>-<arch>.<zip|tar.gz>`.
- `changelog-section.mjs` output: the section body on stdout, UTF-8, no
  heading; exit 1 and a message on stderr when missing.
- Artifacts per stream: 15 files (spec *Success criteria*).

## Test plan

- Unit (`npm test` in `desktop/`, so in `test:desktop` and GitHub's
  `binaries` job):
  - `package-options.test.cjs`: defaults equal today's options; `win32`
    target stages `sectile-agent.exe` whatever the host; an unknown platform
    or arch is an error (the builder takes Electron tokens, the script maps
    Go ones); `ignore` still excludes `bin`, `tests`, `release*`.
  - `changelog-section.test.mjs`: section in the middle, last section before
    link definitions, missing section, empty section, `[Unreleased]` never
    matched for a version.
- Local, on the author's Mac: `package-desktop.sh` for the four targets from
  a `make build-release`-style `dist/`; list the archives (`zipinfo`, `tar
  -tvf`) to check symlinks, modes and the agent location.
- Release rehearsal: the GitHub workflow runs on a personal fork of the
  public repository with a `v0.0.1` tag (a suffixed tag is ignored by the
  trigger by design). The GitLab stream is rehearsed on a scratch tag only if
  the owner agrees, since a tag on the mirror is public; otherwise the first
  real release is its rehearsal.
- Manual acceptance (spec US1): launch each archive on its platform, from
  both streams for macOS arm64 (decision 6), and read the versions in the
  settings.

## Risks

- **Cross-packaged darwin-arm64 does not launch** (decision 6): detected by
  the manual acceptance; fallback `rcodesign`.
- **Runner disk and time**: four Electron zips plus four extracted apps,
  about 1 GB on the GitLab runner. `build:desktop` cleans each packager
  directory after archiving.
- **GitHub `macos-latest` label moves** (arm64 today): the job packages both
  darwin arches whatever the host arch, so a label change does not change
  the output.
