# ADR 0055: Sectile Desktop is installed from a Homebrew tap

Status: Accepted

Builds on: [ADR 0034](0034-a-release-is-published-on-both-forges.md), which
publishes the Sectile Desktop archives on the GitHub Release of every tag.

## Context

Installing Sectile Desktop on macOS from a release takes five manual steps:
download the zip for the right architecture, check it against `SHA256SUMS`,
extract it, move `Sectile.app`, and remove the quarantine mark with `xattr`,
since the app is not notarized. Upgrading means doing all of it again. Most of
the people who use Sectile already install their tools with Homebrew.

Three facts shaped the design:

- **Homebrew only accepts notarized apps in its official casks.** Sectile
  Desktop carries an ad-hoc signature, so its cask has to live in a tap of its
  own, a public repository named `homebrew-<name>`.
- **Homebrew trusts non-official taps explicitly**, since Homebrew 6. Installing
  a cask by its fully qualified name, `user/tap/cask`, trusts that cask only.
- **No credential crosses repositories** in the release streams of ADR 0034,
  and the release scripts name no secret.

## Decision

**Sectile Desktop for macOS is published as the cask `sectile` of the tap
`sebastienferry/homebrew-sectile`, which follows the GitHub Releases by
itself.**

```sh
brew install --cask sebastienferry/sectile/sectile
```

- **The cask points at the GitHub Release archives** of ADR 0034,
  `sectile-desktop-darwin-arm64.zip` and `sectile-desktop-darwin-amd64.zip`,
  and checks each against the checksum that release publishes in `SHA256SUMS`.
  It builds nothing: Homebrew installs the very archive the release page
  offers.
- **The tap updates itself.** A scheduled workflow in the tap, also run by hand,
  reads the latest GitHub Release and its `SHA256SUMS`, rewrites the version and
  the two checksums, audits the cask, installs and uninstalls it on a macOS
  runner, then commits it with the tap's own token. This repository's release
  workflow knows nothing of the tap and gets no new secret.
- **The cask removes the quarantine mark** after installing the app, in a
  `postflight_steps` block. Installing the cask is the user's decision to trust
  the Sectile releases on GitHub; the mark would only make macOS report the app
  as damaged.
- **macOS only.** Homebrew casks install macOS apps; the Linux and Windows
  archives keep their manual installation.

## Consequences

- **A release reaches Homebrew within six hours**, the period of the tap's
  schedule, or at once when somebody runs the tap's workflow by hand. Nothing in
  the release procedure of `AGENTS.md` changes.
- **The tap trusts the GitHub stream only.** The GitLab mirror's package is not
  a source of the cask: its archives are not byte-identical to GitHub's
  (ADR 0034), so their checksums would not match the cask.
- **A release without `sectile-desktop-darwin-*` lines in `SHA256SUMS` is never
  published to the tap.** The updater refuses it and leaves the cask on the
  previous version, which keeps installing.
- **Signing and notarization stay a later decision.** Once the app is notarized,
  the quarantine step goes, and the cask can be proposed to `homebrew/cask`.

## Alternatives rejected

- **The release workflow pushes the cask into the tap.** It updates the tap at
  once, but needs a token with write access to the tap stored in this
  repository's secrets, the credential ADR 0034 set out to avoid.
- **A cask that leaves the quarantine mark** and prints the `xattr` command as a
  caveat. Rejected because the installed app would not open until the user ran
  it, which is the step the cask is there to remove.
- **The cask in this repository**, tapped with
  `brew tap sebastienferry/sectile https://github.com/sebastienferry/sectile`.
  Every user would clone the whole Sectile history to read one file, the short
  `brew install --cask sebastienferry/sectile/sectile` would not work without
  that explicit tap, and each release would need a commit on the protected
  `main` to update the checksums.
