# Design

## Decisions

### A generic name that defaults, rather than replaces
`Client` holds a GitHub token and a Linear token at the same time
(`internal/trackerapi/client.go`). A single shared credential would send a GitHub
PAT to the Linear GraphQL endpoint whenever both providers are configured.
`SECTILE_TRACKER_TOKEN` is therefore consulted *after* the provider-specific
variable and *before* the external convention fallback. A single-tracker
deployment — the common case — sets only the generic name; a multi-tracker
deployment sets the provider-specific ones and is unaffected.

### The provider-specific variables stay supported
`SECTILE_GITHUB_TOKEN` and `SECTILE_LINEAR_API_KEY` are not removed. Removing them
would break every running server at its next restart, with no signal other than
failing tracker round-trips. They become documented overrides.

### Errors name the credential the operator can set
`configure SECTILE_GITHUB_TOKEN on the server` is replaced by
`configure SECTILE_TRACKER_TOKEN on the server`. The message points at the one
variable that works for every provider; naming the provider-specific override in
the error would send a Linear operator to a GitHub variable.

## Rejected alternatives

- **Hard rename with no fallback.** Smallest surface, but a silent break of every
  existing deployment. Rejected: the failure mode is a tracker round-trip error
  far from the cause.
- **Generic variable wins over the provider-specific one.** Reads more like a true
  rename, but makes a GitHub-plus-Linear deployment depend on unsetting the generic
  variable to stay correct. Rejected as unsafe.
- **Renaming the bare `GITHUB_TOKEN` / `GH_TOKEN` / `LINEAR_API_KEY` fallbacks.**
  These are external conventions injected by CI and by the provider CLIs; they are
  not Sectile's to rename.
