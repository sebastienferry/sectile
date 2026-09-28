# Specification #428 - Package Desktop binaries

- Ticket: https://github.com/sebastienferry/sectile/issues/428
- Branch: `feat/428`
- Clarification: `docs/clarifications/428.md` (rounds 1 to 3, confirmed by the
  owner on 2026-09-26), plus two decisions taken during specification (FR3,
  FR10)
- Framework: Spec Kit

## Summary

Every release tag `vX.Y.Z` produces a downloadable Sectile Desktop archive for
four platforms. Each archive holds the desktop app together with the Sectile
agent built for the same platform, so a user can download it, extract it and
run Sectile without cloning the repository or installing Node or Go.

The archives are published twice, by two independent builds of the same tag:

- the GitLab tag pipeline adds them to the Generic Package Registry package
  `sectile/<tag>`, next to the agent and server binaries;
- a GitHub Actions workflow on the public repository creates a GitHub Release
  for the tag carrying the full release set: the four desktop archives, the
  agent and server binaries, and `SHA256SUMS`, with the tag's changelog
  section as release notes.

The packages are unsigned (ad-hoc signed on macOS). The documentation explains
how to open them anyway.

## Scope

In scope:

- Tag-only packaging of Sectile Desktop for darwin-arm64, darwin-amd64,
  linux-amd64 and windows-amd64.
- Publication to the GitLab package `sectile/<tag>` and to a GitHub Release.
- Checksums for every published file.
- Install documentation, including the macOS Gatekeeper workaround.
- One changelog line, and the release documentation (ADRs, `AGENTS.md`) that
  describes the second stream.

Out of scope:

- Developer ID signing, notarization and Authenticode signing (follow-up
  ticket).
- Native installers (`.dmg`, NSIS, MSI, AppImage, `.deb`).
- Auto-update.
- A linux-arm64 desktop package (the linux-arm64 agent and server binaries
  keep being published).
- Desktop packages built on branches or on merges into `main`.
- Any change to what the desktop app does once running.
- `make release` / `make build-release` and their `server-*` / `agent-*`
  file names.

## Definitions

- **Release tag**: a Git tag matching `vX.Y.Z` (SemVer, no suffix), as
  `AGENTS.md` cuts it.
- **Release version**: `X.Y.Z`, the release tag without its leading `v`.
- **Target**: one of the four `<os>-<arch>` pairs `darwin-arm64`,
  `darwin-amd64`, `linux-amd64`, `windows-amd64`. The tokens are the Go ones,
  as in the `sectile-agent-<os>-<arch>` files published next to them.
- **Desktop archive**: the file published for one target:
  `sectile-desktop-darwin-arm64.zip`, `sectile-desktop-darwin-amd64.zip`,
  `sectile-desktop-linux-amd64.tar.gz`, `sectile-desktop-windows-amd64.zip`.
- **Release set**: every file a stream publishes for one tag: the four desktop
  archives, the ten `sectile-agent-*` and `sectile-server-*` binaries of the Go
  matrix, and `SHA256SUMS`.
- **Stream**: one of the two publications, GitLab (tag pipeline, Generic
  Package Registry) or GitHub (Actions workflow, GitHub Release).
- **Bundled agent**: the `sectile-agent` executable inside a desktop archive,
  which the packaged app starts.

## User stories (prioritised)

### US1 - Install the desktop app from a release (P1)

As a Sectile user, I download the desktop archive for my platform from a
release, extract it and start Sectile, without building anything.

**Acceptance scenarios**

1. **Given** release `v0.9.0` is published, **When** I download
   `sectile-desktop-darwin-arm64.zip` on an Apple Silicon Mac, extract it,
   remove the quarantine attribute as the documentation says and open
   `Sectile.app`, **Then** the app starts and connects to its bundled agent.
2. **Given** the same release, **When** I do the same with
   `sectile-desktop-darwin-amd64.zip` on an Intel Mac, **Then** the app starts
   and connects to its bundled agent.
3. **Given** the same release, **When** I extract
   `sectile-desktop-linux-amd64.tar.gz` on an x86-64 Linux desktop and run
   `Sectile`, **Then** the app starts with no `chmod` needed.
4. **Given** the same release, **When** I extract
   `sectile-desktop-windows-amd64.zip` on 64-bit Windows and run
   `Sectile.exe`, confirming the SmartScreen prompt as documented, **Then** the
   app starts.
