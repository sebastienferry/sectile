# Tasks #428 - Package Desktop binaries

Ordered checklist. Each group is one commit (Conventional Commits) and leaves
the tree buildable. Merge `origin/main` first (the spec branch is already
pushed: merge, do not rebase).

## 1. Target-aware packaging (FR2, FR4, FR6, plan decisions 1, 2)

- [ ] T1.1 `desktop/electron/package-options.cjs`: pure
  `packageOptions({platform, arch, agent, out, stagingDir})` returning the
  packager options; defaults reproduce today's call; rejects unknown
  platform/arch.
- [ ] T1.2 `package.cjs`: parse `--platform`, `--arch`, `--agent`, `--out`;
  stage the agent as `agentName(platform)` mode `0755` in a temporary
  directory; remove it afterwards; exit 1 on a missing agent.
- [ ] T1.3 `desktop/tests/package-options.test.cjs` (plan test plan).
- [ ] T1.4 Check locally: `make desktop-package` output unchanged; packaging
  `--platform win32 --arch x64` and `--platform linux --arch x64` from the
  Mac works; with `ELECTRON_SKIP_BINARY_DOWNLOAD=1` set, packager still
  downloads the target zips (plan decision 5). Record the result in the
  commit body.

## 2. Release scripts (FR7, FR11, plan decision 3)

- [ ] T2.1 `scripts/release/build-binaries.sh`, moved verbatim from
  `build:binaries`; `.gitlab-ci.yml` `build:binaries` calls it (no
  behaviour change yet, `SHA256SUMS` still written there).
- [ ] T2.2 `scripts/release/check-desktop-version.sh`.
- [ ] T2.3 `scripts/release/package-desktop.sh` (mapping, archive, Linux
  agent `--version` check).
- [ ] T2.4 `scripts/release/changelog-section.mjs` and
  `desktop/tests/changelog-section.test.mjs`.
- [ ] T2.5 Local run of `package-desktop.sh` for the four targets; inspect
  with `zipinfo` / `tar -tvf` (symlinks in `Sectile.app`, modes, agent at
  `resources/sectile-agent[.exe]`).

## 3. GitLab stream (FR1, FR9, US3, US4.1, US4.3, plan decisions 5, 6, 8)

- [ ] T3.1 `build:desktop` job, tag-only, `ELECTRON_CACHE` cache, `zip`
  installed, four targets, `dist-desktop/` artifact.
- [ ] T3.2 `build:binaries` stops writing `SHA256SUMS`; `publish:binaries`
  needs `build:desktop`, merges `dist-desktop/` into `dist/`, writes
  `SHA256SUMS`, uploads.
- [ ] T3.3 Header comment of `.gitlab-ci.yml`: desktop archives, tag-only,
  second stream on GitHub.
- [ ] T3.4 Branch pipeline check: the MR's pipeline shows no `build:desktop`
  job (US3.3).

## 4. GitHub stream (FR10-FR15, US2, US4.2, plan decisions 7-9)

- [ ] T4.1 `.github/workflows/release.yml`: trigger, permissions,
  concurrency, SHA-pinned actions, jobs `binaries`, `desktop-linux`,
  `desktop-macos`, `release`.
- [ ] T4.2 Create-or-update logic of the `release` job (FR13).
- [ ] T4.3 Grep the workflow and `scripts/release/` for internal names (the
  GitLab groups and hosts `.gitlab-ci.yml` includes, cloud project ids,
  secret names): none (FR15).
- [ ] T4.4 Rehearsal on a personal fork with tag `v0.0.1` (plan test plan):
  15 assets, notes from the changelog, `sha256sum -c` all `OK`, re-run
  replaces assets.

## 5. Documentation and acceptance (FR16-FR18, US1)

- [ ] T5.1 `desktop/README.md` "Install a release" section (per-platform
  steps, `xattr` command, SmartScreen, checksums); `README.md` releases
  section links to it.
- [ ] T5.2 `AGENTS.md` step 6; ADR 0003 unsigned sentence; ADR 0018
  consequence; new `docs/adrs/0034-a-release-is-published-on-both-forges.md`.
- [ ] T5.3 `CHANGELOG.md` `Added` line under `[Unreleased]`.
- [ ] T5.4 Manual acceptance from the rehearsal archives: US1.1 to US1.5 on
  each platform available; macOS arm64 from both a GitHub archive and a
  locally cross-packaged (Linux container) archive, for plan decision 6. If
  the cross-packaged one does not launch, add the `rcodesign` fallback and
  repeat.

## Validation before the implemented stage

- `cd desktop && npm test` green; `make desktop-package` unchanged.
- `sh -n` on every script; `actionlint` on the workflow if available.
- GitLab CI lint of `.gitlab-ci.yml` (CI Lint on the mirror, or the MR
  pipeline).
- T4.4 and T5.4 results reported in the PR description, with what could not
  be run (a platform not available) stated as such.
