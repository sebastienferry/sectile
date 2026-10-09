# ADR 0056: Sectile Desktop is installed from a Scoop bucket on Windows

Status: Accepted

Builds on: [ADR 0055](0055-sectile-desktop-is-installed-from-a-homebrew-tap.md),
which publishes the macOS app as a Homebrew cask that follows the GitHub
Releases by itself.

## Context

On Windows, installing Sectile Desktop from a release means downloading
`sectile-desktop-windows-amd64.zip`, checking it against `SHA256SUMS`,
extracting it somewhere and starting `Sectile.exe` from there, with no Start
menu entry. Upgrading means doing it again. ADR 0055 removed those steps on
macOS; Windows had no equivalent.

Three package managers were weighed:

- **Scoop** installs in the user's profile without administrator rights, and
  reads manifests from *buckets*: plain public Git repositories, as Homebrew
  reads casks from taps.
- **Chocolatey** installs into `C:\ProgramData` with administrator rights. Its
  community repository moderates every version, which delays a release by hours
  to days, and needs an API key in the publishing workflow. A self-hosted feed
  is a NuGet server, not a Git repository; GitHub's NuGet feed asks every
  reader for a token, even for a public package.
- **winget** ships with Windows, but each version is a pull request on
  `microsoft/winget-pkgs` that Microsoft reviews, and opening it needs a token
  with write access somewhere.

## Decision

**Sectile Desktop for Windows is published as the app `sectile` of the Scoop
bucket `sebastienferry/scoop-sectile`, which follows the GitHub Releases by
itself, as the Homebrew tap does.**

```powershell
scoop bucket add sectile https://github.com/sebastienferry/scoop-sectile
scoop install sectile/sectile
```

- **The manifest points at the GitHub Release archive** of ADR 0034,
  `sectile-desktop-windows-amd64.zip`, with the checksum that release publishes
  in `SHA256SUMS`, and adds a **Sectile** Start menu shortcut. It builds
  nothing.
- **The bucket updates itself** with the same design as the tap: a scheduled
  workflow, also run by hand, rewrites the version, the URL and the checksum
  from the latest release, commits the manifest, installs and uninstalls it on
  a Windows runner, checks that the bundled agent reports the version, then
  pushes. Every pull request on the bucket runs the same check. This
  repository's release workflow knows nothing of the bucket and gets no new
  secret.
- **No signing step.** Windows may warn once before opening the app, as with a
  manual install, until the app is signed.

## Consequences

- **A release reaches Scoop within six hours**, or at once when somebody runs
  the bucket's workflow by hand. The release procedure of `AGENTS.md` does not
  change.
- **The bucket trusts the GitHub stream only**, for the reason ADR 0055 gives
  for the tap: the GitLab archives are not byte-identical.
- **An upgrade must stop the agent first.** Desktop starts its agent detached,
  so it outlives the window and keeps the old binary open; the bucket's notes
  and the install guide say to stop it before `scoop update sectile`.
- **Windows supervision is still unsupported**
  ([ADR 0003](0003-local-desktop-consoles.md)): the bucket makes the app easier
  to install, not more capable.

## Alternatives rejected

- **Chocolatey**, community or self-hosted: moderation delays and an API key,
  or a token for every reader, for no gain over a bucket.
- **winget**: a reviewed pull request per release on a repository we do not
  own. Worth reconsidering once the app is signed and Scoop's install step
  becomes the obstacle.
- **The manifest in this repository**: `scoop bucket add` would clone the whole
  Sectile history to read one file, and each release would need a commit on
  the protected `main`.
