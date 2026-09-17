# Design

## Decisions

### The server `Settings` row is the store, not the agent configuration
Two user configurations exist. `~/.config/sectile/settings.json`
(`internal/agentconfig`) is a versioned execution contract that
`internal/agentconfig/config.go` documents as secret-free; it is written for the
local agent and read on a workstation that is not necessarily the one holding the
credential. The server `Settings` row is where Jira's credential already lives, it
is already served through `/api/settings` with the token stripped, and it is what
the setup screen already writes. GitHub and GitLab go there. The agent
configuration keeps carrying no secret.

### Tokens are write-only, exactly like `jiraApiToken`
`Settings.JiraAPIToken` is `json:"jiraApiToken,omitempty"` and the API answers with
`jiraApiTokenSet` / `jiraApiTokenFromEnv` instead. `githubToken` and `gitlabToken`
copy that shape, including the `__clear__` sentinel (`db.JiraTokenClearSentinel`)
an empty field cannot express: an empty token on an update means "unchanged",
because the interface never received the value back and cannot resend it.

The sentinel constant is renamed to `TrackerTokenClearSentinel` — it stops being
Jira's — with the literal value `__clear__` unchanged, so no stored data and no
client payload changes.

### Stored configuration wins over the environment
A user who types a token in the interface and sees the tracker keep using a stale
`GITHUB_TOKEN` exported by their shell has no way to diagnose it. So the stored
value wins. The environment is not removed: CI and headless servers depend on it,
and it stays the last resort, resolved by the existing
`trackerapi.trackerToken` chain (`SECTILE_GITHUB_TOKEN` → `SECTILE_TRACKER_TOKEN` →
`GH_TOKEN` → `GITHUB_TOKEN`). When nothing is stored and the environment answers,
the API reports `...FromEnv: true`, which is what lets the interface say the value
comes from the environment rather than showing an empty, apparently unconfigured
field.

### Credentials are resolved per request, not at startup
`NewDB` builds one `trackerapi.Client` at line 121 and keeps it for the process
lifetime, with the token baked in. A credential typed in the interface would not
take effect until a restart. `Client` therefore keeps its environment-derived
values as defaults only, and gains a method returning a shallow copy carrying the
credentials resolved for one project. The resolver lives beside the settings
(`internal/db`), which is the only component able to read both the `Settings` row
and the `Project` row; `trackerapi` stays free of database knowledge.

### A project override, because one token is not enough
`Project` already carries `GithubRepo` and `TrackerUrl`: a workstation already
drives several repositories. Two GitHub organisations with different personal
access tokens, or a GitHub repository next to a GitLab one, both need the credential
resolved per project. The override fields are optional and empty by default; an
empty field falls through to the global setting, which is the behaviour of every
existing project-level override in this codebase.

### GitLab parameters without a GitLab adapter
`internal/tracker/registry.go` resolves adapters by name and has no `gitlab` entry;
`internal/runner/mergerequest.go` only parses GitLab merge-request URLs. Delivering
the parameters is what the ticket asks for, and the credential check proves they
are real. Registering a `gitlab` adapter is a separate body of work — issue
listing, comments, labels, stage mapping — and is a follow-up ticket. Until it
exists, selecting `issueTracker: "gitlab"` on a project keeps failing with the
registry's existing "aucun tracker distant configuré" error, which is honest rather
than a half-working tracker.

## Rejected alternatives

- **Put the credentials in `~/.config/sectile/settings.json`.** Closer to the
  literal wording "user configuration", and readable by the agent without the
  server. Rejected: the file is a versioned, deliberately secret-free contract, it
  is synchronised to workstations, and Jira's credential is not there — two
  credential stores for one setup screen.
- **Drop the environment variables.** A genuine simplification of the resolution
  chain. Rejected: it breaks every headless and CI deployment at its next restart,
  with no signal beyond failing tracker calls.
- **Environment wins over stored configuration.** Matches how the server behaves
  today. Rejected: it makes the setup screen silently ineffective on precisely the
  machines where a token is already exported.
- **One shared "tracker" credential field instead of one per provider.** Fewer
  fields. Rejected for the reason `SECTILE_TRACKER_TOKEN` is consulted *after* the
  provider-specific variable: a deployment serving two providers would send one
  provider's credential to the other's endpoint.
- **Ship the `gitlab` adapter here.** Makes GitLab usable end to end. Rejected as
  out of the clarified scope; it would also gate this change's delivery on a full
  tracker implementation.
- **Rebuild the process-wide client when settings change.** Smaller diff than a
  per-request resolver. Rejected: it is a mutable global shared by the sync loop and
  every in-flight request, and it still cannot serve two projects with different
  credentials.

## Open questions
- **Keeping the token out of the database.** The interface offers
  "store the token in the file, outside the database" (`storeTokenInFile` in
  `TrackerSetup.tsx`), but no such file store exists: `HandleTrackerSetup` is a stub
  that persists nothing, for Jira included. This change stores tokens in the
  database only and does not expose the checkbox for GitHub and GitLab. Whether an
  out-of-database secret file is wanted, and for all three providers at once, is
  unanswered. It blocks nothing here.
