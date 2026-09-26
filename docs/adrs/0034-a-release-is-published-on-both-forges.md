# ADR 0034: A release is published on both forges

Status: Accepted (#428)

Amends: [ADR 0018](0018-semver-tags-and-changelog.md), the part that says
where a release is published.

## Context

ADR 0018 made a release a `vX.Y.Z` tag, built by the GitLab tag pipeline and
uploaded to the Generic Package Registry of the GitLab mirror. That package held
the agent and server binaries only. Sectile Desktop had no distributable form:
`make desktop-package` builds an unpacked app for the host platform, so using
the desktop app meant cloning the repository and installing Node and Go.

Two facts shaped where the new desktop archives go:

- **The repository is developed on GitHub and is public**, while the pipeline
  runs on a GitLab mirror that knows nothing of GitHub. Somebody who finds
  Sectile on GitHub finds no release page, and the GitLab package is not where
  they look.
- **macOS is best packaged on macOS.** `@electron/packager` writes the asar
  integrity digest and re-signs the Electron Framework ad-hoc only when it runs
  on macOS. Off macOS it leaves the framework's shipped ad-hoc signature
  untouched, which keeps the app launchable on Apple Silicon but without the
  digest. The GitLab runners are Linux; GitHub offers macOS runners.

## Decision

**A release tag is built and published twice, by two independent streams, and
each stream publishes the whole release set.**

- **GitLab** keeps its stream. The tag pipeline cross-packages the four desktop
  targets (darwin-arm64, darwin-amd64, linux-amd64, windows-amd64) on its Linux
  runner, and `publish:binaries` uploads them to the package `sectile/<tag>`
  next to the binaries, after the Go, web and desktop test suites passed.
- **GitHub** gets a stream of its own. `.github/workflows/release.yml` runs on
  the tag pushed to the public repository, builds the same set on GitHub's
  runners (macOS archives on macOS) and creates the GitHub Release of the tag,
  with the tag's `CHANGELOG.md` section as its notes. It authenticates with the
  built-in `GITHUB_TOKEN` only.
- **Both streams run the same steps**, from `scripts/release/`: the Go
  cross-compile loop, the desktop version check, the desktop packaging and the
  changelog extraction. The streams cannot drift on targets, flags or file
  names, and the scripts name no internal host, group, project or secret, since
  the public repository runs them.
- **Each stream vouches for its own files.** Each writes a `SHA256SUMS` over
  what it publishes; the archives of the two streams are equivalent, not
  byte-identical.
- **Desktop archives come from `vX.Y.Z` tags only**, like the binaries. A
  branch, a merge into `main` or a suffixed tag packages no desktop app.
- **The packages are unsigned.** The macOS apps carry an ad-hoc signature, and
  the install guide gives the one command that removes the quarantine mark.
  Developer ID signing and notarization are a later decision, not a step of
  this one.

## Consequences

- **A tag produces fifteen files in each place**: ten Go binaries, four desktop
  archives and `SHA256SUMS`, with no manual step.
- **Two builds, two sets of checksums.** A user checks an archive against the
  `SHA256SUMS` of the place they downloaded it from, never across forges. The
  install guide says so.
- **No credential crosses forges.** GitLab holds no GitHub token and GitHub no
  GitLab one. Neither stream waits on the other, and the failure of one does
  not undo the other: a release can exist on one forge only until the other is
  re-run.
- **The GitHub stream does not run the Go test suite.** It runs the desktop unit
  tests and the builds it publishes. The Go suite gates the GitLab stream of the
  same commit, and the tag is cut from a `main` whose pipeline already ran it.
  Adding a Go test job before the release job is one block, should that change.
- **The GitLab macOS archives carry no asar integrity digest**, unlike the
  GitHub ones. The digest is a tamper check an unsigned app cannot enforce
  against a determined attacker anyway. Should a cross-packaged Apple Silicon
  app fail to launch, the fallback is an ad-hoc signing step with a portable
  signer in `scripts/release/package-desktop.sh`.
- **A release must bump `desktop/package.json`.** Both streams refuse a tag
  whose desktop manifest says another version, since the packaged app shows
  that version in its settings. `AGENTS.md` step 4 already bumps it.

## Alternatives rejected

- **GitLab only.** Rejected because the public repository, where users arrive,
  would still have no release to download.
- **A GitLab job creating the GitHub Release**, with a GitHub token stored as a
  masked GitLab CI variable. Rejected because it puts a credential with write
  access to the public repository in the mirror's settings, and keeps macOS
  packaging on Linux.
- **A local `make` target producing the archives**, uploaded by hand. Rejected
  because a release would then depend on one workstation and one person
  remembering, which is what ADR 0018 set out to remove.
