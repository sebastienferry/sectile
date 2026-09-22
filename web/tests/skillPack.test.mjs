import assert from 'node:assert/strict'
import { test } from 'node:test'
import { pinLabel, isFromMarketplace, originLabel, previewSummary, isReproducible } from '../src/lib/skillPack.ts'

const pin = {
  projectId: 'p1',
  marketplace: 'acme',
  plugin: 'acme-flow',
  version: '2.1.0',
  commit: '1111111222222223333333344444444555555556',
  appliedAt: '2026-09-22T10:00:00Z',
  skills: ['clarify-issue'],
}

test('the strip names the marketplace, the plugin, the version and the revision', () => {
  assert.equal(pinLabel(pin), 'acme / acme-flow @ 2.1.0 · 1111111')
  assert.equal(pinLabel({ ...pin, version: '', commit: '' }), 'acme / acme-flow')
  assert.equal(pinLabel(null), '')
})

test('the badge follows the origin of the baseline, not the edit', () => {
  assert.equal(isFromMarketplace({ origin: 'marketplace' }), true)
  assert.equal(isFromMarketplace({ origin: 'builtin' }), false)
  assert.equal(isFromMarketplace(null), false)
  assert.equal(originLabel({ origin: 'marketplace', packOrigin: 'acme/acme-flow@2.1.0+1111111' }), 'acme/acme-flow@2.1.0+1111111')
  assert.equal(originLabel({ origin: 'builtin' }), '')
})

test('the preview lists what changes, what is ignored, what is refused and what is missing', () => {
  const summary = previewSummary({
    pack: {
      marketplace: 'acme',
      plugin: 'acme-flow',
      commit: '1111111',
      bodies: {},
      ignored: ['docs-writer', 'release-notes'],
      rejected: { 'create-pr': 'SKILL.md has no frontmatter block' },
      warnings: ['the source is a plain directory'],
    },
    entries: [
      { skillId: 'clarify', dirName: 'clarify-issue', name: 'Clarify', current: 'a', proposed: 'b', changed: true },
      { skillId: 'handoff', dirName: 'handoff-issue', name: 'Handoff', current: 'a', proposed: 'a', changed: false },
    ],
    missing: ['code-issue', 'adjust-issue'],
  })

  assert.deepEqual(summary.changed, ['clarify-issue'])
  assert.deepEqual(summary.unchanged, ['handoff-issue'])
  assert.deepEqual(summary.ignored, ['docs-writer', 'release-notes'])
  assert.deepEqual(summary.rejected, [{ dir: 'create-pr', reason: 'SKILL.md has no frontmatter block' }])
  assert.deepEqual(summary.missing, ['code-issue', 'adjust-issue'])
  assert.deepEqual(summary.warnings, ['the source is a plain directory'])
})

test('an empty preview summarizes to nothing rather than throwing', () => {
  const summary = previewSummary(null)
  assert.deepEqual(summary.changed, [])
  assert.deepEqual(summary.missing, [])
  assert.equal(isReproducible(null), false)
})

test('a pack without a resolved revision is not reproducible', () => {
  assert.equal(isReproducible({ pack: { marketplace: 'acme', plugin: 'p', bodies: {} }, entries: [] }), false)
  assert.equal(isReproducible({ pack: { marketplace: 'acme', plugin: 'p', commit: 'abc1234', bodies: {} }, entries: [] }), true)
})