5. **Given** any of those apps is running, **When** I open the General section
   of the settings, **Then** the desktop version reads `0.9.0` and the agent
   version reads `v0.9.0`.

### US2 - Download a release from GitHub (P1)

As a user of the public repository, I find every file of a release on its
GitHub Release page, with its release notes.

**Acceptance scenarios**

1. **Given** tag `v0.9.0` is pushed to GitHub, **When** the release workflow
   completes, **Then** a GitHub Release named `v0.9.0` exists for that tag and
   carries the release set: four desktop archives, ten Go binaries and
   `SHA256SUMS`.
2. **Given** that release, **When** I read its notes, **Then** they are the
   body of the `## [0.9.0]` section of `CHANGELOG.md` at that tag.
3. **Given** that release, **When** I run `sha256sum -c SHA256SUMS` in a
   directory holding every file of the release, **Then** every line reports
   `OK`.
4. **Given** the workflow is re-run for the same tag, **When** it completes,
   **Then** there is still one release for the tag, its files are replaced
   rather than duplicated, and its notes are refreshed.

### US3 - Download a release from the GitLab package registry (P2)

As a user of the GitLab mirror, I find the desktop archives in the same
package as the agent and server binaries.

**Acceptance scenarios**

1. **Given** the tag pipeline for `v0.9.0` succeeds, **When** I open the
   package `sectile` version `v0.9.0`, **Then** it lists the release set.
2. **Given** that package, **When** I verify the files against its
   `SHA256SUMS`, **Then** every file, desktop archives included, matches.
3. **Given** a pipeline on a branch or on `main`, **When** it completes,
   **Then** it has built and published no desktop archive.

### US4 - A broken release is not published (P2)

As the person cutting a release, I would rather see the release fail than
publish archives that lie about their version or lack their notes.

**Acceptance scenarios**

1. **Given** `desktop/package.json` says `0.8.0` on the commit tagged
   `v0.9.0`, **When** either stream runs, **Then** it fails before publishing
   anything, and the log says the desktop version does not match the tag.
2. **Given** `CHANGELOG.md` has no `## [0.9.0]` section at tag `v0.9.0`,
   **When** the GitHub workflow runs, **Then** it fails before creating the
   release, and the log names the missing section.
3. **Given** a test job fails in the GitLab tag pipeline, **When** the
   pipeline ends, **Then** no file of that tag was uploaded, desktop archives
   included.
4. **Given** the bundled agent of the Linux archive does not report the tag
   for `--version`, **When** the stream packages it, **Then** the stream fails
   before publishing.

## Functional requirements

### Production

- **FR1 - Tag only.** Desktop archives are built and published only for a
  release tag. A branch pipeline, a merge into `main` and a tag that is not
  `vX.Y.Z` build none. The local `make desktop-package` keeps producing an
  unpacked host-platform package in `desktop/release/`, unchanged.
- **FR2 - Targets.** Each stream produces exactly the four desktop archives
  named in *Definitions*, one per target. No linux-arm64 desktop archive.
- **FR3 - Archive names use the Go architecture tokens** (`amd64`, `arm64`),
  like the agent and server binaries they sit next to. *(Decision taken
  during specification: the clarification fixed the pattern
  `sectile-desktop-<os>-<arch>` "alongside `sectile-agent-<os>-<arch>`", and
  the tokens follow that neighbour.)*
- **FR4 - Format.** The macOS and Windows archives are `.zip`; the Linux
  archive is `.tar.gz`. No installer. Each archive holds one top-level
  directory, `Sectile-<platform>-<arch>/` as `@electron/packager` names it,
  containing the app (`Sectile.app` on macOS, `Sectile.exe` on Windows,
  `Sectile` on Linux) and the Electron license files.
- **FR5 - Archives preserve what the app needs to start.** Symbolic links
  inside `Sectile.app` (the framework bundles) and the executable bits of the
  app and of the bundled agent survive extraction with the platform's
  standard tools (Finder or `unzip` on macOS, `tar` on Linux, Explorer on
  Windows).
- **FR6 - Bundled agent.** Each archive bundles the `sectile-agent` built for
  its own target from the same commit, stamped with the release tag, at the
  location the packaged app resolves it from (`resources/sectile-agent`, with
  `.exe` on Windows). A stream bundles the agent it published itself.
