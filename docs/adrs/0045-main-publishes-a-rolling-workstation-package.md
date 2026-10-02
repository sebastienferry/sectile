# ADR 0045: A merge into main publishes a rolling workstation package

Status: Accepted

Amends: [ADR 0018](0018-semver-tags-and-changelog.md), the part that says
binaries are published on tags only.

## Context

ADR 0018 made the workstation binaries a release artefact: the agent, the
server and, since [ADR 0034](0034-a-release-is-published-on-both-forges.md),
the Sectile Desktop archives are built and uploaded on a `vX.Y.Z` tag and on
nothing else. A merge into `main` publishes the server image, which is promoted
to dev ([ADR 0016](0016-promotion-automatique-en-dev.md)).

That leaves a gap between releases. Dev runs what `main` holds as soon as it
merges, but the local app that talks to it does not exist anywhere until the
next tag: a workstation either stays on the last release, which may not speak
what the dev server now expects, or builds `main` by hand. The people who run
dev are exactly the ones who need the matching local app.

## Decision

**Every merge into `main` builds the agent, the server and the four Sectile
Desktop archives, and uploads them to the package `sectile`, version `main`.**

- **One rolling version, not one per merge.** Each merge uploads every file
  again into the same `main` version. Of the files sharing a name in a version,
  GitLab serves the most recent, so
  `.../packages/generic/sectile/main/<file>` never changes and always serves
  the latest merge. The registry still lists releases, plus one entry for
  `main`, rather than a pipeline per merge, which was ADR 0018's complaint.
- **The binaries report `<iid>-main`, not `dev`.** ADR 0018 has a build made
  outside a tag say `dev`. That holds for a build made on a workstation, which
  cannot be traced to anything. A `main` build can be traced: `<iid>-main` is
  the exact string the image promoted to dev carries, so a workstation's
  `--version` names the dev deployment it matches, and the commit sha stamped
  next to it can be checked out. It does not look like a release, since it is
  not SemVer.
- **The desktop manifest is not touched.** The app's settings show
  `desktop/package.json`'s version, the last release's, next to the bundled
  agent's `<iid>-main`. The manifest follows tags (AGENTS.md, step 4), and
  packager takes the app version from it, so rewriting it per build would mean
  feeding a non-SemVer string to every target's packaging for no gain: the
  agent's version already names the build.
- **Tags are unchanged.** A `vX.Y.Z` tag still publishes under its own version,
  is still checked against `desktop/package.json`, and is still built again by
  the GitHub release workflow. `main` is published on the GitLab mirror only.
- **Other branches still publish no binaries.** A branch pipeline stays tests
  plus the image.
- **Uploads to the package are serialised** (`resource_group`), so two merges
  close together cannot leave one build's `SHA256SUMS` beside the other's
  files.

## Consequences

- **`main` is an install channel without a changelog.** Whoever installs it
  takes what merged; `CHANGELOG.md`'s `[Unreleased]` section, shipped inside
  the build, is the account of it.
- **Storage grows with every merge unless duplicates are cleaned up.** Each
  merge adds another copy of every file to the `main` version. The project's
  package cleanup policy ("Number of duplicated assets to keep") bounds it; set
  to 1, only the latest copy of each file is kept.
- **A merge pipeline takes longer.** It now cross-compiles five targets and
  packages four Electron apps before `publish:binaries`. The image and its
  promotion to dev do not wait on them.

## Alternatives rejected

- **One package version per merge (`<iid>-main`).** Every build would stay
  downloadable, but the registry would go back to being a list of pipelines,
  and there would be no stable URL to point a workstation at.
- **Per-branch packages.** Rejected by ADR 0018 for the same reasons, which
  this record does not revisit.
- **Pipeline artifacts instead of the package registry.** They expire, and
  their URL names a job id, so nothing stable could point at them.
