# Tasks

- [ ] Resolve each provider token through provider-specific -> `SECTILE_TRACKER_TOKEN` -> convention fallback in `internal/trackerapi.NewClient`.
- [ ] Replace the GitHub-specific credential-missing errors with the tracker-agnostic message.
- [ ] Cover the resolution order and the error text with unit tests, including the multi-tracker case.
- [ ] Update the README credential documentation and the `cmd/server/main.go` comment.
- [ ] Run `make test` and record the output.
