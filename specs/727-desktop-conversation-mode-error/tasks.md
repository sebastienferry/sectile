# Tasks #727 - Desktop Conversation mode error

Ordered checklist. Each group leaves the tree buildable and the tests green.

## 1. Tests first

- [x] T1.1 `internal/agent/agent_conversation_test.go`: add
  `TestConversationKeepsUnexpectedFramesOutOfTheChat` as the plan describes;
  check it fails on the current code with the ticket's frame shown as an error.
- [x] T1.2 Same file: assert a non-JSON stdout line still yields one
  `"kind":"error"` entry (FR2), in the new test or a sibling one.

## 2. The parser (FR1, FR3, FR4, FR5, FR6)

- [x] T2.1 `internal/agent/agent_conversation.go`: declare `Event` as
  `json.RawMessage` and decode it into `conversationStream` in the
  `stream_event` branch only.
- [x] T2.2 Same file: accept a `*json.UnmarshalTypeError` in the frame guard.
- [x] T2.3 Same file: write an `error` event only for a non-empty line that is
  not valid JSON.

## 3. Changelog and checks (FR7)

- [x] T3.1 `CHANGELOG.md`: one `Fixed` line under `[Unreleased]` (#727).
- [x] T3.2 `gofmt -l internal/agent`, `go vet ./internal/agent/`,
  `go test ./internal/agent/` (GOCACHE under `$TMPDIR`; unsandboxed if
  httptest needs local binding). Rerun a keepalive failure before blaming the
  change.