- **FR7 - Versions.** The packaged app reports the release version as its own
  version (from `desktop/package.json`), and its bundled agent reports the
  release tag. A stream refuses to publish when `desktop/package.json`'s
  version differs from the release version (US4.1), and when the bundled
  agent of the Linux archive does not report the tag (US4.4).
- **FR8 - Signing.** No signing identity is used. The macOS apps carry an
  ad-hoc signature valid enough for Apple Silicon to launch them once the
  quarantine attribute is removed (US1.1, US1.2). No certificate, signing
  secret or notarization credential is added to either stream.

### Publication

- **FR9 - GitLab stream.** The four desktop archives are uploaded to the
  Generic Package Registry package `sectile`, version `<tag>`, next to the Go
  binaries, by the job that uploads those binaries, and only after the same
  test jobs (`test:go`, `test:web`, `test:desktop`) have passed. The package's
  `SHA256SUMS` lists every uploaded file, desktop archives included.
- **FR10 - GitHub stream gate.** The GitHub workflow runs the desktop unit
  tests and builds the web interface before publishing; it does not rerun the
  Go test suite, which the GitLab stream gates on for the same commit.
  *(Decision taken during specification; see plan decision 9.)*
- **FR11 - GitHub Release.** A GitHub Actions workflow triggered by a release
  tag pushed to the public repository builds the release set on GitHub's
  runners and creates the GitHub Release for that tag:
  - title: the tag (`v0.9.0`);
  - notes: the body of the tag's `CHANGELOG.md` section, without its heading;
  - assets: the release set, and nothing else;
  - not a draft, not a pre-release, marked as the latest release.
- **FR12 - GitHub credentials.** The workflow authenticates with the built-in
  `GITHUB_TOKEN` only, with `contents: write` granted to the job that creates
  the release and read-only permissions everywhere else. No GitHub credential
  is stored in GitLab; no GitLab credential is stored in GitHub.
- **FR13 - Re-runs.** Re-running the GitHub workflow for a tag whose release
  exists replaces the assets and the notes of that release (US2.4). Re-running
  the GitLab publication job re-uploads the same file names.
- **FR14 - Independent streams.** The two streams build the same commit
  separately: their archives are equivalent, not byte-identical, and each
  stream's `SHA256SUMS` vouches for its own files only. Neither stream waits
  on the other, and the failure of one does not undo the other.
- **FR15 - Public repository hygiene.** The GitHub workflow and every file it
  uses name no internal host, GitLab group, cloud project or secret.

### Documentation

- **FR16 - Install guide.** `desktop/README.md` gains an "Install a release"
  section, and the root `README.md` points to it from its releases section.
  It says, per platform: which archive to download, from where (GitHub
  Release or the GitLab package), how to check it against `SHA256SUMS`, how to
  extract and start it, and:
  - macOS: that the app is unsigned, and the exact command
    `xattr -dr com.apple.quarantine /path/to/Sectile.app` to run once before
    opening it;
  - Windows: that SmartScreen warns about an unknown publisher, and that
    **More info** then **Run anyway** starts it;
  - Linux: that no `chmod` is needed.
- **FR17 - Release documentation.** `AGENTS.md` (step 6 of the release
  procedure), ADR 0018 and the header of `.gitlab-ci.yml` describe the two
  streams, what each publishes, and that desktop archives come from tags
  only. A new ADR records "a release is published on both forges" with the
  alternatives rejected during clarification. ADR 0003's "Development
  packages are unsigned and not notarized" is updated to cover release
  packages.
- **FR18 - Changelog.** `CHANGELOG.md` gains one line under
  `## [Unreleased]` → `### Added` announcing downloadable Sectile Desktop
  packages for macOS, Linux and Windows on every release, with the agent
  included, on the GitHub Release and in the GitLab package. (#428)

## Success criteria

- A release tag produces, with no manual step, a GitHub Release and a GitLab
  package that each list 15 files (4 archives, 10 binaries, `SHA256SUMS`).
- On each of the four platforms, following only the install guide, a user
  goes from the release page to a running Sectile Desktop showing the release
  version in its settings.
- A branch pipeline produces no desktop archive, and its duration does not
  change.

## Open points

- **Windows support statement (non-blocking).** ADR 0003 says Windows
  supervision "remains unsupported", while the owner chose to ship a Windows
  package. This specification only publishes the archive and documents how to
  start it; the install guide keeps pointing to ADR 0003's statement.
  Whether that statement changes is a product decision this ticket does not
  take. It blocks nothing here.
