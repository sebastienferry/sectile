import assert from 'node:assert/strict'
import { test } from 'node:test'
import { batchIndicator, formatBatchLabel } from '../src/lib/batchMembership.ts'

function member(state, overrides = {}) {
  return { batch: { runId: 'run-1', leadTaskId: 't10', leadKey: '#10', position: 2, size: 3, state, ...overrides } }
}

test('a ticket in no batch shows nothing', () => {
  assert.equal(batchIndicator({}), null)
  assert.equal(batchIndicator(null), null)
  assert.equal(batchIndicator(undefined), null)
})

test('a waiting member takes the queued amber and its label', () => {
  const indicator = batchIndicator(member('waiting'))
  assert.equal(indicator.tone, 'amber')
  assert.equal(indicator.labelKey, 'waiting')
  assert.equal(indicator.leadKey, '#10')
})

test('the processing member takes the running indigo and its label', () => {
  const indicator = batchIndicator(member('processing', { position: 1 }))
  assert.equal(indicator.tone, 'indigo')
  assert.equal(indicator.labelKey, 'processing')
})

test('a done member keeps the badge alone', () => {
  const indicator = batchIndicator(member('done'))
  assert.equal(indicator.tone, null)
  assert.equal(indicator.labelKey, null)
  assert.equal(indicator.state, 'done')
})

test('an unknown state reads as done rather than as work in progress', () => {
  const indicator = batchIndicator(member('exploded'))
  assert.equal(indicator.state, 'done')
  assert.equal(indicator.tone, null)
})

test('the labels name the lead and the position', () => {
  const indicator = batchIndicator(member('waiting'))
  assert.equal(formatBatchLabel('Lot {key}', indicator), 'Lot #10')
  assert.equal(formatBatchLabel('Lot mené par {key} · ticket {position} sur {size}', indicator), 'Lot mené par #10 · ticket 2 sur 3')
})
