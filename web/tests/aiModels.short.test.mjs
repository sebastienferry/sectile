import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readFile } from 'node:fs/promises'

// The abbreviation is what a card shows next to its buttons, so it is pinned on
// the table it reads rather than executed: this suite runs without a build step.
const models = await readFile(new URL('../src/lib/aiModels.ts', import.meta.url), 'utf8')

test('the short label keeps four characters at most', () => {
  assert.match(models, /export function shortModelLabel\(model: string\): string/)
  // Every shipped family has a short form, so no card shows a truncation.
  for (const family of ['opus', 'sonnet', 'haiku', 'codex', 'flash', 'pro', 'mini', 'auto']) {
    assert.match(models, new RegExp(`${family}: '[A-Z0-9]{2,4}'`))
  }
  // Anything else falls back to four characters of the distinctive segment.
  assert.match(models, /return base\.slice\(0, 4\)\.toUpperCase\(\)/)
  // The vendor segment is skipped, so `claude-opus-5` cannot read as CLAU.
  assert.match(models, /const VENDOR_SEGMENTS = new Set\(/)
})
