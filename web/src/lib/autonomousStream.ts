/**
 * The rendering half of an autonomous command line.
 *
 * A headless run has no terminal: the only thing a user ever sees of it is what
 * its command line writes to stdout. Asking a provider CLI for its event stream
 * and piping that stream through `jq` is what turns a silent run into a readable
 * one, and keeping the rendering here rather than in the agent is deliberate —
 * `superviseHeadlessRun` moves bytes and knows no provider's schema, so a fourth
 * CLI is a new preset and no Go change.
 *
 * The schemas are the providers' own, so every filter obeys the same four rules:
 *
 * 1. `--unbuffered` and never `-s`. Slurping holds the whole run back until the
 *    CLI exits, which is the defect this change exists to remove.
 * 2. Render every event, not only the terminal one. A run that dies mid-stream
 *    must still show what it did.
 * 3. Read raw (`-R`) and parse with `fromjson`, so a plain-text warning on the
 *    merged stream is passed through verbatim instead of aborting the pipeline.
 *    `catch $raw` prints the original line, not jq's parse error.
 * 4. An unrecognised event shape degrades to a line naming it, so a schema drift
 *    reads as noise rather than as an empty panel.
 *
 * These strings are mirrored in `desktop/src/command-preview.mjs` and
 * `internal/agent/agent_config.go` by contract; a filter that lands in one and
 * not the others is a preview that lies about what runs.
 */

/**
 * Claude emits `{"type":...}` events, the assistant's words in
 * `message.content[]`, and its final answer in the `result` event.
 */
export const CLAUDE_STREAM_FILTER =
  `jq --unbuffered -Rr '. as $raw | try (fromjson | if .type=="result" then "\\n--- result ---\\n"+((.result // "")|if type=="string" then . else tostring end) elif .type=="assistant" then ([.message.content[]? | if .type=="text" then .text elif .type=="tool_use" then "[tool] "+((.name // "")|tostring) else empty end]|join("\\n")) else "["+((.type // "event")|tostring)+"/"+((.subtype // "-")|tostring)+"]" end | select(length>0)) catch $raw'`

/** AGY keys its events on `event` and carries its answer in `result.response`. */
export const AGY_STREAM_FILTER =
  `jq --unbuffered -Rr '. as $raw | try (fromjson | if .event=="result" then "\\n--- result ---\\n"+((.result.response // .result // "")|if type=="string" then . else tostring end) else ("["+((.event // .type // "event")|tostring)+"] "+((.text // .name // "")|if type=="string" then . else tostring end)|sub(" +$";"")) end) catch $raw'`

/** `codex exec --json` wraps each step in an item; the answer is `agent_message`. */
export const CODEX_STREAM_FILTER =
  `jq --unbuffered -Rr '. as $raw | try (fromjson | if .item.type=="agent_message" then "\\n--- result ---\\n"+((.item.text // "")|if type=="string" then . else tostring end) else ("["+((.type // "event")|tostring)+"] "+((.item.type // "")|if type=="string" then . else tostring end)|sub(" +$";"")) end) catch $raw'`

/**
 * The provider CLI invocations that produce those streams, prompt excluded.
 *
 * `claude` refuses `--output-format stream-json` without `--verbose`, which is
 * why the pair travels together.
 */
export const CLAUDE_STREAM_FLAGS = '--output-format stream-json --verbose'
export const AGY_STREAM_FLAGS = '--output-format stream-json'
export const CODEX_STREAM_FLAGS = '--json'
