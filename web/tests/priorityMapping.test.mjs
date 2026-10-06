import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  confirmLine,
  optionFor,
  partitionBulkOutcomes,
  setLevel,
  setPreferred,
  unwritableLevels,
} from '../src/lib/priorityMapping.ts'

const mapping = {
  options: [
    { id: '1', name: 'P1', level: 'urgent' },
    { id: '2', name: 'P2', level: 'high', guessed: true },
    { id: '3', name: 'P3', level: 'medium' },
    { id: '4', name: 'P4', level: 'medium', manual: true },
  ],
}

test('a write sends the most urgent sure option unless another is preferred', () => {
  assert.equal(optionFor(mapping, 'medium')?.id, '3')
  assert.equal(optionFor(setPreferred(mapping, 'medium', '4'), 'medium')?.id, '4')
  // A preferred guess is not sent.
  assert.equal(optionFor(setPreferred(mapping, 'high', '2'), 'high'), undefined)
})

test('a level only guessed lines carry cannot be written', () => {
  assert.deepEqual(unwritableLevels(mapping), ['high', 'low'])
  // Before the first discovery, nothing is refused.
  assert.deepEqual(unwritableLevels({}), [])
  assert.deepEqual(unwritableLevels(undefined), [])
})

test('confirming or moving a line makes it sure and hand-set', () => {
  const confirmed = confirmLine(mapping, '2')
  assert.deepEqual(confirmed.options[1], { id: '2', name: 'P2', level: 'high', guessed: false, manual: true })
  assert.deepEqual(unwritableLevels(confirmed), ['low'])
  const moved = setLevel(mapping, '4', 'low')
  assert.deepEqual(moved.options[3], { id: '4', name: 'P4', level: 'low', guessed: false, manual: true })
  assert.deepEqual(unwritableLevels(moved), ['high'])
  // The mapping it came from is untouched.
  assert.equal(mapping.options[1].guessed, true)
})

test('a bulk change reports refused tickets without hiding the written ones', () => {
  assert.deepEqual(
    partitionBulkOutcomes([{ key: '#1', written: true }, { key: '#2', written: false }, { key: '#3', written: true }]),
    { written: 2, refused: ['#2'] },
  )
  assert.deepEqual(partitionBulkOutcomes([]), { written: 0, refused: [] })
})
