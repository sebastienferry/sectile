# ADR 0018: A release is a SemVer tag, and the changelog ships with it

Status: Accepted

## Context

Sectile had no releases. `main` was what the team's board ran, the pipeline
published a set of workstation binaries on every branch under a version named
after the pipeline — `61-main`, `58-fix-144` — and the container image carried
the same string. Three consequences, all of them quiet:

- **Nobody could say what they were running.** No binary, no interface and no
  API answered the question. A bug report named a machine and a day.
- **The package registry listed builds, not versions.** Every branch pushed an
  artefact somebody could install, named after a pipeline counter that means
  nothing outside the mirror and cannot be checked out.
- **There was nothing to read.** No changelog anywhere, so the only account of
  what had changed was 559 commit subjects, half of them `docs:` and `chore:`.

The trigger was the reverse of the usual one: not a failed release, but the
absence of the concept. ADR 0016 had to reason around it explicitly — it
rejected promoting dev on a git tag because "that repository releases by
tagging and this one does not; a tag convention would have to be invented
first". This record invents it.

## Decision

**A release of Sectile is a Git tag `vX.Y.Z` following Semantic Versioning, and
nothing else is a release.**

- **The tag is the only source of the version.** It reaches the Go binaries
  through link-time flags (`-X tasks/internal/version.Version=…`) and nothing
  else. No file in the repository declares the product version, because a
  declared version drifts from the tag the first time somebody forgets to bump
  it, and the binary then claims a release it was not cut from. A build made
  outside a tag says `dev`, and saying `dev` is the point.
- **Every executable reports it.** `sectile-server --version`,
  `sectile-agent --version`, `GET /api/version`, the agent's `/health`, the web
  interface footer and the General section of the desktop settings.
- **Binaries are published on tags only.** The workstation binaries are what
  somebody installs, so they are releases by definition; publishing one per
  branch turned the Generic Package Registry into a list of pipelines.
- **The server image keeps being built on every ref.** It is not a thing
  anybody installs by hand: it is what dev and the ephemeral test environments
  run, and both need an image per branch (ADR 0016, and argocd-sp's `testenv`
  feature pinning `preview-<sha>`). The interface is compiled inside that
  image, so a merge into `main` still builds and deploys the web UX.
- **Dev promotion stays on the merge, not on the tag.** ADR 0016 rejected a tag
  trigger because no tag convention existed. One exists now, and the answer is
  unchanged for a different reason: dev shows what `main` holds, which is a
  statement about the branch and not about releases. A tagged deployment would
  be a new environment, which is a decision rather than a follow-up.
- **`CHANGELOG.md` lives at the repository root, in Keep a Changelog format,
  written in English only**, whatever language the commits and the
  conversations are in.
- **The changelog ships inside the product.** The server embeds it and serves
  it at `GET /api/changelog`; the web interface opens it from the version in
  the footer; the desktop app inlines it at build time and shows it in its
  settings. A release note the user has to go and find on a forge is a release
  note nobody reads.
- **Cutting a tag is a written procedure, in `AGENTS.md`.** It derives the bump
  from Conventional Commits, writes the changelog entries, bumps the two
  Node manifests, and stops at one confirmation gate before the tag exists.

## Consequences

- **A binary can be traced back to a commit.** `--version` prints the tag and
  the sha it was built from; the sha is stamped by the pipeline and, failing
  that, by the Go toolchain's own VCS settings.
- **`build:web`, `build:binaries` and `publish:binaries` no longer run on
  branches.** A branch pipeline is now tests plus the image. Somebody who needs
  a binary from a branch builds it (`make build-release`), and it will honestly
  report `dev`.
- **The changelog is a release artefact, not a courtesy.** A tag cut without
  entries ships a product that shows an empty panel to its users, so the
  procedure treats the entries as part of the release rather than as
  documentation to write afterwards.
- **`CHANGELOG.md` is compiled in, so it cannot be removed from the build
  context.** `.dockerignore` excludes `*.md` and now carries an explicit
  exception; dropping it breaks the build rather than shipping a blank panel.
- **A release is published on both forges.** The GitLab package and a GitHub
  Release each carry the binaries and the Sectile Desktop archives, built by
  two independent streams of the same tag; see
  [ADR 0034](0034-a-release-is-published-on-both-forges.md).
- **The desktop app's version comes from its own manifest**, which the release
  procedure bumps. A workstation that upgraded the app and not the agent sees
  two different numbers side by side, which is the point of showing both.
- **The image's version depends on the shared kaniko job forwarding build
  arguments.** `publish:image` passes `--build-arg SECTILE_VERSION=…` through
  `DOCKER_BUILD_ARGS`; if the shared template ignores it, the image reports
  `dev` while its registry tag is still the release. The published binaries are
  stamped by this repository's own job and never depend on that.

## Alternatives rejected

- **A `VERSION` file, or the version in `package.json`, as the source.**
  Rejected because it is a second place to be right. The release procedure does
  bump the Node manifests, but they follow the tag rather than defining it, and
  the Go binaries never read them.
- **Deriving the version from `git describe` at build time in CI.** Rejected
  for the published artefacts: `git describe` answers `v0.1.0-3-gabc1234` on
  the commit after a tag, which is a useful string for a developer and a
  confusing one on a release. The Makefile uses it precisely because a local
  build is not a release.
- **Serving the changelog from the forge's releases API.** Rejected because it
  makes an offline desktop app and an air-gapped server show nothing, and puts
  a network call on a settings panel.
- **Keeping a per-branch binary publication alongside the tagged one.**
  Rejected because the two are indistinguishable once installed: the whole
  problem was artefacts whose version nobody could act on.
