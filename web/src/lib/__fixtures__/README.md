# Recorded provider event streams

One stream per autonomous provider, used by `tests/autonomousStream.test.mjs` to
exercise the `jq` filter each shipped preset carries. Each file holds a plain-text
line among its events on purpose: stderr is merged into stdout on a headless run,
and a filter that dies on a warning blanks the whole panel.

- `claude-stream.jsonl` — recorded from `claude -p --output-format stream-json
  --verbose --model claude-haiku-4-5-20251001`, with the session identifiers,
  the thinking signature and the rate-limit payload shortened, and a `tool_use`
  event and a warning line added.
- `agy-stream.jsonl`, `codex-stream.jsonl` — written from the schemas the
  providers document (`.event` / `.result.response` for AGY, `.type` /
  `.item.type` for `codex exec --json`). Neither CLI is installed on the
  workstation this change was written on, so these two are not recordings. The
  filters degrade to the raw line on an unknown shape, which is what keeps a
  schema drift readable rather than silent.
