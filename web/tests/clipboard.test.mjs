import assert from 'node:assert/strict'
import { test } from 'node:test'
import { copyText } from '../src/lib/clipboard.ts'

test('a clipboard that accepts the write reports success with the exact text', async () => {
  const written = []
  const ok = await copyText('#431', { writeText: async text => { written.push(text) } })
  assert.equal(ok, true)
  assert.deepEqual(written, ['#431'])
})

test('a clipboard that refuses the write reports failure instead of throwing', async () => {
  const ok = await copyText('SFE-123', { writeText: async () => { throw new Error('NotAllowedError') } })
  assert.equal(ok, false)
})

test('a missing clipboard reports failure', async () => {
  assert.equal(await copyText('M-7', undefined), false)
})
