import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { test } from 'node:test'
import { AGY_STREAM_FILTER, CLAUDE_STREAM_FILTER, CODEX_STREAM_FILTER } from '../src/lib/autonomousStream.ts'
import { getCommandPresets } from '../src/lib/commandPresets.ts'

// The filters are shell programs, so they are tested by running them, against a
// stream recorded from the provider (claude) or written from the schema the
// provider documents (agy, codex). See src/lib/__fixtures__/README.md.
const fixture = name => fileURLToPath(new URL('../src/lib/__fixtures__/' + name, import.meta.url))

// jq is a prerequisite of an autonomous run, not of the test suite: a checkout
// without it still runs everything else.
const hasJq = spawnSync('jq', ['--version']).status === 0

/** Runs `head -n <lines> fixture | <filter>`, as a headless run would. */
function render(filter, name, lines = 0) {
  const source = lines ? 'head -n ' + lines + ' ' + fixture(name) : 'cat ' + fixture(name)
  return execFileSync('bash', ['-c', 'set -o pipefail; ' + source + ' | ' + filter], { encoding: 'utf8' })
}

const providers = [
  { name: 'claude', filter: CLAUDE_STREAM_FILTER, stream: 'claude-stream.jsonl', answer: "D'accord", warning: 'warning: some npm warning written to stderr' },
  { name: 'agy', filter: AGY_STREAM_FILTER, stream: 'agy-stream.jsonl', answer: 'The suite passes. I changed two files.', warning: 'warning: agy could not read ~/.agyrc, falling back to defaults' },
  { name: 'codex', filter: CODEX_STREAM_FILTER, stream: 'codex-stream.jsonl', answer: 'The suite passes. I changed two files.', warning: 'warning: codex could not detect a git repository root' },
]

for (const provider of providers) {
  test(provider.name + ' renders its whole stream, final answer set apart', { skip: hasJq ? false : 'jq is not installed' }, () => {
    const out = render(provider.filter, provider.stream)
    assert.match(out, /--- result ---/)
    assert.ok(out.includes(provider.answer), 'the final answer is missing from:\n' + out)
    assert.ok(out.split('\n').filter(line => line !== '').length > 3, 'only the final answer was rendered:\n' + out)
  })

  test(provider.name + ' keeps a plain-text line verbatim and keeps going', { skip: hasJq ? false : 'jq is not installed' }, () => {
    const out = render(provider.filter, provider.stream)
    assert.ok(out.includes(provider.warning), 'the warning was swallowed:\n' + out)
    const after = out.slice(out.indexOf(provider.warning) + provider.warning.length)
    assert.ok(after.trim() !== '', 'nothing was rendered after the warning')
  })

  test(provider.name + ' shows what a run that died mid-stream printed', { skip: hasJq ? false : 'jq is not installed' }, () => {
    // Four lines: enough to be past the first events, never as far as the
    // terminal one. A slurping filter prints nothing at all here.
    const out = render(provider.filter, provider.stream, 4)
    assert.ok(out.trim() !== '', 'a truncated stream rendered nothing')
    assert.ok(!out.includes('--- result ---'), 'the fixture reached its terminal event too early')
  })
}

test('no shipped preset slurps the stream', () => {
  for (const preset of getCommandPresets()) {
    for (const command of [preset.cmd, preset.autonomous]) {
      if (!command.includes('jq')) continue
      assert.ok(command.includes('--unbuffered'), preset.label + ' pipes through jq without --unbuffered')
      assert.doesNotMatch(command, /jq\s+(-\S*s\S*|--slurp)/, preset.label + ' invokes jq in slurp mode')
    }
  }
})

test('every autonomous preset asks its CLI for an event stream', () => {
  const streaming = { Claude: '--output-format stream-json --verbose', AGY: '--output-format stream-json', Codex: '--json' }
  for (const preset of getCommandPresets()) {
    const flags = streaming[preset.label]
    if (!flags) continue
    assert.ok(preset.autonomous.includes(flags), preset.label + ' does not ask for an event stream')
    assert.ok(preset.autonomous.includes('| jq '), preset.label + ' does not render its event stream')
  }
})
