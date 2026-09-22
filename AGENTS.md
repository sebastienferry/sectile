<!-- sectile:project-context:start -->
## Sectile workflow

Sectile operates the development workflow for this repository. Use `.taskflow/config.json` as the source of truth for the project and remote tracker context.

- Tracker: `github`
- GitHub repository: `sebastienferry/sectile`
- Git remote: `git@github.com:sebastienferry/sectile.git`

Development work follows Sectile's stages: clarify, specify, implement, adjust the existing pull request, then human merge and handoff. Keep the assigned branch/worktree, use Sectile's local stage handler for standalone runs, and let managed Sectile runs own stage transitions and tracker synchronization.
<!-- sectile:project-context:end -->

## Everything written in the repository is in English

Code comments, doc comments, identifiers, test names and failure messages,
commit messages, pull request descriptions, `CHANGELOG.md`, `docs/` and the
ADRs are written in English, whatever language the conversation is held in.
This holds for a file whose neighbouring comments are still in French: a
comment being touched is rewritten in English rather than continued in French.

The exception is what a user reads at runtime. The interface, the activity
steps and summaries, the log lines and the error messages Sectile shows are
written in the language that surface already speaks, which is French today.
Translating one of those in passing changes what the product says, so it is a
decision of its own, never a side effect of an edit.

## Releases: cutting a tag

A release of Sectile is a Git tag, and nothing else. The tag is what the
pipeline builds the binaries from, what those binaries report for `--version`
and on `GET /api/version`, and what the web footer and the desktop settings
show. Nothing derives a version from a file, so nothing can drift from the tag.

Follow this procedure whenever somebody asks for a tag, a release, or a
version bump. It is meant to be run end to end without further questions,
except at the one confirmation gate below.

### 1. Refuse to tag the wrong thing

Stop and say why, rather than working around any of these:

- the working tree is not clean (`git status --porcelain` prints anything);
- the checkout is not on `main`, or `main` is not up to date with `origin/main`;
- the commit to tag already carries a tag;
- `CHANGELOG.md` has no `## [Unreleased]` heading to promote.

### 2. Decide the number

Read the commits since the last tag — `git log $(git describe --tags --abbrev=0)..HEAD --pretty='%h %s%n%b'`, or the whole history when
no tag exists yet — and derive the bump from Conventional Commits:

| What the range contains | Bump |
| --- | --- |
| a `!` after the type, or a `BREAKING CHANGE:` trailer | MAJOR |
| at least one `feat` | MINOR |
| anything else worth releasing | PATCH |

While the major version is `0`, a breaking change takes the MINOR slot rather
than the MAJOR one: `0.y.z` promises nothing, and moving to `1.0.0` is a
decision about the product, never a consequence of a commit message. Ask before
proposing it.

### 3. Write the changelog entries

`CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and is written **in English only**, whatever language the conversation or the
commits are in.

- One line per user-visible change, grouped under `Added`, `Changed`,
  `Deprecated`, `Removed`, `Fixed` or `Security`. Omit the sections that would
  be empty.
- Write for whoever uses Sectile, not for whoever wrote it: name the feature
  and what changed for the reader. A line that only makes sense with the diff
  open does not belong in the file.
- `refactor`, `test`, `chore`, `style`, `ci` and `docs` commits produce no
  entry unless they changed something a user can see. Several commits on one
  feature collapse into one line.
- Reference the pull request when it helps (`(#324)`), never a bare commit sha.
- Never invent an entry, and never copy a commit subject verbatim as a line.

Then edit the file:

1. rename `## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD` (today, ISO 8601) and
   put the new entries under it;
2. add a fresh, empty `## [Unreleased]` above it;
3. update the link definitions at the bottom: point `[Unreleased]` at
   `compare/vX.Y.Z...HEAD` and add `[X.Y.Z]`.

### 4. Bump the manifests

Set the same `X.Y.Z` (no leading `v`) in `web/package.json`,
`desktop/package.json` and the `version` fields at the top of both
`package-lock.json` files. The desktop app reports its own version from its
manifest, so a manifest left behind makes the settings panel lie.

### 5. Confirm, then tag

Show the proposed version and the changelog entries, and wait for the user to
approve them. This is the only gate, and it is not optional: a tag is public
and cannot be meaningfully withdrawn.

Once approved:

```bash
git commit -am "chore(release): vX.Y.Z"
git tag -a vX.Y.Z -m "vX.Y.Z"      # message: the changelog section, verbatim
git push origin main
git push origin vX.Y.Z
```

Push the commit before the tag, so the tag never points at a commit the remote
does not have.

### 6. Say what happens next

The tag pipeline builds and publishes the binaries and the server image under
`vX.Y.Z`. A merge into `main` publishes the image only — binaries come from
tags and from nowhere else. See `.gitlab-ci.yml` and
`docs/adrs/0018-semver-tags-and-changelog.md`.
