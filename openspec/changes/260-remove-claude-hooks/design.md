# Design

## Context
The ticket asked for the hook to work on Windows. The first implementation
(PR #263, three commits) ported it into the agent binary as a subcommand,
`<agent binary> sectile-hook`, which did run there. That branch was replaced
by this change after a discussion between the maintainers: the hook is removed,
and the model it fed is kept idle. The decision is recorded in ADR 0012's last
revision; this document covers how the removal is done.

Two constraints shape it. Workstations in the field carry the script and its
registrations, so the removal is mostly a migration. And `~/.claude/settings.json`
is the user's file: the whole point is to stop touching it, so the cleanup has
to be the last write Sectile ever makes there, and only where Sectile wrote.

## Decisions

### The retirement stays; the installer goes
`hooks.go` keeps `claudeHookDir`, `claudeSettingsFile`, `retiredClaudeHookFiles`
(now three names, `sectile-hook.sh` included), `managedHookPath` and the
ownership test. Everything that produced a hook — the `//go:embed`, `hookFiles`,
`executableHooks`, `claudeHookEvents`, `sectileHookGroup`, `mergeHookGroups`,
`registerClaudeHooks` — is deleted rather than disabled. A disabled installer
is a temptation to re-enable; the retirement is the only thing the file has to
do now.

Rejected: **deleting `hooks.go` entirely.** `anyManagedPath` must keep
recognising `.claude/hooks/<retired name>`, otherwise a manifest written by an
earlier release is rejected as foreign and `Scaffold` aborts on every
workstation that ever had the hook. The retired names are load-bearing.

### The cleanup removes, it never merges
`registerClaudeHooks` merged: it kept third-party groups, replaced the first
Sectile group and appended one where missing. `retireClaudeHooks` only drops.
That simplifies the write rule to the one that matters: **rewrite only if a
Sectile group was removed.** A file with no Sectile entry is left byte for byte,
formatting included; a missing file is never created; an event or a `hooks`
object emptied by the removal is deleted rather than left as `[]` or `{}`. An
unparseable file is still left alone and reported through `Scaffold`'s return,
because rewriting it would destroy configuration Sectile does not understand.

### Ownership is by retired script name, plus the pre-release marker
A registration is Sectile's if its command's base name is one of the three
retired scripts, or if its last whitespace-separated token is `sectile-hook`.
The second form never shipped in a release, but the #263 branch was built and
run on developer workstations, and a dead `<binary> sectile-hook` entry would
error on every tool call once that binary is gone. Recognising it costs three
lines. Nothing is matched on the executable path or name: the binary is `agent`
from the Makefile and `sectile-agent` from the desktop package.

### The cleanup runs for every provider
The old registration ran for `claude` only, and the old test forbade touching
Claude settings while setting up another provider. Both were right for an
installer and wrong for a cleanup: a workstation that switched its project to
Codex still has Claude Code installed and still uses it by hand, and `refresh`
would retire the script through the manifest regardless of provider — leaving
the registration pointing at nothing, a hook error on every turn. Since the
cleanup never creates the file and never rewrites one without a Sectile entry,
running it unconditionally has no effect on a workstation that never had the
hook, which is what the other-provider test now asserts.

### The receivers go with the reporter
`/control/runs/{id}/waiting`, its relay to `/api/activities/{id}/waiting`, the
`relay` mutex on `controlledRun`, `/desktop/session-alert`, the alert backlog on
`runQueue`, `desktopRun.WaitingSince` and the desktop's `session-alerts` IPC had
exactly one caller each: the hook. An endpoint nothing calls is attack surface
and a false promise in the code, so they are removed rather than kept dormant.

Kept, on the other side of the wire: the server route, `SetRemoteRunWaiting`,
the `waiting_since` column and every rendering of a waiting run. They are the
model, they are additive, and removing them is a change of its own — the user
chose to keep them as the place a future reporter would plug into.

### `SECTILE_LOOPBACK_URL` stays
It was added by #174 for the hook, but `sectile-agent attach` (#281) now
resolves the daemon from it. It is no longer hook-related and is not touched.

### Tests set both `HOME` and `USERPROFILE`
`os.UserHomeDir` reads `USERPROFILE` on Windows. A test that sets only `HOME`
installs into the developer's real home directory — which is how a test run on
this host overwrote a real `~/.config/sectile/settings.json` with fixture values.
`hooks_test.go` uses a `setHome` helper that sets both, so its assertions hold
on every platform; the rest of the package is left as it is, out of scope.

## Risks
- **An installation that never sets up a project again keeps its hook.** The
  retirement runs on `Scaffold`, which every dispatch and every `init` runs, so
  the window is one task. A workstation that stops using Sectile altogether
  keeps a script that posts to a loopback nobody listens on; it exits 0 and is
  harmless, if untidy.
- **The waiting UI can no longer light up.** Until something feeds
  `waitingSince`, the amber indicator, the activities filter and the desktop
  waiting banner are reachable only by hand through the server route. That is
  the accepted trade: documented in CAPABILITIES §4bis and MEMORY §6.
