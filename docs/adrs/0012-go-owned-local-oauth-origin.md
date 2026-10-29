# ADR 0012: Go owns the local OAuth origin

Status: Accepted

The Atlassian OAuth application's registered callback must remain stable. A
persisted random port survives a restart but changes with a fresh installation.
The native launcher also previously reconstructed redirect URLs separately from
the backend that performs authorization and token exchange.

The Go server now publishes a read-only `--local-runtime-info` JSON contract
before loading environment files or opening its database. It defines loopback
port 52963 and the provider callback URLs. The native launcher discovers the
port from this contract. Authenticated OAuth configuration returns the expected
callback, and Go owns it during save, authorization and exchange. Tauri retains
only narrow native browser-opening and polling operations.

This policy applies solely to the isolated companion selected by its private
runtime key. Ordinary Sectile web and Electron deployments retain configurable
origins. Port collisions fail visibly rather than falling back. An existing
manifest on another port requires an explicit stopped-service migration; no
running task's server URL is silently changed. The current installation already
uses the selected port.

Existing encrypted application credentials, OAuth state/session binding,
single-use browser tickets, grant refresh and Jira account verification remain
in Go. This change does not replace Jira 3LO with a public-client PKCE flow:
Atlassian's registered client secret is still needed for the current integration.

The renderer receives only presence flags for saved OAuth client credentials, never the saved client ID or secret. Masked editing fields remain empty; a blank client ID is resolved by the native layer and a blank secret is retained by Go. Closing or submitting the form clears both drafts.
